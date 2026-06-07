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
