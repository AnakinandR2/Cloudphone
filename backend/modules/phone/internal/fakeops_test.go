package phone

import (
	"context"

	"manager-backend/framework/midplat"
)

// fakeOps 是 midplatPort 的假实现，供需要中台 ops 的单测共享（recycle 等）。
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
func (f *fakeOps) ScriptTemplateID(_ context.Context, _ string) (int64, error) { return 1, nil }
func (f *fakeOps) UploadScriptTemplate(_ context.Context, _, _, _, _ string, _ []byte) error {
	return nil
}
func (f *fakeOps) CreateScriptTask(_ context.Context, _ int64, _, _, _ string) (int64, string, error) {
	return 1, "T1", nil
}
func (f *fakeOps) ScriptTaskStatus(_ context.Context, _ int64) (*midplat.ScriptTaskVO, error) {
	return &midplat.ScriptTaskVO{}, nil
}
func (f *fakeOps) ScriptTaskReport(_ context.Context, _ int64) (*midplat.ScriptTaskReport, error) {
	return &midplat.ScriptTaskReport{}, nil
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
