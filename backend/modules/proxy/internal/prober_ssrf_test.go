package proxy

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S2 回归：代理探测拨号前必须拒绝内网/环回/链路本地地址，防被当内网端口探测原语。

func TestResolvePublicHostPort_RejectsPrivateAndLocal(t *testing.T) {
	ctx := context.Background()
	for _, host := range []string{
		"127.0.0.1",       // 环回
		"::1",             // IPv6 环回
		"10.0.0.1",        // 私网 A
		"172.16.0.1",      // 私网 B
		"192.168.1.1",     // 私网 C
		"169.254.169.254", // 链路本地（云元数据）
		"100.64.0.1",      // CGNAT（RFC6598，云/k8s 内网）
		"0.0.0.0",         // 未指定
	} {
		_, err := resolvePublicHostPort(ctx, host, 6379)
		require.Error(t, err, "应拒绝内网/环回/链路本地地址: %s", host)
	}
}

func TestResolvePublicHostPort_AllowsPublic(t *testing.T) {
	addr, err := resolvePublicHostPort(context.Background(), "8.8.8.8", 1080)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8:1080", addr, "公网地址应通过并 pin 到该 IP")
}

func TestIsDisallowedIP(t *testing.T) {
	disallowed := []string{"127.0.0.1", "10.1.2.3", "192.168.0.5", "169.254.169.254", "100.64.1.2", "::1", "0.0.0.0", "224.0.0.1"}
	for _, s := range disallowed {
		assert.True(t, isDisallowedIP(net.ParseIP(s)), "应判定为禁止: %s", s)
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "203.0.113.10"} {
		assert.False(t, isDisallowedIP(net.ParseIP(s)), "公网地址不应被禁止: %s", s)
	}
}
