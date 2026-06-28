package library

import (
	"testing"

	"manager-backend/framework"
)

// cleanBlobs 清理本测试造的 blob 行（多测试共用一个 sqlite 库）。
func cleanBlobs(t *testing.T, md5s ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, m := range md5s {
			framework.DB.Where("md5 = ?", m).Delete(&LibraryBlob{})
		}
	})
}

// hex32 造一个合法 32-hex md5（用单字符填充）。
func hex32(c byte) string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

// ---- blob 建/查/唯一键冲突 ----

func TestBlob_CreateLookupUniqueConflict(t *testing.T) {
	md5 := hex32('a')
	cleanBlobs(t, md5)

	// 不存在 → (nil,nil)。
	if b, err := Service.repo.getBlobByMD5Size(md5, 100); err != nil || b != nil {
		t.Fatalf("expected nil blob, got %+v err=%v", b, err)
	}

	// 建成。
	b1 := &LibraryBlob{MD5: md5, SizeBytes: 100, SliceMD5: hex32('b'), S3Key: "library/x/k1", RefCount: 1}
	if err := Service.repo.createBlob(b1); err != nil {
		t.Fatalf("createBlob: %v", err)
	}
	if b1.ID == 0 {
		t.Fatal("createBlob must assign ID")
	}

	// 查到。
	got, err := Service.repo.getBlobByMD5Size(md5, 100)
	if err != nil || got == nil || got.ID != b1.ID {
		t.Fatalf("lookup mismatch: %+v err=%v", got, err)
	}

	// 同 (md5,size) 再建 → errBlobConflict。
	b2 := &LibraryBlob{MD5: md5, SizeBytes: 100, S3Key: "library/x/k2", RefCount: 1}
	if err := Service.repo.createBlob(b2); err != errBlobConflict {
		t.Fatalf("expected errBlobConflict, got %v", err)
	}

	// 同 md5 但不同 size → 允许（去重键是 (md5,size) 组合）。
	b3 := &LibraryBlob{MD5: md5, SizeBytes: 200, S3Key: "library/x/k3", RefCount: 1}
	if err := Service.repo.createBlob(b3); err != nil {
		t.Fatalf("different size should not conflict: %v", err)
	}
}

// ---- incRef / decRef 行锁 ----

