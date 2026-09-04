package ipregion

import (
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// resourcePaths 返回仓库内双栈 XDB 的绝对路径，不依赖测试工作目录。
func resourcePaths(t *testing.T) (string, string) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法获取 IP 地区测试文件路径")
	}
	resourceDir := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "assets", "ipregion")
	v4Path, err := filepath.Abs(filepath.Join(resourceDir, "ip2region.xdb"))
	if err != nil {
		t.Fatalf("resolve IPv4 XDB path: %v", err)
	}
	v6Path, err := filepath.Abs(filepath.Join(resourceDir, "ip2region_v6.xdb"))
	if err != nil {
		t.Fatalf("resolve IPv6 XDB path: %v", err)
	}
	return v4Path, v6Path
}

// TestResolverInvalidAndPrivateIPs 验证无需访问 XDB 的边界地址行为。
func TestResolverInvalidAndPrivateIPs(t *testing.T) {
	resolver := &Resolver{}
	for _, test := range []struct {
		name string
		ip   string
		want string
	}{
		{name: "空地址", ip: "", want: "内网"},
		{name: "本地主机", ip: "localhost", want: "内网"},
		{name: "IPv4 回环", ip: "127.0.0.1", want: "内网"},
		{name: "IPv6 回环", ip: "::1", want: "内网"},
		{name: "私网", ip: "192.168.1.1", want: "内网"},
		{name: "非法地址", ip: "not-an-ip", want: "未知"},
		{name: "未配置解析器", ip: "8.8.8.8", want: "未知"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := resolver.Resolve(test.ip); got != test.want {
				t.Fatalf("Resolve(%q) = %q, want %q", test.ip, got, test.want)
			}
		})
	}
}

// TestResolverRealXDB 验证真实双栈 XDB 的国内、境外解析结果。
func TestResolverRealXDB(t *testing.T) {
	v4Path, v6Path := resourcePaths(t)
	resolver, err := NewResolver(v4Path, v6Path)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	defer resolver.Close()

	tests := []struct {
		name string // name 是测试场景名称。
		ip   string // ip 是待查询公网地址。
		want string // want 是期望地区文案。
	}{
		{name: "IPv4 国内", ip: "223.5.5.5", want: "浙江"},
		{name: "IPv6 国内", ip: "2001:250:200::", want: "北京"},
		{name: "IPv4 境外", ip: "8.8.8.8", want: "美国"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolver.Resolve(test.ip); got != test.want {
				t.Fatalf("Resolve(%q) = %q, want %q", test.ip, got, test.want)
			}
		})
	}
}

// TestNormalizeRegion 验证国内省级文案和境外国家文案的脱敏转换。
func TestNormalizeRegion(t *testing.T) {
	cases := map[string]string{
		"中国|广东省|深圳市|电信|CN":                        "广东",
		"中国|0|0|0|0":                              "中国",
		"United States|0|California|Los Angeles|": "美国",
		"Canada|0|Ontario|Toronto|":               "加拿大",
		"Atlantis|0|0|0|":                         "Atlantis",
	}
	for input, want := range cases {
		if got := normalize(input); got != want {
			t.Errorf("normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestResolverConcurrentQueries 验证边界查询可并发执行且不会修改共享状态。
func TestResolverConcurrentQueries(t *testing.T) {
	v4Path, v6Path := resourcePaths(t)
	resolver, err := NewResolver(v4Path, v6Path)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	defer resolver.Close()
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if got := resolver.Resolve("223.5.5.5"); got != "浙江" {
				t.Errorf("Resolve() = %q, want 浙江", got)
			}
		}()
	}
	group.Wait()
}
