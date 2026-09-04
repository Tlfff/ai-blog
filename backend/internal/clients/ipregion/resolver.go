package ipregion

import (
	"net"
	"strings"
	"sync"

	"github.com/lionsoul2014/ip2region/binding/golang/service"
)

const defaultSearcherCount = 20

var countryNames = map[string]string{
	"United States": "美国", "Japan": "日本", "Singapore": "新加坡", "Australia": "澳大利亚",
	"Germany": "德国", "France": "法国", "United Kingdom": "英国", "Russia": "俄罗斯",
	"Canada": "加拿大", "Korea": "韩国", "South Korea": "韩国", "North Korea": "朝鲜",
	"India": "印度", "Vietnam": "越南", "Thailand": "泰国", "Malaysia": "马来西亚",
	"Indonesia": "印度尼西亚", "Philippines": "菲律宾", "Hong Kong": "中国香港",
	"Taiwan": "中国台湾", "Macao": "中国澳门",
}

// Resolver 是 ip2region 的基础设施适配器。
type Resolver struct {
	db    *service.Ip2Region // db 是支持 IPv4 和 IPv6 的内存查询服务。
	mutex sync.Mutex         // mutex 保护 BufferCache 模式下非并发安全的共享 Searcher。
}

// NewResolver 校验并同时加载 IPv4、IPv6 XDB，使用 BufferCache 和 20 个 Searcher。
func NewResolver(v4Path, v6Path string) (*Resolver, error) {
	// 1. 分别校验并加载 IPv4、IPv6 XDB 到内存
	v4, err := service.NewV4Config(service.BufferCache, v4Path, defaultSearcherCount)
	if err != nil {
		return nil, err
	}
	v6, err := service.NewV6Config(service.BufferCache, v6Path, defaultSearcherCount)
	if err != nil {
		return nil, err
	}
	// 2. 创建统一双栈查询服务
	db, err := service.NewIp2Region(v4, v6)
	if err != nil {
		return nil, err
	}
	return &Resolver{db: db}, nil
}

// Resolve 将 IP 地址解析为不泄漏原始地址的地区文案。
func (r *Resolver) Resolve(ip string) string {
	// 1. 对空地址、非法地址和内网地址返回稳定脱敏文案
	ip = strings.TrimSpace(ip)
	parsed := net.ParseIP(ip)
	if ip == "" || strings.EqualFold(ip, "localhost") || parsed == nil {
		if ip == "" || isPrivate(parsed) || strings.EqualFold(ip, "localhost") {
			return "内网"
		}
		return "未知"
	}
	if isPrivate(parsed) {
		return "内网"
	}
	// 2. 公网地址通过离线双栈 XDB 查询
	if r == nil || r.db == nil {
		return "未知"
	}
	r.mutex.Lock()
	value, err := r.db.Search(ip)
	r.mutex.Unlock()
	if err != nil || strings.TrimSpace(value) == "" {
		return "未知"
	}
	return normalize(value)
}

// Close 释放 IPv4 和 IPv6 XDB 搜索器资源。
func (r *Resolver) Close() {
	// 1. 关闭底层搜索器池；重复调用保持安全
	if r != nil && r.db != nil {
		r.mutex.Lock()
		defer r.mutex.Unlock()
		r.db.Close()
	}
}

// isPrivate 判断地址是否属于回环、私网或未指定地址。
func isPrivate(ip net.IP) bool {
	// 1. 统一识别回环、私网和未指定地址
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified())
}

// normalize 将 ip2region 的字段结果转换为产品约定的地区文案。
func normalize(value string) string {
	// 1. ip2region v4/v6 数据格式均为 国家|省份|城市|ISP|国家码
	parts := strings.Split(value, "|")
	if len(parts) < 5 {
		return "未知"
	}
	country := strings.TrimSpace(parts[0])
	province := strings.TrimSpace(parts[1])
	if country == "" || country == "0" {
		return "未知"
	}

	// 2. 中国地址返回省级地区，境外地址返回中文国家名或原始国家名
	if country == "中国" {
		province = strings.TrimSuffix(strings.TrimSuffix(province, "省"), "市")
		if province == "" || province == "0" {
			return "中国"
		}
		return province
	}
	return countryName(country)
}

// countryName 复用原项目的常见国家中文映射，并保留未命中的原始国家名。
func countryName(country string) string {
	// 1. 复用原项目完整映射，未命中时保留库返回的国家名
	if translated, ok := countryNames[country]; ok {
		return translated
	}
	return country
}
