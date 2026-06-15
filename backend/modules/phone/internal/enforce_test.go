package phone

import (
	"context"
	"fmt"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
	"manager-backend/modules/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOps 是 midplatPort 的假实现，专供 enforcement worker 测试。
// Destroy 记录 cpID 到 destroyed，StartOrShutdown 记录 cpID 到 shutdown；其余为空桩。
type fakeOps struct {
	destroyed []string
	shutdown  []string
}

func (f *fakeOps) Create(_ context.Context, _ CreateArgs) (*CreateResult, error) {
	return &CreateResult{}, nil
}
func (f *fakeOps) StartOrShutdown(_ context.Context, cpID, _ string) error {
	f.shutdown = append(f.shutdown, cpID)
	return nil
}
func (f *fakeOps) Restart(_ context.Context, _ string) error  { return nil }
func (f *fakeOps) Reset(_ context.Context, _, _ string) error { return nil }
func (f *fakeOps) Destroy(_ context.Context, cpID string) error {
	f.destroyed = append(f.destroyed, cpID)
	return nil
}
func (f *fakeOps) RefreshPhone(_ context.Context, _ string) error { return nil }
func (f *fakeOps) WebRTCAuth(_ context.Context, _ string) (*midplat.WebRTCAuthInfo, error) {
	return &midplat.WebRTCAuthInfo{}, nil
}
func (f *fakeOps) WebRTCState(_ context.Context, _ string) (bool, error) { return false, nil }
func (f *fakeOps) Screenshot(_ context.Context, _, _ string) error       { return nil }
func (f *fakeOps) SetVolume(_ context.Context, _ string, _ int) error    { return nil }
func (f *fakeOps) Rotate(_ context.Context, _, _ string) error           { return nil }
func (f *fakeOps) Shake(_ context.Context, _ string) error               { return nil }
func (f *fakeOps) FileList(_ context.Context, _, _, _ string) ([]midplat.PhoneFile, error) {
	return nil, nil
}
func (f *fakeOps) FileDownload(_ context.Context, _, _, _ string) ([]byte, error) {
	return nil, nil
}
func (f *fakeOps) FileUpload(_ context.Context, _, _, _ string, _ []midplat.UploadFile) error {
	return nil
}
func (f *fakeOps) FileDelete(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeOps) InstalledApps(_ context.Context, _ string) ([]midplat.InstalledApp, error) {
	return nil, nil
}
func (f *fakeOps) InstallApp(_ context.Context, _ string, _ []int64) error { return nil }
func (f *fakeOps) UninstallApp(_ context.Context, _ string, _ []int64, _ []string) error {
	return nil
}
func (f *fakeOps) StartApp(_ context.Context, _ string, _ []int64, _ []string) error { return nil }
func (f *fakeOps) StopApp(_ context.Context, _ string, _ []int64, _ []string) error  { return nil }
func (f *fakeOps) KillAllApps(_ context.Context, _ string) error                     { return nil }
func (f *fakeOps) AdbEnableToken(_ context.Context, _ string) (*midplat.ADBTokenContainer, error) {
	return &midplat.ADBTokenContainer{}, nil
}
func (f *fakeOps) AdbDisableToken(_ context.Context, _ string) error {
	return nil
}
func (f *fakeOps) AdbEnabledMap(_ context.Context, _ []string) (map[string]bool, error) {
	return nil, nil
}
func (f *fakeOps) Root(_ context.Context, _, _ string, _ bool) error { return nil }
func (f *fakeOps) RootEnabledMap(_ context.Context, _ []string) (map[string]bool, error) {
	return nil, nil
}
func (f *fakeOps) RunLogs(_ context.Context, _ string, _, _ int) (*midplat.RunLogPage, error) {
	return &midplat.RunLogPage{}, nil
}
func (f *fakeOps) AdbInfo(_ context.Context, _ string) (*midplat.CloudPhoneAdbInfo, error) {
	return &midplat.CloudPhoneAdbInfo{}, nil
}
func (f *fakeOps) Statuses(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}

func TestEnforcementRecyclesOverQuota(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	const u = 9601
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)
	for i := 0; i < 3; i++ {
		require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "p", CpID: fmt.Sprintf("cp%d", i), Status: StatusStopped}).Error)
	}
	require.NoError(t, billing.GrantInstanceSeatsForTest(u, 1)) // capacity 1
	require.NoError(t, billing.SetDunningForTest(u, billing.DunningRecycled))

	svc.runEnforcement(context.Background())

	var n int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ?", u).Count(&n)
	assert.Equal(t, int64(1), n)     // keep 1 (within capacity, oldest)
	assert.Len(t, fake.destroyed, 2) // destroyed 2 newest

	// idempotent: re-run destroys nothing more
	svc.runEnforcement(context.Background())
	framework.DB.Model(&CloudPhone{}).Where("user_id = ?", u).Count(&n)
	assert.Equal(t, int64(1), n)
	assert.Len(t, fake.destroyed, 2)
}

func TestEnforcementFreezeShutsDownRunningOnly(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	const u = 9602
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "run", CpID: "cpR", Status: StatusRunning}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "stop", CpID: "cpS", Status: StatusStopped}).Error)
	require.NoError(t, billing.GrantInstanceSeatsForTest(u, 5)) // capacity 5 (not over → no recycle)
	require.NoError(t, billing.SetDunningForTest(u, billing.DunningFrozen))

	svc.runEnforcement(context.Background())

	assert.Len(t, fake.shutdown, 1)  // only the RUNNING one shut down
	assert.Len(t, fake.destroyed, 0) // frozen never destroys
	var n int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ?", u).Count(&n)
	assert.Equal(t, int64(2), n) // nothing deleted
}

func TestEnforcementRecycleSkipsTransitional(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_seat_usages", "billing_dunning_states", "billing_entitlement_batches", "billing_ledger_entries")
	})
	const u = 9603
	fake := &fakeOps{}
	svc := newService(newRepository(framework.DB), fake)
	// capacity 1; 3 phones (id asc): oldest STOPPED(keep), then a CREATING(transitional, skip), then a STOPPED(recycle)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "keep", CpID: "cpK", Status: StatusStopped}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "mid", CpID: "cpM", Status: StatusCreating}).Error)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: u, Name: "new", CpID: "cpN", Status: StatusStopped}).Error)
	require.NoError(t, billing.GrantInstanceSeatsForTest(u, 1))
	require.NoError(t, billing.SetDunningForTest(u, billing.DunningRecycled))

	svc.runEnforcement(context.Background())

	// over-quota tail = [cpM(creating, skipped), cpN(stopped, destroyed)] → only cpN destroyed
	assert.Equal(t, []string{"cpN"}, fake.destroyed)
	var n int64
	framework.DB.Model(&CloudPhone{}).Where("user_id = ?", u).Count(&n)
	assert.Equal(t, int64(2), n) // cpK + cpM remain (cpM retried next cycle once settled)
}
