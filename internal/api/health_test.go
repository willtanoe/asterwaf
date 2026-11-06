package 管理接口

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
)

func Test健康探测状态(t *testing.T) {
	var 状态 atomic.Int32
	状态.Store(http.StatusOK)
	var 互斥 sync.Mutex
	var 见过主机, 见过路径 string
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, 请求 *http.Request) {
		互斥.Lock()
		见过主机 = 请求.Host
		见过路径 = 请求.URL.Path
		互斥.Unlock()
		写.WriteHeader(int(状态.Load()))
	}))
	defer 上游.Close()

	探测器 := U新建探测器([]配置.U站点{{
		U主机: "app.example.com", U上游: 上游.URL, U健康检查路径: "/health",
	}}, -1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer 探测器.Close()

	if 结果 := 探测器.U状态(); len(结果) != 1 || 结果[0].U状态 != "unknown" {
		t.Fatalf("initial status=%+v", 结果)
	}

	探测器.U探测一次()
	结果 := 探测器.U状态()[0]
	if 结果.U状态 != "up" || 结果.U失败次数 != 0 {
		t.Fatalf("healthy probe=%+v", 结果)
	}
	互斥.Lock()
	if 见过主机 != "app.example.com" || 见过路径 != "/health" {
		t.Fatalf("probe host=%q path=%q", 见过主机, 见过路径)
	}
	互斥.Unlock()

	状态.Store(http.StatusInternalServerError)
	for 次数 := 0; 次数 < 2; 次数++ {
		探测器.U探测一次()
	}
	if 结果 := 探测器.U状态()[0]; 结果.U状态 == "down" {
		t.Fatal("marked down before three consecutive failures")
	}
	探测器.U探测一次()
	结果 = 探测器.U状态()[0]
	if 结果.U状态 != "down" || 结果.U失败次数 < 失败判定次数 {
		t.Fatalf("down probe=%+v", 结果)
	}

	状态.Store(http.StatusOK)
	探测器.U探测一次()
	if 结果 := 探测器.U状态()[0]; 结果.U状态 != "up" || 结果.U失败次数 != 0 {
		t.Fatalf("recovery probe=%+v", 结果)
	}
}

func Test健康探测不跟随重定向(t *testing.T) {
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		写.Header().Set("Location", "http://127.0.0.1:1/never")
		写.WriteHeader(http.StatusFound)
	}))
	defer 上游.Close()

	探测器 := U新建探测器([]配置.U站点{{
		U主机: "app.example.com", U上游: 上游.URL, U健康检查路径: "/",
	}}, -1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer 探测器.Close()
	探测器.U探测一次()
	if 结果 := 探测器.U状态()[0]; 结果.U状态 != "up" {
		t.Fatalf("3xx should count as healthy: %+v", 结果)
	}
}

func Test健康地址保留上游基础路径(t *testing.T) {
	站点 := 配置.U站点{U上游: "http://127.0.0.1:9/base/", U健康检查路径: "/health"}
	if 地址 := 健康地址(站点); !strings.HasSuffix(地址, "/base/health") {
		t.Fatalf("health address=%q", 地址)
	}
}