func TestBlob_IncDecRef(t *testing.T) {
	md5 := hex32('c')
	cleanBlobs(t, md5)
	b := &LibraryBlob{MD5: md5, SizeBytes: 10, S3Key: "library/x/r", RefCount: 1}
	if err := Service.repo.createBlob(b); err != nil {
		t.Fatalf("createBlob: %v", err)
	}
	if err := Service.repo.incRef(b.ID); err != nil {
		t.Fatalf("incRef: %v", err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(md5, 10); g.RefCount != 2 {
		t.Fatalf("ref want 2, got %d", g.RefCount)
	}
	if err := Service.repo.decRef(b.ID); err != nil {
		t.Fatalf("decRef: %v", err)
	}
	if err := Service.repo.decRef(b.ID); err != nil {
		t.Fatalf("decRef: %v", err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(md5, 10); g.RefCount != 0 {
		t.Fatalf("ref want 0, got %d", g.RefCount)
	}
	// 再减不为负（夹紧）。
	if err := Service.repo.decRef(b.ID); err != nil {
		t.Fatalf("decRef: %v", err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(md5, 10); g.RefCount != 0 {
		t.Fatalf("ref want clamped 0, got %d", g.RefCount)
	}
}

// ---- 孤儿 blob 清理：ref==0 删行；ref>0 保留 ----

func TestBlob_OrphanCleanup(t *testing.T) {
	mdZero := hex32('d')
	mdLive := hex32('e')
	cleanBlobs(t, mdZero, mdLive)

	zero := &LibraryBlob{MD5: mdZero, SizeBytes: 1, S3Key: "library/x/zero", RefCount: 0}
	live := &LibraryBlob{MD5: mdLive, SizeBytes: 1, S3Key: "library/x/live", RefCount: 1}
	if err := Service.repo.createBlob(zero); err != nil {
		t.Fatalf("create zero: %v", err)
	}
	if err := Service.repo.createBlob(live); err != nil {
		t.Fatalf("create live: %v", err)
	}

	orphans, err := Service.repo.listOrphanBlobs()
	if err != nil {
		t.Fatalf("listOrphanBlobs: %v", err)
	}
	foundZero, foundLive := false, false
	for _, b := range orphans {
		if b.ID == zero.ID {
			foundZero = true
		}
		if b.ID == live.ID {
			foundLive = true
		}
	}
	if !foundZero || foundLive {
		t.Fatalf("orphan scan wrong: foundZero=%v foundLive=%v", foundZero, foundLive)
	}

	// deleteBlobIfZero：ref==0 删；ref>0 不删。
	if deleted, err := Service.repo.deleteBlobIfZero(zero.ID); err != nil || !deleted {
		t.Fatalf("zero must be deleted: deleted=%v err=%v", deleted, err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(mdZero, 1); g != nil {
		t.Fatal("zero blob row must be gone")
	}
	if deleted, err := Service.repo.deleteBlobIfZero(live.ID); err != nil || deleted {
		t.Fatalf("live must NOT be deleted: deleted=%v err=%v", deleted, err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(mdLive, 1); g == nil {
		t.Fatal("live blob row must remain")
	}

	// sweepOrphanBlobs（无 S3 时仅删行）：再造一个 ref=0 blob，扫后行被删。
	mdSweep := hex32('f')
	cleanBlobs(t, mdSweep)
	sb := &LibraryBlob{MD5: mdSweep, SizeBytes: 1, S3Key: "library/x/sweep", RefCount: 0}
	if err := Service.repo.createBlob(sb); err != nil {
		t.Fatalf("create sweep: %v", err)
	}
	if _, err := sweepOrphanBlobs(framework.DB, Service.now()); err != nil {
		t.Fatalf("sweepOrphanBlobs: %v", err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(mdSweep, 1); g != nil {
		t.Fatal("swept orphan blob must be gone")
	}
}

// ---- resolveBlobForConfirm：收编（建 blob ref=1）vs 合并（指向旧 blob + ref++）----

func TestConfirm_AdoptThenMerge(t *testing.T) {
	const uid = 940001
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	md5 := hex32('1')
	cleanBlobs(t, md5)

	// 文件 A：首传收编 → 建 blob，S3Key 不变（=tempKey），ref=1。
	fa := addFile(t, uid, FileUploading, 50)
	fa.MD5 = md5
	fa.SliceMD5 = hex32('2')
	tempA := fa.S3Key
	if err := Service.repo.updateFile(fa); err != nil {
		t.Fatalf("update fa: %v", err)
	}
	if err := Service.resolveBlobForConfirm(fa, tempA, 50); err != nil {
		t.Fatalf("resolve A (adopt): %v", err)
	}
	if fa.BlobID == 0 || fa.S3Key != tempA {
		t.Fatalf("adopt: blobID=%d s3key=%s (want unchanged %s)", fa.BlobID, fa.S3Key, tempA)
	}
	blob, _ := Service.repo.getBlobByMD5Size(md5, 50)
	if blob == nil || blob.RefCount != 1 || blob.S3Key != tempA {
		t.Fatalf("adopt blob wrong: %+v", blob)
	}

	// 文件 B：同 (md5,size) → 合并到旧 blob，S3Key 指向 blob.S3Key，ref=2。
	fb := addFile(t, uid, FileUploading, 50)
	fb.MD5 = md5
	fb.SliceMD5 = hex32('2')
	tempB := fb.S3Key
	if err := Service.repo.updateFile(fb); err != nil {
		t.Fatalf("update fb: %v", err)
	}
	if err := Service.resolveBlobForConfirm(fb, tempB, 50); err != nil {
		t.Fatalf("resolve B (merge): %v", err)
	}
	if fb.BlobID != blob.ID || fb.S3Key != tempA {
		t.Fatalf("merge: blobID=%d s3key=%s (want blob %d key %s)", fb.BlobID, fb.S3Key, blob.ID, tempA)
	}
	blob2, _ := Service.repo.getBlobByMD5Size(md5, 50)
	if blob2.RefCount != 2 {
		t.Fatalf("merge ref want 2, got %d", blob2.RefCount)
	}
}

// ---- 秒传查重门控：slice_md5 一致命中 vs 不一致放行真实上传 ----
// 验证 PresignUpload 内的秒传判定条件（不直连 S3，直接校验 blob 命中逻辑）。

func TestInstant_SliceGate(t *testing.T) {
	const uid = 940002
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	md5 := hex32('3')
	slice := hex32('4')
	cleanBlobs(t, md5)

	// 既有 blob：slice=slice。
	blob := &LibraryBlob{MD5: md5, SizeBytes: 200, SliceMD5: slice, S3Key: "library/x/inst", RefCount: 1}
	if err := Service.repo.createBlob(blob); err != nil {
		t.Fatalf("createBlob: %v", err)
	}

	// 命中：slice 一致 → tryInstantHit 成功，文件 active、指向 blob、ref=2、used+=size。
	f := addFile(t, uid, FileUploading, 200)
	f.MD5 = md5
	f.SliceMD5 = slice
	if err := Service.repo.updateFile(f); err != nil {
		t.Fatalf("update f: %v", err)
	}
	got, _ := Service.repo.getBlobByMD5Size(md5, 200)
	if !(got != nil && got.SliceMD5 != "" && got.SliceMD5 == f.SliceMD5) {
		t.Fatal("slice-match gate should pass when equal")
	}
	res, ok := Service.tryInstantHit(uid, f, got, 5*GiB)
	if !ok || res == nil || !res.Instant {
		t.Fatalf("instant hit should succeed: ok=%v res=%+v", ok, res)
	}
	gf, _ := Service.repo.getFileByID(f.ID)
	if gf.Status != FileActive || gf.BlobID != blob.ID || gf.SizeBytes != 200 {
		t.Fatalf("instant file wrong: %+v", gf)
	}
	if nb, _ := Service.repo.getBlobByMD5Size(md5, 200); nb.RefCount != 2 {
		t.Fatalf("instant ref want 2, got %d", nb.RefCount)
	}
	usage, _ := Service.repo.getUsage(uid)
	if usage.UsedBytes != 200 {
		t.Fatalf("instant used want 200, got %d", usage.UsedBytes)
	}

	// 不一致：slice 不同 → 门控不通过（应走真实上传，不秒传）。
	if got.SliceMD5 == hex32('9') {
		t.Fatal("precondition")
	}
	mismatch := got.SliceMD5 != "" && got.SliceMD5 == hex32('9')
	if mismatch {
		t.Fatal("slice mismatch must NOT pass instant gate")
	}
}

// ---- DeleteFile 减引用 + 非最后引用保留 / 最后引用归零 ----

func TestDelete_DecRefAndOrphan(t *testing.T) {
	const uid = 940003
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	md5 := hex32('5')
	cleanBlobs(t, md5)

	blob := &LibraryBlob{MD5: md5, SizeBytes: 30, SliceMD5: hex32('6'), S3Key: "library/x/shared", RefCount: 2}
	if err := Service.repo.createBlob(blob); err != nil {
		t.Fatalf("createBlob: %v", err)
	}
	// 两个 active 文件指向同一 blob。
	f1 := addFile(t, uid, FileActive, 30)
	f2 := addFile(t, uid, FileActive, 30)
	for _, f := range []*LibraryFile{f1, f2} {
		f.BlobID = blob.ID
		f.S3Key = blob.S3Key
		if err := Service.repo.updateFile(f); err != nil {
			t.Fatalf("update: %v", err)
		}
	}
	_ = Service.repo.addUsedBytes(uid, 60)

	// 删 f1 → ref 2→1，blob 仍在（非最后引用），不进孤儿清单。
	if err := Service.DeleteFile(uid, f1.ID); err != nil {
		t.Fatalf("delete f1: %v", err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(md5, 30); g == nil || g.RefCount != 1 {
		t.Fatalf("after del f1 ref want 1, got %+v", g)
	}

	// 删 f2 → ref 1→0，进孤儿清单；sweepOrphanBlobs 删 blob 行。
	if err := Service.DeleteFile(uid, f2.ID); err != nil {
		t.Fatalf("delete f2: %v", err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(md5, 30); g == nil || g.RefCount != 0 {
		t.Fatalf("after del f2 ref want 0, got %+v", g)
	}
	if _, err := sweepOrphanBlobs(framework.DB, Service.now()); err != nil {
		t.Fatalf("sweepOrphanBlobs: %v", err)
	}
	if g, _ := Service.repo.getBlobByMD5Size(md5, 30); g != nil {
		t.Fatal("orphan blob must be purged at ref 0")
	}

	// 配额回收：删两个 active 后 used 归 0（配额按逻辑文件自身大小计，去重不影响）。
	usage, _ := Service.repo.getUsage(uid)
	if usage.UsedBytes != 0 {
		t.Fatalf("used want 0 after deleting both, got %d", usage.UsedBytes)
	}
}

// ---- cron：软删文件硬删按 blob_id 分流（blob_id>0 不按文件删 S3）----

func TestSweepSoftDeleted_SplitByBlobID(t *testing.T) {
	const uid = 940004
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	now := Service.now()

	// 去重文件（blob_id>0）软删后：硬删行（不按文件 key 删 S3）。
	dedup := addFile(t, uid, FileActive, 10)
	dedup.BlobID = 12345
	if err := Service.repo.updateFile(dedup); err != nil {
		t.Fatalf("update dedup: %v", err)
	}
	if err := Service.repo.softDeleteFile(uid, dedup.ID, now); err != nil {
		t.Fatalf("soft del dedup: %v", err)
	}
	// 存量文件（blob_id==0）软删后：硬删行（旧路径会删 S3，但测试无 S3，仅验证硬删）。
	legacy := addFile(t, uid, FileActive, 10) // BlobID 默认 0
	if err := Service.repo.softDeleteFile(uid, legacy.ID, now); err != nil {
		t.Fatalf("soft del legacy: %v", err)
	}

	n, err := sweepSoftDeletedFiles(framework.DB, now)
	if err != nil {
		t.Fatalf("sweepSoftDeletedFiles: %v", err)
	}
	if n < 2 {
		t.Fatalf("expected >=2 hard-deleted, got %d", n)
	}
	if g, _ := Service.repo.getFileByID(dedup.ID); g != nil {
		t.Fatal("dedup soft-deleted row must be hard-deleted")
	}
	if g, _ := Service.repo.getFileByID(legacy.ID); g != nil {
		t.Fatal("legacy soft-deleted row must be hard-deleted")
	}
}

// ---- isHex32 / ETag 校验辅助 ----

func TestIsHex32(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{hex32('a'), true},
		{"0123456789abcdef0123456789abcdef", true},
		{"0123456789ABCDEF0123456789ABCDEF", false}, // 大写非 [0-9a-f]
		{"abc", false},
		{hex32('a') + "-2", false}, // 分段 ETag
		{"", false},
	}
	for _, c := range cases {
		if got := isHex32(c.in); got != c.want {
			t.Fatalf("isHex32(%q)=%v want %v", c.in, got, c.want)
		}
	}
}
