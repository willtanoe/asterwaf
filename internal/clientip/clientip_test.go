package 客户端地址

import (
	"net/http/httptest"
	"testing"
)

func Test解析客户端地址(t *testing.T) {
	测试用例 := []struct {
		名称   string
		受信   []string
		对端   string
		请求头  map[string]string
		协议   string
		期望地址 string
		对端受信 bool
	}{
		{
			名称:   "untrusted peer ignores forwarded headers",
			对端:   "198.51.100.7:1111",
			请求头:  map[string]string{"X-Forwarded-For": "203.0.113.9", "X-Forwarded-Proto": "https"},
			协议:   "http",
			期望地址: "198.51.100.7",
		},
		{
			名称:   "trusted peer takes rightmost untrusted hop",
			受信:   []string{"10.0.0.0/8"},
			对端:   "10.0.0.1:1111",
			请求头:  map[string]string{"X-Forwarded-For": "203.0.113.9, 10.0.0.2", "X-Forwarded-Proto": "https"},
			协议:   "https",
			期望地址: "203.0.113.9",
			对端受信: true,
		},
		{
			名称:   "trusted peer without forwarded for uses peer address",
			受信:   []string{"10.0.0.0/8"},
			对端:   "10.0.0.1:1111",
			协议:   "http",
			期望地址: "10.0.0.1",
			对端受信: true,
		},
		{
			名称:   "invalid rightmost entry falls back to last valid hop",
			受信:   []string{"10.0.0.0/8"},
			对端:   "10.0.0.1:1111",
			请求头:  map[string]string{"X-Forwarded-For": "203.0.113.9, not-an-ip"},
			协议:   "http",
			期望地址: "10.0.0.1",
			对端受信: true,
		},
		{
			名称:   "all hops trusted uses leftmost",
			受信:   []string{"10.0.0.0/8"},
			对端:   "10.0.0.1:1111",
			请求头:  map[string]string{"X-Forwarded-For": "10.0.0.5, 10.0.0.4"},
			协议:   "http",
			期望地址: "10.0.0.5",
			对端受信: true,
		},
		{
			名称:   "invalid forwarded proto is ignored for trusted peer",
			受信:   []string{"10.0.0.0/8"},
			对端:   "10.0.0.1:1111",
			请求头:  map[string]string{"X-Forwarded-Proto": "gopher"},
			协议:   "http",
			期望地址: "10.0.0.1",
			对端受信: true,
		},
		{
			名称:   "ipv4 mapped address is unmapped",
			对端:   "[::ffff:192.0.2.5]:2222",
			协议:   "http",
			期望地址: "192.0.2.5",
		},
	}
	for _, 用例 := range 测试用例 {
		t.Run(用例.名称, func(t *testing.T) {
			解析器, 错误 := U新建(用例.受信)
			if 错误 != nil {
				t.Fatalf("new resolver: %v", 错误)
			}
			请求 := httptest.NewRequest("GET", "http://app.example.com/", nil)
			请求.RemoteAddr = 用例.对端
			for 键, 值 := range 用例.请求头 {
				请求.Header.Set(键, 值)
			}
			结果 := 解析器.U解析(请求)
			if 结果.U地址.String() != 用例.期望地址 {
				t.Fatalf("client=%s, want %s", 结果.U地址, 用例.期望地址)
			}
			if 结果.U协议 != 用例.协议 {
				t.Fatalf("scheme=%s, want %s", 结果.U协议, 用例.协议)
			}
			if 结果.U受信对端 != 用例.对端受信 {
				t.Fatalf("peer trusted=%v, want %v", 结果.U受信对端, 用例.对端受信)
			}
		})
	}
}

func Test新建拒绝无效可信网段(t *testing.T) {
	if _, 错误 := U新建([]string{"10.0.0.0"}); 错误 == nil {
		t.Fatal("expected error for non-CIDR trusted proxy")
	}
}
