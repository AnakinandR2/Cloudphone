package billing

// 资源科目（统一流水 Subject 取值；与 SubjectBalance 并列）。
// 旧的 instance_seat/boot_seat 已被新模型 LicenseUnit 的 Kind(seat/boot_slot) 取代，仅保留时长科目。
const (
	SubjectRuntimeMinute = "runtime_minute" // 时长包余额（消耗型，分钟）
)

// 批次/履约来源（发放入口）。
const (
	SourceOrder  = "order"
	SourceGrant  = "grant"
	SourceAdjust = "adjust"
	SourceTrial  = "trial"
)
