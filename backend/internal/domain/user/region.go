package user

// IPRegionResolver 将原始 IP 转换为对外展示的地区文案。
type IPRegionResolver interface {
	// Resolve 返回脱敏地区文案，不返回原始 IP。
	Resolve(ip string) string
}
