package proxy

import (
	"context"
	"strings"
	"time"

	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

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

// orderable 列表允许排序的字段白名单（杜绝 SQL 注入）。
var orderable = map[string]bool{"id": true, "name": true, "latency": true, "created_at": true}

func normalizeProtocol(p string) string {
	if p == "" {
		return "socks5"
	}
	return p
}

// --- 前台（属主隔离）---

func (s *serviceImpl) GetList(userID, page, size int, kw, order, sort string) ([]Proxy, int64, error) {
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
	item := Proxy{
		UserID:   uint(userID),
		Name:     req.Name,
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

// BatchCreate 批量为当前用户新增代理：跳过缺 host/port 的无效条目，返回成功条数。
func (s *serviceImpl) BatchCreate(userID int, items []ProxyCreate) (int, error) {
	created := 0
	for i := range items {
		it := items[i]
		if it.Host == "" || it.Port == 0 {
			continue
		}
		if _, err := s.Create(userID, &it); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func (s *serviceImpl) Update(userID, id int, req *ProxyUpdate) (*Proxy, error) {
	if _, err := s.GetByID(userID, id); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.Protocol != "" {
		fields["protocol"] = req.Protocol
	}
	if req.Host != "" {
		fields["host"] = req.Host
	}
	if req.Port != 0 {
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
