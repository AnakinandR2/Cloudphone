package library

import (
	"testing"

	"manager-backend/framework"
)

// TestCreateFolder_RejectsDuplicateName 同父级同名文件夹应被拒绝；不同父级/不同名允许；
// 重命名撞名拒绝、改回自身名允许（excludeID 排除自身）。
func TestCreateFolder_RejectsDuplicateName(t *testing.T) {
	uid := 990201
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM library_folders WHERE user_id = ?", uid) })

	// 根下建「作品」→ ok
	a, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "作品"})
	if err != nil {
		t.Fatalf("首次创建应成功: %v", err)
	}
	// 根下再建同名 → 拒绝
	if _, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "作品"}); err == nil {
		t.Fatal("同父级同名文件夹应被拒绝，却成功了")
	}
	// 带空格的同名（TrimSpace 后相同）→ 拒绝
	if _, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "  作品 "}); err == nil {
		t.Fatal("去空格后同名应被拒绝，却成功了")
	}
	// 不同父夹下同名 → 允许
	if _, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "作品", ParentID: a.ID}); err != nil {
		t.Fatalf("不同父级同名应允许: %v", err)
	}
	// 不同名 → 允许
	b, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "草稿"})
	if err != nil {
		t.Fatalf("不同名应允许: %v", err)
	}
	// 重命名「草稿」→「作品」（与根下已有冲突）→ 拒绝
	if _, err := Service.RenameFolder(uid, b.ID, "作品"); err == nil {
		t.Fatal("重命名撞名应被拒绝，却成功了")
	}
	// 重命名为自身原名 → 允许（排除自身）
	if _, err := Service.RenameFolder(uid, b.ID, "草稿"); err != nil {
		t.Fatalf("改回自身名应允许: %v", err)
	}
}

// TestMoveFolder 防环（不能移到自身或子孙）、移到根、目标同名拒绝。
func TestMoveFolder(t *testing.T) {
	uid := 990202
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM library_folders WHERE user_id = ?", uid) })

	a, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "A"})
	if err != nil {
		t.Fatalf("A: %v", err)
	}
	b, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "B", ParentID: a.ID})
	if err != nil {
		t.Fatalf("B: %v", err)
	}
	c, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "C", ParentID: b.ID})
	if err != nil {
		t.Fatalf("C: %v", err)
	}

	// 防环：A 移到其子孙 C → 拒绝
	if _, err := Service.MoveFolder(uid, a.ID, c.ID); err == nil {
		t.Fatal("移动到自身子孙应被拒绝")
	}
	// 移到自身 → 拒绝
	if _, err := Service.MoveFolder(uid, a.ID, a.ID); err == nil {
		t.Fatal("移动到自身应被拒绝")
	}
	// 合法：C 移到根
	if _, err := Service.MoveFolder(uid, c.ID, 0); err != nil {
		t.Fatalf("C 移到根应允许: %v", err)
	}
	if got, _ := Service.repo.getFolder(uid, c.ID); got == nil || got.ParentID != 0 {
		t.Fatal("C 应已在根")
	}
	// 目标同名冲突：A 下建同名 "C"，移到根（根已有 C）→ 拒绝
	dup, err := Service.CreateFolder(uid, CreateFolderRequest{Name: "C", ParentID: a.ID})
	if err != nil {
		t.Fatalf("dup C: %v", err)
	}
	if _, err := Service.MoveFolder(uid, dup.ID, 0); err == nil {
		t.Fatal("移到已有同名的目标应被拒绝")
	}
}

// TestFileDedupName dedupFileName 拼接规则 + fileNameExists 同夹同名判定。
func TestFileDedupName(t *testing.T) {
	if got := dedupFileName("video.mp4", 123); got != "video_123.mp4" {
		t.Fatalf("dedupFileName 扩展名前插 id: got %s", got)
	}
	if got := dedupFileName("noext", 5); got != "noext_5" {
		t.Fatalf("dedupFileName 无扩展名: got %s", got)
	}

	uid := 990301
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM library_files WHERE user_id = ?", uid) })
	f := &LibraryFile{UserID: uint(uid), FolderID: 0, Name: "a.jpg", S3Key: "dedup-test-k1", Status: FileActive}
	if err := Service.repo.createFile(f); err != nil {
		t.Fatalf("createFile: %v", err)
	}
	if ex, _ := Service.repo.fileNameExists(uid, 0, "a.jpg", 0); !ex {
		t.Fatal("同夹同名应命中")
	}
	if ex, _ := Service.repo.fileNameExists(uid, 0, "a.jpg", f.ID); ex {
		t.Fatal("排除自身不应命中")
	}
	if ex, _ := Service.repo.fileNameExists(uid, 0, "b.jpg", 0); ex {
		t.Fatal("不同名不应命中")
	}
}
