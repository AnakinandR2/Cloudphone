package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModelCapacities_FromLicenseUnitsAndWallet(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_license_units", "billing_runtime_minute_wallets", "billing_ledger_entries")
	})
	uid := 930101
	now := time.Now()
	require.NoError(t, LicenseService.repo.create([]LicenseUnit{
		{UserID: uint(uid), Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(48 * time.Hour)},
		{UserID: uint(uid), Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(72 * time.Hour)},
		{UserID: uint(uid), Kind: KindSeat, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(-1 * time.Hour)}, // 过期不计
		{UserID: uint(uid), Kind: KindBootSlot, Status: LicenseActive, Source: SourceOrder, ExpireAt: now.Add(48 * time.Hour)},
	}))
	_, err := newRuntimeWalletRepository(framework.DB).addMinutes(uid, 600)
	require.NoError(t, err)

	caps, err := newModelCapacities(uid)
	require.NoError(t, err)
	assert.Equal(t, 2, caps.Seat, "未过期 seat")
	assert.Equal(t, 1, caps.BootSlot)
	assert.Equal(t, int64(600), caps.RuntimeMinute)
}
