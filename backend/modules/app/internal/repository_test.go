package app

import (
	"testing"

	"manager-backend/framework"
)

// newRepo 返回基于测试 sqlite 的真实仓库。
func newRepo() repository { return newRepository(framework.DB) }

// mkApp 造一条本地绑定记录并落库，返回其 ID。
func mkApp(t *testing.T, a *CustomerApp) uint {
	t.Helper()
	if err := framework.DB.Create(a).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}
	return a.ID
}

func TestRepo_ListByUser_OwnerIsolationAndOrder(t *testing.T) {
	r := newRepo()
	const owner = 90001
	id1 := mkApp(t, &CustomerApp{UserID: owner, AppMD5: "r-md5-a", AppName: "A"})
	id2 := mkApp(t, &CustomerApp{UserID: owner, AppMD5: "r-md5-b", AppName: "B"})
	// 他人 + 商店应用都不应出现在该用户列表里。
	mkApp(t, &CustomerApp{UserID: 90002, AppMD5: "r-md5-c"})
	mkApp(t, &CustomerApp{UserID: 0, Store: true, AppMD5: "r-md5-d"})

	list, err := r.listByUser(owner)
	if err != nil {
		t.Fatalf("listByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 owned apps, got %d", len(list))
	}
	// id DESC：后插入的 id2 在前。
	if list[0].ID != id2 || list[1].ID != id1 {
		t.Errorf("order not id DESC: got [%d,%d] want [%d,%d]", list[0].ID, list[1].ID, id2, id1)
	}
}

