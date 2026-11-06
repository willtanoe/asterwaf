package 配置

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test配置往返(t *testing.T) {
	原始 := U配置{
		U监听:   "127.0.0.1:8080",
		U管理监听: "127.0.0.1:9090",
		U模式:   "shadow",
		U受信代理: []string{"10.0.0.0/8", "192.168.1.0/24"},
		U检查头:  []string{"User-Agent", "X-Custom"},
		U站点: []U站点{
			{
				U主机:     "app.example.com",
				U上游:     "http://127.0.0.1:3000",
				U模式:     "enforce",
				U最大请求体:  2048,
				U健康检查路径: "/health",
			},
		},
	}
	数据, 错误 := yaml.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("marshal: %v", 错误)
	}
	var 还原 U配置
	if 错误 := yaml.Unmarshal(数据, &还原); 错误 != nil {
		t.Fatalf("unmarshal: %v", 错误)
	}
	if !reflect.DeepEqual(原始, 还原) {
		t.Fatalf("round trip mismatch:\noriginal=%+v\nrestored=%+v", 原始, 还原)
	}
}

func Test加载应用默认值(t *testing.T) {
	路径 := 写临时配置(t, `
listen: 127.0.0.1:8080
admin_listen: 127.0.0.1:9091
mode: enforce
sites:
  - host: App.Example.com.
    upstream: http://127.0.0.1:3000
`)
	配置值, 错误 := U加载(路径)
	if 错误 != nil {
		t.Fatalf("load: %v", 错误)
	}
	站点 := 配置值.U站点[0]
	if 站点.U主机 != "app.example.com" {
		t.Fatalf("host not normalized: %q", 站点.U主机)
	}
	if 站点.U最大请求体 != U默认最大请求体 {
		t.Fatalf("max body default = %d", 站点.U最大请求体)
	}
	if 站点.U健康检查路径 != 默认健康路径 {
		t.Fatalf("health path default = %q", 站点.U健康检查路径)
	}
	if 站点.U模式 != "enforce" {
		t.Fatalf("site mode did not inherit global: %q", 站点.U模式)
	}
}

func Test加载拒绝未知键(t *testing.T) {
	路径 := 写临时配置(t, `
listen: 127.0.0.1:8080
admin_listen: 127.0.0.1:9091
mode: enforce
typo_key: true
sites:
  - host: app.example.com
    upstream: http://127.0.0.1:3000
`)
	if _, 错误 := U加载(路径); 错误 == nil {
		t.Fatal("expected unknown key to be rejected")
	}
}

func Test校验拒绝无效配置(t *testing.T) {
	测试用例 := map[string]U配置{
		"empty sites": {
			U监听: "127.0.0.1:8080", U管理监听: "127.0.0.1:9091", U模式: "enforce",
		},
		"bad global mode": {
			U监听: "127.0.0.1:8080", U管理监听: "127.0.0.1:9091", U模式: "watch",
			U站点: []U站点{{U主机: "a.example.com", U上游: "http://127.0.0.1:1"}},
		},
		"bad upstream scheme": {
			U监听: "127.0.0.1:8080", U管理监听: "127.0.0.1:9091", U模式: "enforce",
			U站点: []U站点{{U主机: "a.example.com", U上游: "ftp://127.0.0.1:1"}},
		},
		"bad wildcard": {
			U监听: "127.0.0.1:8080", U管理监听: "127.0.0.1:9091", U模式: "enforce",
			U站点: []U站点{{U主机: "a.*.example.com", U上游: "http://127.0.0.1:1"}},
		},
		"duplicate host": {
			U监听: "127.0.0.1:8080", U管理监听: "127.0.0.1:9091", U模式: "enforce",
			U站点: []U站点{
				{U主机: "a.example.com", U上游: "http://127.0.0.1:1"},
				{U主机: "A.example.com", U上游: "http://127.0.0.1:2"},
			},
		},
		"bad trusted proxy": {
			U监听: "127.0.0.1:8080", U管理监听: "127.0.0.1:9091", U模式: "enforce",
			U受信代理: []string{"not-a-cidr"},
			U站点:   []U站点{{U主机: "a.example.com", U上游: "http://127.0.0.1:1"}},
		},
	}
	for 名称, 配置值 := range 测试用例 {
		t.Run(名称, func(t *testing.T) {
			if 错误 := 配置值.U校验(); 错误 == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func Test按主机取站点(t *testing.T) {
	配置值 := U配置{
		U站点: []U站点{
			{U主机: "*.example.com"},
			{U主机: "*.api.example.com"},
			{U主机: "app.example.com"},
		},
	}
	测试用例 := []struct {
		主机 string
		期望 string
		命中 bool
	}{
		{"app.example.com", "app.example.com", true},
		{"APP.EXAMPLE.COM", "app.example.com", true},
		{"app.example.com:8080", "", false},
		{"x.api.example.com", "*.api.example.com", true},
		{"api.example.com", "*.example.com", true},
		{"x.example.com", "*.example.com", true},
		{"example.com", "", false},
		{"other.test", "", false},
	}
	for _, 用例 := range 测试用例 {
		t.Run(用例.主机, func(t *testing.T) {
			站点, 命中 := 配置值.U按主机取站点(用例.主机)
			if 命中 != 用例.命中 {
				t.Fatalf("matched=%v, want %v", 命中, 用例.命中)
			}
			if 命中 && 站点.U主机 != 用例.期望 {
				t.Fatalf("site=%q, want %q", 站点.U主机, 用例.期望)
			}
		})
	}
}

func 写临时配置(t *testing.T, 内容 string) string {
	t.Helper()
	路径 := filepath.Join(t.TempDir(), "config.yaml")
	if 错误 := os.WriteFile(路径, []byte(内容), 0o600); 错误 != nil {
		t.Fatalf("write config: %v", 错误)
	}
	return 路径
}
