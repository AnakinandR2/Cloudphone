package library

import (
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/apperr"
)

// cleanLibraryFiles 额外清理文件/夹/标签数据（在 cleanLibraryData 基础上）。
func cleanLibraryFiles(t *testing.T, userID int) {
	t.Helper()
	t.Cleanup(func() {
		var ids []uint
		framework.DB.Model(&LibraryFile{}).Where("user_id = ?", userID).Pluck("id", &ids)
		if len(ids) > 0 {
			framework.DB.Where("file_id IN ?", ids).Delete(&LibraryFileTag{})
		}
		framework.DB.Unscoped().Where("user_id = ?", userID).Delete(&LibraryFile{})
		framework.DB.Where("user_id = ?", userID).Delete(&LibraryFolder{})
		framework.DB.Where("user_id = ?", userID).Delete(&LibraryTag{})
	})
}

// addFile 直接造一个文件行（绕过 S3 直传），便于测配额/锁定/列表。
func addFile(t *testing.T, userID int, status string, size int64) *LibraryFile {
	t.Helper()
	f := &LibraryFile{
		UserID:    uint(userID),
		Name:      "f.bin",
		FileType:  FileTypeOther,
		SizeBytes: size,
		S3Key:     "library/x/" + status + "-" + time.Now().Format("150405.000000000") + randSuffix(),
		Status:    status,
	}
	if err := Service.repo.createFile(f); err != nil {
		t.Fatalf("addFile: %v", err)
	}
	return f
}

var suffixCounter int

