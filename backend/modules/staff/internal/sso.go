package staff

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"manager-backend/framework"
)

const (
	SSOVerifyURL = "https://sso.xiaoxitech.com/api/sso/verifyToken"
	SSOLoginURL  = "https://sso.xiaoxitech.com/login"
)

// SSOService SSO服务单例
var SSOService *ssoServiceImpl

// InitSSOService 初始化SSO服务（在模块启动时调用，读取全局 AppConfig）
func InitSSOService() {
	SSOService = &ssoServiceImpl{}
}

type ssoServiceImpl struct{}

func (s *ssoServiceImpl) VerifyToken(token string) (*SSOUserInfo, error) {
	url := fmt.Sprintf("%s?project=%s&token=%s", SSOVerifyURL, framework.AppConfig.SSOProjectID, token)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求SSO验证接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取SSO响应失败: %w", err)
	}

	var verifyResp SSOVerifyResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return nil, fmt.Errorf("解析SSO响应失败: %w", err)
	}

	if verifyResp.E != "" {
		return nil, errors.New(verifyResp.E)
	}
	if verifyResp.P == nil {
		return nil, errors.New("SSO返回的用户信息为空")
	}
	return verifyResp.P, nil
}

func (s *ssoServiceImpl) LoginWithSSO(token string) (*Staff, error) {
	ssoUser, err := s.VerifyToken(token)
	if err != nil {
		return nil, err
	}

	username := ssoUser.WxUid
	displayName := strings.TrimSpace(ssoUser.Name)
	u, err := Service.GetStaffByUsername(username)
	if err != nil {
		newUser := &Staff{
			Username:       username,
			Name:           displayName,
			HashedPassword: "",
			IsActive:       true,
			IsSuperuser:    promoteIfFirst(username),
			Avatar:         ssoUser.Avatar,
		}
		if err := Service.CreateStaff(newUser); err != nil {
			return nil, fmt.Errorf("创建SSO用户失败: %w", err)
		}
		u, err = Service.GetStaffByUsername(username)
		if err != nil {
			return nil, fmt.Errorf("获取新创建的SSO用户失败: %w", err)
		}
	} else {
		if u.Avatar != ssoUser.Avatar {
			u.Avatar = ssoUser.Avatar
			if err := Service.UpdateStaff(u); err != nil {
				fmt.Printf("更新SSO用户头像失败: %v\n", err)
			}
		}
	}

	if !u.IsActive {
		return nil, errors.New("账号已被禁用")
	}
	return u, nil
}
