package automation

import "manager-backend/framework/apperr"

// Service 模块内服务实例，由 module.Init 注入后装配。
var Service *serviceImpl

type serviceImpl struct {
	repo repository
	ops  midplatPort
}

func newService(repo repository, ops midplatPort) *serviceImpl {
	return &serviceImpl{repo: repo, ops: ops}
}

// requireOps 在中台未配置时给出统一错误。
func (s *serviceImpl) requireOps() error {
	if s.ops == nil {
		return apperr.Internal("云手机中台未配置")
	}
	return nil
}
