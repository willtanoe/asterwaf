package 管理接口

import (
	"net/http"
	"strings"
	"testing"
)

// 页面级统计端点：形状、认证与 200 行为。数据内容由 store 层测试覆盖。
func Test页面统计端点(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 登录客户端(t, 服务.URL)

	用例 := []struct {
		名称 string
		路径 string
		键  string
	}{
		{"rule stats", "/api/stats/rules?range=15m", "rules"},
		{"rule stats 7d", "/api/stats/rules?range=7d", "raw_available"},
		{"access stats", "/api/stats/access?range=15m", "top_offenders"},
		{"log summary", "/api/logs/summary?range=15m", "series"},
		{"site detail", "/api/sites/detail?host=app.example.com", "health_history"},
	}
	for _, 用例项 := range 用例 {
		t.Run(用例项.名称, func(t *testing.T) {
			响应 := 发送JSON(t, 客户端, http.MethodGet, 服务.URL+用例项.路径, nil)
			体 := 读取体(t, 响应)
			if 响应.StatusCode != http.StatusOK {
				t.Fatalf("status=%d body=%s", 响应.StatusCode, 体)
			}
			if !strings.Contains(体, `"`+用例项.键+`"`) {
				t.Fatalf("body missing %q", 用例项.键)
			}
		})
	}

	// 未认证一律拒绝。
	匿名响应, 错误 := http.Get(服务.URL + "/api/stats/rules?range=15m")
	if 错误 != nil {
		t.Fatalf("anonymous get: %v", 错误)
	}
	if 匿名响应.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", 匿名响应.StatusCode)
	}
	_ = 匿名响应.Body.Close()

	// site detail 必须要求 host。
	响应 := 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/sites/detail", nil)
	if 响应.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing host status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
}
