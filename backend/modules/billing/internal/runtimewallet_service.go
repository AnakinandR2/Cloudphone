package billing

// runtimeWalletServiceImpl 临时开机时长余量服务（余量读取、封顶判定的薄封装）。
type runtimeWalletServiceImpl struct{ repo runtimeWalletRepository }

// RuntimeWalletService 模块内单例。
var RuntimeWalletService *runtimeWalletServiceImpl

func newRuntimeWalletService(repo runtimeWalletRepository) *runtimeWalletServiceImpl {
	return &runtimeWalletServiceImpl{repo: repo}
}

// Remaining 临时开机时长余量（分钟）。
func (s *runtimeWalletServiceImpl) Remaining(userID int) (int64, error) {
	return s.repo.remaining(userID)
}
