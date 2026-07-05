package proxy

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

// maxProxyNameLen 代理名称最大长度（对齐 model 的 varchar(100)）。
const maxProxyNameLen = 100

// validateProxyPort 端口须在合法范围 1-65535（CP-0023 / #28）。
func validateProxyPort(port int) error {
	if port < 1 || port > 65535 {
		return apperr.Validation("端口必须在 1-65535 之间")
	}
	return nil
}

// validateProxyName 名称必填且不超长（CP-0055 / #48）。
func validateProxyName(name string) error {
	if name == "" {
		return apperr.Validation("名称不能为空")
	}
	if utf8.RuneCountInString(name) > maxProxyNameLen {
		return apperr.Validation("名称长度不能超过 100 个字符")
	}
	return nil
}

// validateProxyProtocol 协议白名单（CP-0027 / #32）：本产品仅支持 SOCKS5（列表定位与探测均为
// SOCKS5 拨号）。留空按默认 socks5；显式传入其他（如 http）一律拒绝，杜绝非 SOCKS5 协议入库。
func validateProxyProtocol(protocol string) error {
	p := strings.ToLower(strings.TrimSpace(protocol))
	if p == "" || p == "socks5" {
		return nil
	}
	return apperr.Validation("仅支持 socks5 代理协议")
}

// serviceImpl 代理业务服务，依赖注入的 repository 与探测端口 prober。
// 前台方法都带 userID：只操作「当前用户自己的」代理；Admin* 方法供后台运营。
type serviceImpl struct {
	repo   repository
	prober proberPort
}

// ProxyService 模块内服务实例，由 module.Init 注入 DB 后装配。
var ProxyService *serviceImpl

func newService(repo repository, prober proberPort) *serviceImpl {
	return &serviceImpl{repo: repo, prober: prober}
}

// TestProxy 实测一条本人代理：经 SOCKS5 访问取出口 IP + 延迟（连通性），
// 并用出口 IP 自动识别归属（国家/城市/ASN/公司），结果写回该代理记录后返回。
// 探测失败（超时/拒绝）也是有效结果：记 status=fail，不当作接口错误。
func (s *serviceImpl) TestProxy(userID, id int) (*Proxy, error) {
	p, err := s.GetByID(userID, id) // 非本人 → NotFound
	if err != nil {
		return nil, err
	}
	if s.prober == nil {
		return nil, apperr.Internal("代理探测能力未启用")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	now := time.Now()
	fields := map[string]interface{}{"last_checked_at": now}
	if res, perr := s.prober.Probe(ctx, *p); perr != nil {
		fields["status"] = StatusFail
		fields["latency"] = 0
		fields["egress_ip"] = ""
	} else {
		fields["status"] = StatusOK
		fields["latency"] = res.LatencyMs
		fields["egress_ip"] = res.EgressIP
		fields["country"] = res.Country
		fields["city"] = res.City
		fields["asn"] = res.ASN
		fields["asn_name"] = res.ASNName
		fields["company"] = res.Company
		fields["conn_type"] = res.ConnType
		// 自动识别到的地区回填 region（便于列表统一展示）。
		if region := strings.TrimSpace(res.Country + " " + res.City); region != "" {
			fields["region"] = region
		}
	}
	if err := s.repo.update(userID, id, fields); err != nil {
		return nil, err
	}
	return s.GetByID(userID, id)
}

// ProbeOwned 对本人某代理做一次连通性探测（不落库），供开机前「短超时」校验复用：
// 调用方传入带较短超时的 ctx 控制耗时。返回 nil=连通；err=不可用（超时/拒绝/非属主）。
func (s *serviceImpl) ProbeOwned(ctx context.Context, userID, id int) error {
	p, err := s.GetByID(userID, id) // 非本人 → NotFound
	if err != nil {
		return err
	}
	if s.prober == nil {
		return apperr.Internal("代理探测能力未启用")
	}
	if _, err := s.prober.Probe(ctx, *p); err != nil {
		return err
	}
	return nil
}

// orderable 列表允许排序的字段白名单（杜绝 SQL 注入）。
var orderable = map[string]bool{"id": true, "name": true, "latency": true, "created_at": true}

// normalizeProtocol 规范化协议：去空白并小写；留空回落 socks5。
// 配合 validateProxyProtocol（白名单仅 socks5），落库值恒为规范化后的 "socks5"。
func normalizeProtocol(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "socks5"
	}
	return p
}

