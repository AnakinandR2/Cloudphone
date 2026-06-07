package billing

import "manager-backend/framework/apperr"

// serviceImpl 计费服务，依赖注入 repository（不接触全局 DB）。
type serviceImpl struct{ repo repository }

// BillingService 模块内服务实例，由 module.Init 注入 DB 后装配。
var BillingService *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// GetAccount 取（或自动创建）当前用户账户。
func (s *serviceImpl) GetAccount(userID int) (*Account, error) {
	return s.repo.getOrCreateAccount(userID)
}

// Topup 充值入账（计划1 为桩：直接增加余额；计划4 改由支付网关回调驱动）。
func (s *serviceImpl) Topup(userID int, amountCents int64, reason, operator string) (*Account, error) {
	if amountCents <= 0 {
		return nil, apperr.Validation("充值金额必须大于0")
	}
	return s.repo.applyBalance(userID, amountCents, LedgerTopup, reason, 0, operator)
}

// AdjustBalance 运营手动赠送(正)/扣减(负)余额，理由必填；扣减不可越扣为负。退款=负向扣减。
func (s *serviceImpl) AdjustBalance(userID int, deltaCents int64, reason, operator string) (*Account, error) {
	if deltaCents == 0 {
		return nil, apperr.Validation("调整金额不能为0")
	}
	if reason == "" {
		return nil, apperr.Validation("调整理由必填")
	}
	typ := LedgerAdjustGrant
	if deltaCents < 0 {
		typ = LedgerAdjustDeduct
	}
	return s.repo.applyBalance(userID, deltaCents, typ, reason, 0, operator)
}

// ListLedger 费用日志：当前用户的余额/资源包流水（分页 + 科目/类型筛选）。
func (s *serviceImpl) ListLedger(userID, page, size int, subject, typ string) ([]LedgerEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	items, total, err := s.repo.listLedger(userID, (page-1)*size, size, subject, typ)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// AdminGetAccount 运营查看任意用户的账户 + 近期流水（属主由调用方按权限控制）。
func (s *serviceImpl) AdminGetAccount(userID, page, size int) (*Account, []LedgerEntry, error) {
	acc, err := s.repo.getOrCreateAccount(userID)
	if err != nil {
		return nil, nil, err
	}
	ledger, _, err := s.ListLedger(userID, page, size, "", "")
	if err != nil {
		return nil, nil, err
	}
	return acc, ledger, nil
}
