package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userA   = 7001
	userB   = 7002
	staffOp = "staff:1"
)

func TestGetAccountAutoCreateIdempotent(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	acc, err := BillingService.GetAccount(userA)
	require.NoError(t, err)
	assert.NotZero(t, acc.ID)
	assert.Equal(t, uint(userA), acc.UserID)
	assert.Equal(t, int64(0), acc.BalanceCents)

	again, err := BillingService.GetAccount(userA)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, again.ID)
}

func TestTopupAndAdjustBalanceWithLedger(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	acc, err := BillingService.Topup(userA, 10000, "首次充值", "user:7001")
	require.NoError(t, err)
	assert.Equal(t, int64(10000), acc.BalanceCents)

	acc, err = BillingService.AdjustBalance(userA, 5000, "活动补偿", staffOp)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), acc.BalanceCents)

	acc, err = BillingService.AdjustBalance(userA, -3000, "误充退款", staffOp)
	require.NoError(t, err)
	assert.Equal(t, int64(12000), acc.BalanceCents)

	list, total, err := BillingService.repo.listLedger(userA, 0, 10, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, int64(12000), list[0].BalanceAfterCents) // id DESC，最新在前
	assert.Equal(t, LedgerAdjustDeduct, list[0].Type)
}

func TestAdjustBalanceGuards(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_accounts", "billing_ledger_entries") })

	_, err := BillingService.AdjustBalance(userA, -100, "越扣", staffOp)
	assert.Error(t, err)

	_, err = BillingService.AdjustBalance(userA, 100, "", staffOp)
	assert.Error(t, err)

	_, err = BillingService.Topup(userA, 0, "零充值", "user:7001")
	assert.Error(t, err)
}
