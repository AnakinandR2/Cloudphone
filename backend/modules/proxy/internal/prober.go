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
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
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
		return ProbeResult{}, fmt.Errorf("未从响应解析出出口 IP：%s", strings.TrimSpace(string(body)))
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
