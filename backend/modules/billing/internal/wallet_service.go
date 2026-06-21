package billing

// walletServiceImpl 钱包服务（新购买/结算流程的余额读写门面，复用现有 repository）。
type walletServiceImpl struct{ repo repository }

// WalletService 模块内单例。
var WalletService *walletServiceImpl

func newWalletService(repo repository) *walletServiceImpl { return &walletServiceImpl{repo: repo} }

// BalanceCents 当前余额（无账户则 0）。
func (s *walletServiceImpl) BalanceCents(userID int) (int64, error) {
	acc, err := s.repo.getAccountOrNil(userID)
	if err != nil {
		return 0, err
	}
	if acc == nil {
		return 0, nil
	}
	return acc.BalanceCents, nil
}

// Charge 扣余额并写流水（余额不足报错）。
func (s *walletServiceImpl) Charge(userID int, cents int64, typ, reason string, orderID uint, operator string) error {
	_, err := s.repo.applyBalance(userID, -cents, typ, reason, orderID, operator)
	return err
}

// TopUp 充值入账。
func (s *walletServiceImpl) TopUp(userID int, cents int64, reason, operator string) error {
	_, err := s.repo.applyBalance(userID, cents, LedgerTopup, reason, 0, operator)
	return err
}

// insertResourceLedger 写一条资源科目流水（subject=runtime_minute 等）。
func (s *walletServiceImpl) insertResourceLedger(userID int, subject string, delta, after int64, ledgerType, reason, operator string) error {
	return s.repo.insertLedgerEntry(&LedgerEntry{
		UserID: uint(userID), Subject: subject, Type: ledgerType,
		Delta: delta, BalanceAfter: after, Reason: reason, Operator: operator,
	})
}
