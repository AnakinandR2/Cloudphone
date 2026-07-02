package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

// ProbeResult 是一次代理探测的结果：连通性（出口 IP + 延迟）+ 归属识别（国家/城市/ASN/公司）。
type ProbeResult struct {
	EgressIP  string
	LatencyMs int
	Country   string
	City      string
	ASN       string
	ASNName   string
	Company   string
	ConnType  string
}

// proberPort 是 proxy 模块的出站端口：实测一条代理是否可用并识别出口归属。
// 抽成接口便于单测注入假实现（不打真网），真实实现见 realProber。
type proberPort interface {
	Probe(ctx context.Context, p Proxy) (ProbeResult, error)
}

const (
	// 经代理访问该地址，响应里是「出口真实 IP」，用于判定连通性 + 取出口 IP。
	probeTargetURL = "http://myip.ipipgo.com/"
	// 用出口 IP 查归属（国家/城市/ASN/公司），直连不走代理。
	ipInfoURLFmt = "https://www.ipvibe.com/api/search?ip=%s"
)

var ipRe = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)

// realProber 用 golang.org/x/net/proxy 的 SOCKS5 拨号器实测代理。
type realProber struct{}

func newProber() proberPort { return &realProber{} }

func (rp *realProber) Probe(ctx context.Context, p Proxy) (ProbeResult, error) {
	// 1) 构造 SOCKS5 拨号器（带可选用户名/密码）。
	var auth *xproxy.Auth
	if p.Username != "" {
		auth = &xproxy.Auth{User: p.Username, Password: p.Password}
	}
	// SSRF 防护：拒绝把代理指向内网/环回/链路本地地址（否则可借探测做内网端口扫描），
	// 并 pin 到已校验的 IP 拨号，避免拨号时二次解析被 DNS rebinding 绕过（S2）。
	addr, err := resolvePublicHostPort(ctx, p.Host, p.Port)
	if err != nil {
		return ProbeResult{}, err
	}
	dialer, err := xproxy.SOCKS5("tcp", addr, auth, &net.Dialer{Timeout: 10 * time.Second})
	if err != nil {
		return ProbeResult{}, fmt.Errorf("构造 SOCKS5 拨号器失败: %w", err)
	}

	transport := &http.Transport{}
	if cd, ok := dialer.(xproxy.ContextDialer); ok {
		transport.DialContext = cd.DialContext
	} else {
		transport.DialContext = func(_ context.Context, network, address string) (net.Conn, error) {
			return dialer.Dial(network, address)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}

	// 2) 经代理访问取出口 IP + 测延迟。
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, probeTargetURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("经代理访问失败: %w", err)
	}
	defer resp.Body.Close()
	latency := int(time.Since(start).Milliseconds())
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	egressIP := ipRe.FindString(string(body))
	if egressIP == "" {
		// 不回显响应原始字节：避免把探测目标返回的内容当侧信道泄露（S2）。
		return ProbeResult{}, fmt.Errorf("未从响应解析出出口 IP")
	}

	res := ProbeResult{EgressIP: egressIP, LatencyMs: latency}

	// 3) 直连查归属（失败不致命：连通性已确认，归属留空即可）。
	if info, err := lookupIPInfo(ctx, egressIP); err == nil {
		res.Country, res.City = info.Country, info.City
		res.ASN, res.ASNName = info.ASN, info.ASNName
		res.Company, res.ConnType = info.Company, info.ConnType
	}
	return res, nil
}

// resolvePublicHostPort 解析代理 host → 校验所有 IP 均为公网地址（拒绝内网/环回/链路本地/
// 未指定/多播），返回 pin 到首个 IP 的 host:port 拨号地址（防拨号时二次解析被 DNS rebinding 绕过）。
func resolvePublicHostPort(ctx context.Context, host string, port int) (string, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("解析代理地址失败: %v", err)
	}
	for _, ip := range ips {
		if isDisallowedIP(ip) {
			return "", fmt.Errorf("代理地址不合法：禁止指向内网/环回/链路本地地址")
		}
	}
	return net.JoinHostPort(ips[0].String(), strconv.Itoa(port)), nil
}

// reservedCIDRs 是 net.IP.IsPrivate 未覆盖、但同样不应作为代理目标的网段：
// RFC6598 CGNAT（云/k8s 节点/Pod 常用内网段）、IETF 协议分配、基准测试、保留/未来用途。
var reservedCIDRs = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{
		"100.64.0.0/10", // RFC6598 CGNAT
		"192.0.0.0/24",  // IETF 协议分配
		"198.18.0.0/15", // 基准测试
		"240.0.0.0/4",   // 保留/未来用途
	} {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}()

// isDisallowedIP 报告 IP 是否属于禁止的私有/特殊网段（环回/私网/链路本地/未指定/多播，
// 含云元数据地址 169.254.169.254 与 CGNAT 100.64.0.0/10 等保留段）。
func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, n := range reservedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ipvibeResponse 对应 https://www.ipvibe.com/api/search 的返回结构（只取所需字段）。
type ipvibeResponse struct {
	Data struct {
		CurrentCountry struct {
			CountryCode string `json:"countryCode"`
			Province    string `json:"province"`
			City        string `json:"city"`
		} `json:"currentCountry"`
		ConnectionType string `json:"connectionType"`
		ASNInfo        struct {
			Name string `json:"name"`
			ASN  string `json:"asn"`
		} `json:"asnInfo"`
		Company struct {
			Name string `json:"name"`
		} `json:"company"`
	} `json:"data"`
}

type ipInfo struct {
	Country, City, ASN, ASNName, Company, ConnType string
}

// lookupIPInfo 直连 ipvibe 查询 IP 归属信息。
func lookupIPInfo(ctx context.Context, ip string) (ipInfo, error) {
	url := fmt.Sprintf(ipInfoURLFmt, ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ipInfo{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ipInfo{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return parseIPVibe(body)
}

// parseIPVibe 把 ipvibe 的响应体解析为归属信息（与网络解耦，便于单测）。
func parseIPVibe(body []byte) (ipInfo, error) {
	var r ipvibeResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return ipInfo{}, err
	}
	city := strings.TrimSpace(strings.Join(dedupNonEmpty(r.Data.CurrentCountry.Province, r.Data.CurrentCountry.City), " "))
	return ipInfo{
		Country:  r.Data.CurrentCountry.CountryCode,
		City:     city,
		ASN:      r.Data.ASNInfo.ASN,
		ASNName:  r.Data.ASNInfo.Name,
		Company:  r.Data.Company.Name,
		ConnType: r.Data.ConnectionType,
	}, nil
}

// dedupNonEmpty 过滤空串并去掉相邻重复（如 province==city 时只留一个）。
func dedupNonEmpty(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1] == p {
			continue
		}
		out = append(out, p)
	}
	return out
}
