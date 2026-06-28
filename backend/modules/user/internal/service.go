package user

import (
	"errors"
	"regexp"

	"manager-backend/framework/apperr"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// minPasswordLen 注册密码最小长度。
const minPasswordLen = 6

// phonePattern 中国大陆手机号格式。
var phonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

type serviceImpl struct{ repo repository }

// Service 前台用户服务单例；仓储由 module.Init 注入。
var Service = &serviceImpl{}

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// Register 注册前台用户。
func (s *serviceImpl) Register(req *RegisterRequest) (*User, error) {
	if !phonePattern.MatchString(req.Phone) {
		return nil, apperr.Validation("手机号格式不正确")
	}
	if len(req.Password) < minPasswordLen {
		return nil, apperr.Validation("密码至少 6 位")
	}

	exists, err := s.repo.existsByPhone(req.Phone)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("手机号已注册")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	c := UserDB{
		Phone:          req.Phone,
		Nickname:       req.Nickname,
		HashedPassword: string(hashed),
		IsActive:       true,
	}
	if err := s.repo.create(&c); err != nil {
		return nil, err
	}
	return dbToUser(&c), nil
}

// Authenticate 校验手机号+密码，成功返回用户。
func (s *serviceImpl) Authenticate(phone, password string) (*User, error) {
	c, err := s.repo.findByPhone(phone)
	if err != nil {
		return nil, apperr.Unauthorized("手机号或密码错误")
	}
	if bcrypt.CompareHashAndPassword([]byte(c.HashedPassword), []byte(password)) != nil {
		return nil, apperr.Unauthorized("手机号或密码错误")
	}
	if !c.IsActive {
		return nil, apperr.Forbidden("账号已被禁用")
	}
	return dbToUser(c), nil
}

// GetByID 获取前台用户。
func (s *serviceImpl) GetByID(id int) (*User, error) {
	c, err := s.repo.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("用户不存在")
	}
	return dbToUser(c), nil
}

// CurrentTokenVersion 返回用户当前令牌版本（鉴权中间件用于比对）。
func (s *serviceImpl) CurrentTokenVersion(id int) (int, error) {
	return s.repo.tokenVersion(id)
}

// Logout 递增令牌版本：使该用户此前签发的所有令牌立即失效（强制登出全部设备）。
func (s *serviceImpl) Logout(id int) error {
	return s.repo.bumpTokenVersion(id)
}

// --- 管理侧 ---

// AdminList 管理侧分页查询前台用户（可按手机号模糊、按启用状态过滤）。
func (s *serviceImpl) AdminList(page, size int, phone string, active *bool) ([]User, int64, error) {
	rows, total, err := s.repo.list((page-1)*size, size, phone, active)
	if err != nil {
		return nil, 0, err
	}
	list := make([]User, len(rows))
	for i := range rows {
		list[i] = *dbToUser(&rows[i])
	}
	return list, total, nil
}

// IDByPhone 精确按手机号查用户 ID（跨模块门面用，如 billing 按手机号过滤订单）。
// phone 为空或查不到返回 (0, false, nil)；其它库错误原样返回。
func (s *serviceImpl) IDByPhone(phone string) (uint, bool, error) {
	if phone == "" {
		return 0, false, nil
	}
	c, err := s.repo.findByPhone(phone)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return c.ID, true, nil
}

// PhonesByIDs 批量按用户 ID 取手机号（跨模块门面用，如 billing 订单列表回填手机号）。
func (s *serviceImpl) PhonesByIDs(ids []uint) (map[uint]string, error) {
	return s.repo.phonesByIDs(ids)
}

// SetActive 启用/禁用前台用户；禁用时递增令牌版本，令其已签发令牌立即失效（强制下线）。
func (s *serviceImpl) SetActive(id int, active bool) (*User, error) {
	if _, err := s.repo.findByID(id); err != nil {
		return nil, apperr.NotFound("用户不存在")
	}
	if err := s.repo.setActive(id, active); err != nil {
		return nil, err
	}
	if !active {
		if err := s.repo.bumpTokenVersion(id); err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}
