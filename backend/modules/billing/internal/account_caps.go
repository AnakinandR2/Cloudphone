package billing

import (
	"fmt"
	"sync"
)

// AccountCapacitiesV2 新模型下的账户资源容量（统一命名 seat/boot_slot/runtime_minute）。
// 席位/包月开机数取自未过期 license_units；临时开机时长取自钱包余量。
type AccountCapacitiesV2 struct {
	Seat          int   `json:"seat"`
	BootSlot      int   `json:"boot_slot"`
	RuntimeMinute int64 `json:"runtime_minute"`
}

// newModelCapacities 并行查询三个独立来源，组合新模型账户容量。
func newModelCapacities(userID int) (AccountCapacitiesV2, error) {
	var (
		seat int
		boot int
		mins int64
		errs [3]error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		seat, errs[0] = LicenseService.Capacity(userID, KindSeat)
	}()
	go func() {
		defer wg.Done()
		boot, errs[1] = LicenseService.Capacity(userID, KindBootSlot)
	}()
	go func() {
		defer wg.Done()
		mins, errs[2] = RuntimeWalletService.Remaining(userID)
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return AccountCapacitiesV2{}, fmt.Errorf("查询账户容量失败: %w", err)
		}
	}
	return AccountCapacitiesV2{Seat: seat, BootSlot: boot, RuntimeMinute: mins}, nil
}
