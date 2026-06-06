package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 用户给的 ipvibe 真实样例：验证我们从中正确提取国家/城市/ASN/公司/连接类型。
const ipvibeSample = `{"code":200,"success":true,"data":{"ip":"156.59.87.47","registeredCountry":"CN","currentCountry":{"country":"CN","countryCode":"CN","province":"Hong Kong","city":"Hong Kong","geoLocation":{"lat":"22.2783","lon":"114.1747"},"timeZone":"Asia/Hong Kong Time","postalCode":"999077"},"numericAddress":"2621134639","connectionType":"Corporate","cidr":"156.59.80.0/21","asnInfo":{"name":"Zenlayer Inc","asn":"AS21859","type":"Hosting","domain":"zenlayer.com"},"company":{"name":"Zenlayer IP Block @Hong Kong","type":"Hosting","domain":"zenlayer.com"},"isAnyCast":false,"isSatellite":false},"message":"success"}`

func TestParseIPVibe(t *testing.T) {
	info, err := parseIPVibe([]byte(ipvibeSample))
	require.NoError(t, err)
	assert.Equal(t, "CN", info.Country)
	assert.Equal(t, "Hong Kong", info.City) // province==city → 去重后只保留一个
	assert.Equal(t, "AS21859", info.ASN)
	assert.Equal(t, "Zenlayer Inc", info.ASNName)
	assert.Equal(t, "Zenlayer IP Block @Hong Kong", info.Company)
	assert.Equal(t, "Corporate", info.ConnType)
}

func TestParseIPVibeBadJSON(t *testing.T) {
	_, err := parseIPVibe([]byte("not-json"))
	assert.Error(t, err)
}

func TestParseIPVibeDifferentProvinceCity(t *testing.T) {
	body := `{"data":{"currentCountry":{"countryCode":"US","province":"California","city":"Los Angeles"},"asnInfo":{"asn":"AS123","name":"Foo"},"company":{"name":"Bar"},"connectionType":"Hosting"}}`
	info, err := parseIPVibe([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, "US", info.Country)
	assert.Equal(t, "California Los Angeles", info.City) // province != city → 拼接
}

// 出口 IP 正则：从 myip 各种响应体里抓出第一个 IPv4。
func TestExtractIPRegex(t *testing.T) {
	cases := map[string]string{
		"156.59.87.47":              "156.59.87.47",
		"Your IP is 156.59.87.47\n": "156.59.87.47",
		`{"ip":"203.0.113.9"}`:      "203.0.113.9",
		"  10.0.0.1  ":              "10.0.0.1",
		"no ip here":                "",
		"":                          "",
	}
	for body, want := range cases {
		assert.Equal(t, want, ipRe.FindString(body), "body=%q", body)
	}
}

func TestDedupNonEmpty(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, dedupNonEmpty("a", "", "b"))
	assert.Equal(t, []string{"x"}, dedupNonEmpty("x", "x")) // 相邻重复去掉
	assert.Equal(t, []string{"HK"}, dedupNonEmpty("", " HK ", "HK"))
	assert.Empty(t, dedupNonEmpty("", "  ", ""))
}
