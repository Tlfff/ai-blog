package requestip

import (
	"net"
	"net/http"
	"strings"
)

// FromRequest 仅在请求来源属于受信代理时读取转发头，否则使用直接连接地址。
func FromRequest(r *http.Request, trustedProxyCIDRs []string) string {
	// 1. 非受信直连来源不得使用任何转发头
	remote := host(r.RemoteAddr)
	if !isTrusted(remote, trustedProxyCIDRs) {
		return remote
	}

	// 2. 从右向左跳过受信代理，选择最接近服务端的非受信客户端地址
	forwarded := splitForwardedFor(r.Header.Get("X-Forwarded-For"))
	for index := len(forwarded) - 1; index >= 0; index-- {
		candidate := forwarded[index]
		if net.ParseIP(candidate) != nil && !isTrusted(candidate, trustedProxyCIDRs) {
			return candidate
		}
	}

	// 3. 无有效转发链时回退到可信代理声明的单一真实地址
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(realIP) != nil {
		return realIP
	}
	return remote
}

// splitForwardedFor 拆分并清理 X-Forwarded-For 地址链。
func splitForwardedFor(value string) []string {
	// 1. 保留地址顺序并过滤空元素
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if candidate := strings.TrimSpace(part); candidate != "" {
			result = append(result, candidate)
		}
	}
	return result
}

// host 从 RemoteAddr 中提取不带端口的主机地址。
func host(value string) string {
	// 1. 优先按带端口地址解析，失败时保留原始主机文本
	value = strings.TrimSpace(value)
	if h, _, err := net.SplitHostPort(value); err == nil {
		return h
	}
	return value
}

// isTrusted 判断直接连接地址是否落在受信代理 CIDR 中。
func isTrusted(ip string, cidrs []string) bool {
	// 1. 仅有效 IP 才参与受信网段匹配
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range cidrs {
		// 2. 配置在启动阶段已经校验，此处仅执行快速匹配
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(parsed) {
			return true
		}
	}
	return false
}
