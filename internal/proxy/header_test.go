package 反向代理

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
)

// 客户端提供的换行/逗号式欺骗值不能变成多个转发跳。
func Test转发头不接受注入值(t *testing.T) {
	var 见到的XFF, 见到的Real string
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, 请求 *http.Request) {
		见到的XFF = 请求.Header.Get("X-Forwarded-For")
		见到的Real = 请求.Header.Get("X-Real-IP")
		写.WriteHeader(http.StatusOK)
	}))
	defer 上游.Close()

	解析器, 错误 := 客户端地址.U新建(nil)
	if 错误 != nil {
		t.Fatalf("resolver: %v", 错误)
	}
	引擎, 错误 := 引擎.U新建(引擎.U默认规则())
	if 错误 != nil {
		t.Fatalf("engine: %v", 错误)
	}
	处理器, 错误 := U新建(配置.U配置{
		U模式: "enforce",
		U站点: []配置.U站点{{U主机: "app.example.com", U上游: 上游.URL, U最大请求体: 1 << 20}},
	}, 解析器, 引擎, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if 错误 != nil {
		t.Fatalf("proxy: %v", 错误)
	}
	t.Cleanup(处理器.U关闭空闲连接)

	请求 := httptest.NewRequest("GET", "http://app.example.com/", nil)
	请求.RemoteAddr = "198.51.100.10:1234"
	请求.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	请求.Header.Set("X-Real-IP", "3.3.3.3")
	请求.Header.Set("X-Forwarded-Proto", "ftp")
	记录 := httptest.NewRecorder()
	处理器.ServeHTTP(记录, 请求)
	if 记录.Code != http.StatusOK {
		t.Fatalf("status=%d", 记录.Code)
	}
	if 见到的XFF != "198.51.100.10" {
		t.Fatalf("X-Forwarded-For=%q", 见到的XFF)
	}
	if 见到的Real != "198.51.100.10" {
		t.Fatalf("X-Real-IP=%q", 见到的Real)
	}
	if strings.Contains(见到的XFF, ",") {
		t.Fatalf("forwarded header kept multiple hops: %q", 见到的XFF)
	}
}