// --- 前台（属主隔离）---

func (s *serviceImpl) GetList(userID, page, size int, kw, order, sort string) ([]Proxy, int64, error) {
	// 分页下界 clamp（CP-0004）：size=0/负值或超上限直进 GORM Limit 会返空/全表，回落默认。
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	total, err := s.repo.count(userID, kw)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.list(userID, (page-1)*size, size, kw, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListOptions 返回当前用户「可用」的全部代理（不分页），供绑定代理下拉选择。
// 这里把探测失败（fail）的过滤掉，未检测（unknown）与正常（ok）都视为可选。
func (s *serviceImpl) ListOptions(userID int) ([]Proxy, error) {
	all, err := s.repo.listAllOwned(userID)
	if err != nil {
		return nil, err
	}
	out := make([]Proxy, 0, len(all))
	for _, p := range all {
		if p.Status == StatusFail {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *serviceImpl) GetByID(userID, id int) (*Proxy, error) {
	item, err := s.repo.findByID(userID, id)
	if err != nil {
		return nil, apperr.NotFound("代理不存在")
	}
	return item, nil
}

func (s *serviceImpl) Create(userID int, req *ProxyCreate) (*Proxy, error) {
	name := strings.TrimSpace(req.Name)
	if err := validateProxyName(name); err != nil {
		return nil, err
	}
	if err := validateProxyPort(req.Port); err != nil {
		return nil, err
	}
	// 协议白名单：仅 socks5（CP-0027 / #32）。
	if err := validateProxyProtocol(req.Protocol); err != nil {
		return nil, err
	}
	// 名称同属主唯一（CP-0063 / #55）。
	exists, err := s.repo.existsByName(userID, name, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("代理名称已存在")
	}
	item := Proxy{
		UserID:   uint(userID),
		Name:     name,
		Protocol: normalizeProtocol(req.Protocol),
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Region:   req.Region,
		Status:   StatusUnknown,
		Remark:   req.Remark,
	}
	if err := s.repo.create(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

// Probe 按参数即时探测一条代理（不落库），供添加/编辑表单测试展示。
// 探测失败（超时/拒绝）返回 status=fail + message，非接口错误。
func (s *serviceImpl) Probe(req *ProxyProbeRequest) (*ProbeOutcome, error) {
	if s.prober == nil {
		return nil, apperr.Internal("代理探测能力未启用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	p := Proxy{Protocol: "socks5", Host: req.Host, Port: req.Port, Username: req.Username, Password: req.Password}
	res, err := s.prober.Probe(ctx, p)
	if err != nil {
		return &ProbeOutcome{Status: StatusFail, Message: err.Error()}, nil
	}
	return &ProbeOutcome{
		Status:   StatusOK,
		Latency:  res.LatencyMs,
		EgressIP: res.EgressIP,
		Country:  res.Country,
		City:     res.City,
		ASN:      res.ASN,
		ASNName:  res.ASNName,
		Company:  res.Company,
		ConnType: res.ConnType,
	}, nil
}

// BatchCreate 批量为当前用户新增代理（CP-0055 / #48）：逐行校验主机/端口/名称，任一非法即整批
// 拒绝并指明行号（不再静默跳过/落库脏数据）。名称导入规则：留空则以 host:port 兜底；名称须在
// 「存量 + 批内」同属主唯一（CP-0063 / #55），重复即报错。全部通过后一次性 INSERT，返回条数。
func (s *serviceImpl) BatchCreate(userID int, items []ProxyCreate) (int, error) {
	existing, err := s.repo.namesOwned(userID)
	if err != nil {
		return 0, err
	}
	used := make(map[string]bool, len(existing)+len(items))
	for _, n := range existing {
		used[n] = true
	}
	toCreate := make([]Proxy, 0, len(items))
	for i := range items {
		it := items[i]
		row := i + 1
		host := strings.TrimSpace(it.Host)
		if host == "" {
			return 0, apperr.Validation(fmt.Sprintf("第 %d 行：缺少主机地址", row))
		}
		if err := validateProxyPort(it.Port); err != nil {
			return 0, apperr.Validation(fmt.Sprintf("第 %d 行：端口必须在 1-65535 之间", row))
		}
		// 协议白名单：仅 socks5（CP-0027 / #32）。
		if err := validateProxyProtocol(it.Protocol); err != nil {
			return 0, apperr.Validation(fmt.Sprintf("第 %d 行：仅支持 socks5 代理协议", row))
		}
		// 名称导入规则：缺省用 host:port 兜底。
		name := strings.TrimSpace(it.Name)
		if name == "" {
			name = fmt.Sprintf("%s:%d", host, it.Port)
		}
		if err := validateProxyName(name); err != nil {
			return 0, apperr.Validation(fmt.Sprintf("第 %d 行：%s", row, err.Error()))
		}
		if used[name] {
			return 0, apperr.Conflict(fmt.Sprintf("第 %d 行：代理名称「%s」重复", row, name))
		}
		used[name] = true
		toCreate = append(toCreate, Proxy{
			UserID:   uint(userID),
			Name:     name,
			Protocol: normalizeProtocol(it.Protocol),
			Host:     host,
			Port:     it.Port,
			Username: it.Username,
			Password: it.Password,
			Region:   it.Region,
			Status:   StatusUnknown,
			Remark:   it.Remark,
		})
	}
	if len(toCreate) == 0 {
		return 0, nil
	}
	if err := s.repo.createBatch(toCreate); err != nil {
		return 0, err
	}
	return len(toCreate), nil
}

func (s *serviceImpl) Update(userID, id int, req *ProxyUpdate) (*Proxy, error) {
	if _, err := s.GetByID(userID, id); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.Name != "" {
		name := strings.TrimSpace(req.Name)
		if err := validateProxyName(name); err != nil {
			return nil, err
		}
		// 改名时同属主查重（排除自身；CP-0063 / #55）。
		exists, err := s.repo.existsByName(userID, name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperr.Conflict("代理名称已存在")
		}
		fields["name"] = name
	}
	if req.Protocol != "" {
		// 协议白名单：仅 socks5（CP-0027 / #32）；落库前规范化。
		if err := validateProxyProtocol(req.Protocol); err != nil {
			return nil, err
		}
		fields["protocol"] = normalizeProtocol(req.Protocol)
	}
	if req.Host != "" {
		fields["host"] = req.Host
	}
	if req.Port != 0 {
		if err := validateProxyPort(req.Port); err != nil {
			return nil, err
		}
		fields["port"] = req.Port
	}
	if req.Username != "" {
		fields["username"] = req.Username
	}
	if req.Password != "" {
		fields["password"] = req.Password
	}
	if req.Region != "" {
		fields["region"] = req.Region
	}
	if req.Remark != "" {
		fields["remark"] = req.Remark
	}
	if len(fields) > 0 {
		if err := s.repo.update(userID, id, fields); err != nil {
			return nil, err
		}
	}
	return s.GetByID(userID, id)
}

func (s *serviceImpl) Delete(userID, id int) error {
	if _, err := s.GetByID(userID, id); err != nil {
		return err
	}
	return s.repo.delete(userID, id)
}

// --- 管理侧（不限属主）---

func (s *serviceImpl) AdminList(page, size int, kw, status, order, sort string) ([]Proxy, int64, error) {
	// 分页下界 clamp（CP-0004）：与 GetList 同源，避免 size=0/负值返空、超大 size 全表扫描。
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	total, err := s.repo.adminCount(kw, status)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.adminList((page-1)*size, size, kw, status, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *serviceImpl) AdminGetByID(id int) (*Proxy, error) {
	item, err := s.repo.adminFindByID(id)
	if err != nil {
		return nil, apperr.NotFound("代理不存在")
	}
	return item, nil
}

func (s *serviceImpl) AdminDelete(id int) error {
	if _, err := s.AdminGetByID(id); err != nil {
		return err
	}
	return s.repo.adminDelete(id)
}
