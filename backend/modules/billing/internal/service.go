package billing

// serviceImpl 计费服务，依赖注入 repository（不接触全局 DB）。
type serviceImpl struct{ repo repository }

// BillingService 模块内服务实例，由 module.Init 注入 DB 后装配。
var BillingService *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// GetAccount 取（或自动创建）当前用户账户。
func (s *serviceImpl) GetAccount(userID int) (*Account, error) {
	return s.repo.getOrCreateAccount(userID)
}
