package library

import (
	"testing"
	"time"

	"manager-backend/framework"
)

// sweepExpired：过期付费订阅 → 降级免费（容量重置、月价清零、expire_at 置空、status=expired）。
func TestSweepExpired_DowngradesToFree(t *testing.T) {
	const uid = 930001
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	setSub(t, uid, "t100", 100*GiB, 1800, &past)

	// 另造一个未过期订阅，不应被动到。
	const uid2 = 930002
	cleanLibraryData(t, uid2)
	future := now.Add(48 * time.Hour)
	setSub(t, uid2, "t50", 50*GiB, 1000, &future)

	n, err := sweepExpired(framework.DB, now)
	if err != nil {
		t.Fatalf("sweepExpired: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected >=1 downgraded, got %d", n)
	}

	sub, _ := Service.repo.getSubscription(uid)
	if sub == nil || sub.TierCode != TierFree {
		t.Fatalf("uid should be free, got %+v", sub)
	}
	if sub.ExpireAt != nil || sub.MonthlyPriceCents != 0 {
		t.Fatalf("free reset wrong: %+v", sub)
	}
	// minor-4：降级后是生效态（active），不是 expired。
	if sub.Status != SubActive {
		t.Fatalf("status want active after downgrade, got %s", sub.Status)
	}
	freeBytes := defaultPricingConfig().FreeQuotaBytes
	if sub.CapacityBytes != freeBytes {
		t.Fatalf("capacity want %d, got %d", freeBytes, sub.CapacityBytes)
	}
	// minor-4：DB 中 expire_at 列确为 NULL（map+Select 写 NULL，非零值跳过）。
	var nullCount int64
	framework.DB.Model(&LibrarySubscription{}).
		Where("user_id = ? AND expire_at IS NULL", uid).Count(&nullCount)
	if nullCount != 1 {
		t.Fatalf("expire_at must be NULL in DB, got nullCount=%d", nullCount)
	}

	sub2, _ := Service.repo.getSubscription(uid2)
	if sub2 == nil || sub2.TierCode != "t50" {
		t.Fatalf("uid2 should stay t50, got %+v", sub2)
	}
}

// sweepStaleUploads：超 24h 未确认的 uploading 行被删（释放预占）；新鲜的保留。
func TestSweepStaleUploads_RemovesOld(t *testing.T) {
	const uid = 930003
	cleanLibraryData(t, uid)
	cleanLibraryFiles(t, uid)
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	stale := addFile(t, uid, FileUploading, 1*GiB)
	// 把 created_at 改到 25h 前。
	framework.DB.Model(&LibraryFile{}).Where("id = ?", stale.ID).
		Update("created_at", now.Add(-25*time.Hour))

	fresh := addFile(t, uid, FileUploading, 1*GiB)
	framework.DB.Model(&LibraryFile{}).Where("id = ?", fresh.ID).
		Update("created_at", now.Add(-1*time.Hour))

	n, err := sweepStaleUploads(framework.DB, now)
	if err != nil {
		t.Fatalf("sweepStaleUploads: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected >=1 cleaned, got %d", n)
	}

	if g, _ := Service.repo.getFileByID(stale.ID); g != nil {
		t.Fatal("stale upload should be hard-deleted")
	}
	if g, _ := Service.repo.getFileByID(fresh.ID); g == nil {
		t.Fatal("fresh upload should remain")
	}
}
