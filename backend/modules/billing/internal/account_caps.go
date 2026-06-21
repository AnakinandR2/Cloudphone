package billing

// AccountCapacitiesV2 新模型下的账户资源容量（统一命名 seat/boot_slot/runtime_minute）。
// 席位/包月开机数取自未过期 license_units；临时开机时长取自钱包余量。
type AccountCapacitiesV2 struct {
	Seat          int   `json:"seat"`
	BootSlot      int   `json:"boot_slot"`
	RuntimeMinute int64 `json:"runtime_minute"`
}

// newModelCapacities 组合新模型各来源，给后台账户页提供权威容量。
func newModelCapacities(userID int) (AccountCapacitiesV2, error) {
	seat, err := LicenseService.Capacity(userID, KindSeat)
	if err != nil {
		return AccountCapacitiesV2{}, err
	}
	boot, err := LicenseService.Capacity(userID, KindBootSlot)
	if err != nil {
		return AccountCapacitiesV2{}, err
	}
	mins, err := RuntimeWalletService.Remaining(userID)
	if err != nil {
		return AccountCapacitiesV2{}, err
	}
	return AccountCapacitiesV2{Seat: seat, BootSlot: boot, RuntimeMinute: mins}, nil
}