func randSuffix() string {
	suffixCounter++
	return time.Now().Format(".999999999") + "-" + itoaTest(suffixCounter)
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// 配额预占防超卖：uploading 行计入占用，二次预占请求被拦。
func TestQuota_PendingPreventsOversell(t *testing.T) {
	const uid = 920001
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	// 免费 5GiB；造一个声明 4GiB 的 uploading 行。
	addFile(t, uid, FileUploading, 4*GiB)

	pending, err := Service.repo.sumPendingSize(uid)
	if err != nil {
		t.Fatalf("sumPendingSize: %v", err)
	}
	if pending != 4*GiB {
		t.Fatalf("pending want 4GiB, got %d", pending)
	}
	// 容量 5GiB；pending 4GiB；再来 2GiB 应超卖被拦。
	cur, err := Service.currentSubscription(uid)
	if err != nil {
		t.Fatalf("cur: %v", err)
	}
	if pending+2*GiB <= cur.capacity {
		t.Fatalf("expected oversell: %d + 2GiB <= %d", pending, cur.capacity)
	}
	// 1GiB 仍可放下。
	if pending+1*GiB > cur.capacity {
		t.Fatalf("1GiB should fit: %d + 1GiB > %d", pending, cur.capacity)
	}
}

// major-3：reserveUpload 事务校验在 pending+本次 > capacity 时拒绝建行（关闭 TOCTOU 窗口）。
func TestReserveUpload_RejectsOversellInTx(t *testing.T) {
	const uid = 920010
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	// 容量 5GiB；已有一个声明 4GiB 的 uploading 行。
	addFile(t, uid, FileUploading, 4*GiB)

	// 再来 2GiB（4+2>5）→ 事务内重算 SUM 后拒绝，不建行。
	f := &LibraryFile{
		UserID: uint(uid), Name: "big.bin", FileType: FileTypeOther,
		SizeBytes: 2 * GiB, S3Key: "library/x/reserve-" + randSuffix(), Status: FileUploading,
	}
	if err := Service.repo.reserveUpload(uid, f, 5*GiB); err == nil {
		t.Fatal("expected reserve rejected on oversell")
	}
	if f.ID != 0 {
		t.Fatalf("rejected reserve must not insert row, got id=%d", f.ID)
	}
	// 占用未变（仍只有那一个 4GiB 行）。
	pending, _ := Service.repo.sumPendingSize(uid)
	if pending != 4*GiB {
		t.Fatalf("pending want 4GiB unchanged, got %d", pending)
	}

	// 1GiB 仍可放下（4+1<=5）→ 建行成功。
	f2 := &LibraryFile{
		UserID: uint(uid), Name: "ok.bin", FileType: FileTypeOther,
		SizeBytes: 1 * GiB, S3Key: "library/x/reserve-" + randSuffix(), Status: FileUploading,
	}
	if err := Service.repo.reserveUpload(uid, f2, 5*GiB); err != nil {
		t.Fatalf("1GiB should fit: %v", err)
	}
	if f2.ID == 0 {
		t.Fatal("accepted reserve must insert row")
	}
}

// major-3：confirm 真实大小 > 声明且会越容量 → 拒绝，且不污染 used_bytes（行被删、对象语义上删除）。
func TestConfirm_RealSizeOverCapacity_Rejected(t *testing.T) {
	const uid = 920011
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	// 容量 5GiB；已有 active 4GiB 占用。
	active := addFile(t, uid, FileActive, 4*GiB)
	if err := Service.repo.addUsedBytes(uid, 4*GiB); err != nil {
		t.Fatalf("seed used: %v", err)
	}
	// 新 uploading 声明 1GiB（预占合法：4+1<=5）。
	upl := addFile(t, uid, FileUploading, 1*GiB)

	// confirm 时真实大小 3GiB（4+3>5 越容量）→ 拒绝。
	if err := Service.repo.confirmActiveWithinCapacity(uid, upl.ID, 3*GiB, 5*GiB); err == nil {
		t.Fatal("expected reject when real size overflows capacity")
	}
	// used_bytes 未被污染（仍是已 active 的 4GiB）。
	usage, _ := Service.repo.getUsage(uid)
	if usage.UsedBytes != 4*GiB {
		t.Fatalf("used_bytes must stay 4GiB, got %d", usage.UsedBytes)
	}
	// 该 uploading 行未被转成 active（仍 uploading，等待上层删除或 cron 清理）。
	g, _ := Service.repo.getFileByID(upl.ID)
	if g == nil || g.Status != FileUploading {
		t.Fatalf("uploading row should remain uploading (not active), got %+v", g)
	}
	_ = active
}

// major-3：confirm 真实大小在容量内 → 修正大小 + active + 累加 used_bytes（事务路径）。
func TestConfirm_WithinCapacity_Commits(t *testing.T) {
	const uid = 920012
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	upl := addFile(t, uid, FileUploading, 1*GiB)
	realSize := int64(700 * 1024 * 1024)
	if err := Service.repo.confirmActiveWithinCapacity(uid, upl.ID, realSize, 5*GiB); err != nil {
		t.Fatalf("confirm within capacity: %v", err)
	}
	g, _ := Service.repo.getFileByID(upl.ID)
	if g == nil || g.Status != FileActive || g.SizeBytes != realSize {
		t.Fatalf("file should be active with corrected size, got %+v", g)
	}
	usage, _ := Service.repo.getUsage(uid)
	if usage.UsedBytes != realSize {
		t.Fatalf("used want %d, got %d", realSize, usage.UsedBytes)
	}
}

// confirm 真实大小修正：声明 1GiB，真实 500MiB → used_bytes 累加真实值。
func TestConfirm_RealSizeCorrection(t *testing.T) {
	const uid = 920002
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	f := addFile(t, uid, FileUploading, 1*GiB)

	// 模拟 confirm 的核心副作用（不依赖 S3）：修正大小→active→累加用量。
	realSize := int64(500 * 1024 * 1024)
	f.SizeBytes = realSize
	f.Status = FileActive
	if err := Service.repo.updateFile(f); err != nil {
		t.Fatalf("updateFile: %v", err)
	}
	if err := Service.repo.addUsedBytes(uid, realSize); err != nil {
		t.Fatalf("addUsedBytes: %v", err)
	}
	usage, _ := Service.repo.getUsage(uid)
	if usage.UsedBytes != realSize {
		t.Fatalf("used want %d, got %d", realSize, usage.UsedBytes)
	}
	// reconcile 应与 sum(active) 一致。
	recon, err := Service.reconcileUsage(uid)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if recon != realSize {
		t.Fatalf("reconcile want %d, got %d", realSize, recon)
	}
}

// 删除回收：软删 active 文件后 used_bytes 减去其大小。
func TestDelete_ReclaimsUsage(t *testing.T) {
	const uid = 920003
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	fixedNow(t, time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC))
	f := addFile(t, uid, FileActive, 2*GiB)
	if err := Service.repo.addUsedBytes(uid, 2*GiB); err != nil {
		t.Fatalf("seed usage: %v", err)
	}

	if err := Service.DeleteFile(uid, f.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	usage, _ := Service.repo.getUsage(uid)
	if usage.UsedBytes != 0 {
		t.Fatalf("used want 0 after delete, got %d", usage.UsedBytes)
	}
	// 软删后列表不再返回该文件。
	list, total, err := Service.ListFiles(uid, fileListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Fatalf("expected empty list, got total=%d len=%d", total, len(list))
	}
}

// 超额锁定：used>capacity 时 assertNotLocked 报错；列表/删除仍可用；降回容量内恢复。
func TestOverflowLock_BlocksFetchAllowsListDelete(t *testing.T) {
	const uid = 920004
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	// 当前 t50 50GiB → 降级免费态前模拟：直接设容量 5GiB 但用量 8GiB（降级造成超额）。
	setSub(t, uid, TierFree, 5*GiB, 0, nil)
	f1 := addFile(t, uid, FileActive, 5*GiB)
	f2 := addFile(t, uid, FileActive, 3*GiB)
	if err := Service.repo.setUsedBytes(uid, 8*GiB); err != nil {
		t.Fatalf("seed usage: %v", err)
	}

	// 取用被拦。
	if err := Service.assertNotLocked(uid); err == nil {
		t.Fatal("expected locked error")
	} else if apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("want forbidden, got %v", err)
	}

	// 列表仍可用（看得见才能删）。
	list, total, err := Service.ListFiles(uid, fileListFilter{})
	if err != nil {
		t.Fatalf("list during lock: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("list want 2 during lock, got %d", total)
	}

	// 删除仍可用，且回收用量。
	if err := Service.DeleteFile(uid, f1.ID); err != nil {
		t.Fatalf("delete during lock: %v", err)
	}
	usage, _ := Service.repo.getUsage(uid)
	if usage.UsedBytes != 3*GiB {
		t.Fatalf("used want 3GiB after delete, got %d", usage.UsedBytes)
	}

	// 降回容量内（3GiB <= 5GiB）→ 取用恢复。
	if err := Service.assertNotLocked(uid); err != nil {
		t.Fatalf("expected unlocked after delete, got %v", err)
	}
	_ = f2
}

// overview 的 locked 标志随用量翻转。
func TestOverview_LockedFlag(t *testing.T) {
	const uid = 920005
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	setSub(t, uid, TierFree, 5*GiB, 0, nil)
	if err := Service.repo.setUsedBytes(uid, 6*GiB); err != nil {
		t.Fatalf("seed usage: %v", err)
	}
	ov, err := Service.Overview(uid)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if !ov.Usage.Locked {
		t.Fatal("expected locked=true when used>capacity")
	}
}