func TestRepo_GetByIDs_OwnerScopedAndEmpty(t *testing.T) {
	r := newRepo()
	const owner = 90010
	id := mkApp(t, &CustomerApp{UserID: owner, AppMD5: "r-md5-g1"})
	otherID := mkApp(t, &CustomerApp{UserID: 90011, AppMD5: "r-md5-g2"})

	// 空 ids → 空结果，不报错。
	got, err := r.getByIDs(owner, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty ids: err=%v n=%d", err, len(got))
	}

	// 只返回属于 owner 的；他人 id 被 WHERE user_id 过滤掉。
	got, err = r.getByIDs(owner, []int{int(id), int(otherID)})
	if err != nil {
		t.Fatalf("getByIDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("expected only owner's app, got %+v", got)
	}
}

func TestRepo_CreateUpdate(t *testing.T) {
	r := newRepo()
	rec := &CustomerApp{UserID: 90020, AppMD5: "r-md5-u", AppName: "before", Status: StatusCreating}
	if err := r.create(rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.ID == 0 {
		t.Fatalf("create did not set ID")
	}
	rec.AppName = "after"
	rec.Status = StatusNormal
	if err := r.update(rec); err != nil {
		t.Fatalf("update: %v", err)
	}
	var reloaded CustomerApp
	if err := framework.DB.First(&reloaded, rec.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.AppName != "after" || reloaded.Status != StatusNormal {
		t.Errorf("update not persisted: %+v", reloaded)
	}
}

func TestRepo_DeleteByIDs_OwnerScopedAndEmpty(t *testing.T) {
	r := newRepo()
	const owner = 90030
	id := mkApp(t, &CustomerApp{UserID: owner, AppMD5: "r-md5-d1"})
	otherID := mkApp(t, &CustomerApp{UserID: 90031, AppMD5: "r-md5-d2"})

	// 空 ids → no-op。
	if err := r.deleteByIDs(owner, nil); err != nil {
		t.Fatalf("delete empty: %v", err)
	}

	// 试删他人 id（带 owner 作用域）→ 不应删掉他人记录。
	if err := r.deleteByIDs(owner, []int{int(otherID)}); err != nil {
		t.Fatalf("delete other: %v", err)
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", otherID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("other user's app was wrongly deleted")
	}

	// 删自己的 → 成功。
	if err := r.deleteByIDs(owner, []int{int(id)}); err != nil {
		t.Fatalf("delete own: %v", err)
	}
	framework.DB.Model(&CustomerApp{}).Where("id = ?", id).Count(&cnt)
	if cnt != 0 {
		t.Errorf("own app not deleted")
	}
}

func TestRepo_ListAll_JoinUsers_ExcludesStore(t *testing.T) {
	r := newRepo()
	// 造一个 user，让 LEFT JOIN 能回填手机号/昵称。
	u := &testUser{ID: 90040, Phone: "13800000040", Nickname: "运营测试"}
	if err := framework.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	withUser := mkApp(t, &CustomerApp{UserID: 90040, AppMD5: "r-md5-all1", AppName: "joined"})
	// 商店应用 Store=true 不应出现在 listAll。
	mkApp(t, &CustomerApp{UserID: 0, Store: true, AppMD5: "r-md5-all2"})

	list, err := r.listAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	var found *AdminApp
	for i := range list {
		if list[i].ID == withUser {
			found = &list[i]
		}
		if list[i].Store {
			t.Errorf("listAll must exclude store apps, found id=%d", list[i].ID)
		}
	}
	if found == nil {
		t.Fatalf("seeded user app not in listAll")
	}
	if found.UserPhone != "13800000040" || found.UserNickname != "运营测试" {
		t.Errorf("join users not populated: phone=%q nickname=%q", found.UserPhone, found.UserNickname)
	}
}

func TestRepo_GetAllByIDs_ExcludesStoreAndEmpty(t *testing.T) {
	r := newRepo()
	userApp := mkApp(t, &CustomerApp{UserID: 90050, AppMD5: "r-md5-ga1"})
	storeApp := mkApp(t, &CustomerApp{UserID: 0, Store: true, AppMD5: "r-md5-ga2"})

	got, err := r.getAllByIDs(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty: err=%v n=%d", err, len(got))
	}

	got, err = r.getAllByIDs([]int{int(userApp), int(storeApp)})
	if err != nil {
		t.Fatalf("getAllByIDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != userApp {
		t.Fatalf("store app must be excluded, got %+v", got)
	}
}

func TestRepo_DeleteAllByIDs_ExcludesStoreAndEmpty(t *testing.T) {
	r := newRepo()
	userApp := mkApp(t, &CustomerApp{UserID: 90060, AppMD5: "r-md5-da1"})
	storeApp := mkApp(t, &CustomerApp{UserID: 0, Store: true, AppMD5: "r-md5-da2"})

	if err := r.deleteAllByIDs(nil); err != nil {
		t.Fatalf("empty: %v", err)
	}
	// 试图通过 deleteAllByIDs 删商店应用 → 应被 store=false 条件挡住。
	if err := r.deleteAllByIDs([]int{int(storeApp)}); err != nil {
		t.Fatalf("delete store via all: %v", err)
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", storeApp).Count(&cnt)
	if cnt != 1 {
		t.Errorf("store app wrongly deleted via deleteAllByIDs")
	}
	// 删用户应用 → 成功。
	if err := r.deleteAllByIDs([]int{int(userApp)}); err != nil {
		t.Fatalf("delete user app: %v", err)
	}
	framework.DB.Model(&CustomerApp{}).Where("id = ?", userApp).Count(&cnt)
	if cnt != 0 {
		t.Errorf("user app not deleted")
	}
}

func TestRepo_StoreCRUD(t *testing.T) {
	r := newRepo()
	storeID := mkApp(t, &CustomerApp{UserID: 0, Store: true, AppMD5: "r-md5-s1", AppName: "store"})
	// 非商店应用不应出现在商店查询里。
	userID := mkApp(t, &CustomerApp{UserID: 90070, AppMD5: "r-md5-s2"})

	// listStore 只含商店应用。
	list, err := r.listStore()
	if err != nil {
		t.Fatalf("listStore: %v", err)
	}
	sawStore, sawUser := false, false
	for _, a := range list {
		if a.ID == storeID {
			sawStore = true
		}
		if a.ID == userID {
			sawUser = true
		}
	}
	if !sawStore || sawUser {
		t.Errorf("listStore wrong: sawStore=%v sawUser=%v", sawStore, sawUser)
	}

	// getStoreByIDs：空 + 过滤非商店。
	got, err := r.getStoreByIDs(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("getStoreByIDs empty: err=%v n=%d", err, len(got))
	}
	got, err = r.getStoreByIDs([]int{int(storeID), int(userID)})
	if err != nil {
		t.Fatalf("getStoreByIDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != storeID {
		t.Fatalf("getStoreByIDs must filter to store only, got %+v", got)
	}

	// deleteStoreByIDs：空 no-op + 不能删用户应用 + 能删商店应用。
	if err := r.deleteStoreByIDs(nil); err != nil {
		t.Fatalf("delete empty: %v", err)
	}
	if err := r.deleteStoreByIDs([]int{int(userID)}); err != nil {
		t.Fatalf("delete user via store: %v", err)
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", userID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("user app wrongly deleted via deleteStoreByIDs")
	}
	if err := r.deleteStoreByIDs([]int{int(storeID)}); err != nil {
		t.Fatalf("delete store: %v", err)
	}
	framework.DB.Model(&CustomerApp{}).Where("id = ?", storeID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("store app not deleted")
	}
}
