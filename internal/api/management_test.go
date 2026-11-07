package 管理接口

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

func Test规则管理API(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 登录客户端(t, 服务.URL)

	响应 := 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/rules", nil)
	var 列表 U规则列表响应
	解码(t, 响应, http.StatusOK, &列表)
	if len(列表.U规则) < 20 || 列表.U版本 == 0 {
		t.Fatalf("default rules not seeded: %+v", 列表)
	}

	新规则 := U规则输入{UID: "custom.evil", U名称: "Custom evil", U类别: "sqli", U正则: "evilword", U启用: true, UScore: 5}
	响应 = 发送JSON(t, 客户端, http.MethodPost, 服务.URL+"/api/rules", 新规则)
	var 保存 U规则保存响应
	解码(t, 响应, http.StatusCreated, &保存)
	if 保存.U版本 <= 列表.U版本 || 保存.U规则.U来源 != "custom" {
		t.Fatalf("create response=%+v", 保存)
	}
	第一版本 := 保存.U版本

	响应 = 发送JSON(t, 客户端, http.MethodPost, 服务.URL+"/api/rules/test", U规则测试请求{
		U规则: 新规则, U载荷: "this contains evilword",
	})
	var 测试 U规则测试响应
	解码(t, 响应, http.StatusOK, &测试)
	if !测试.U命中 || 测试.U规则ID != "custom.evil" {
		t.Fatalf("rule test response=%+v", 测试)
	}

	响应 = 发送JSON(t, 客户端, http.MethodPost, 服务.URL+"/api/rules/custom.evil/toggle", nil)
	解码(t, 响应, http.StatusOK, &保存)
	if 保存.U规则.U启用 {
		t.Fatal("toggle did not disable rule")
	}

	更新 := 新规则
	更新.UID = ""
	更新.U正则 = "worseword"
	更新.U启用 = true
	响应 = 发送JSON(t, 客户端, http.MethodPut, 服务.URL+"/api/rules/custom.evil", 更新)
	解码(t, 响应, http.StatusOK, &保存)
	if 保存.U规则.U正则 != "worseword" || 保存.U规则.UID != "custom.evil" {
		t.Fatalf("update response=%+v", 保存)
	}

	// 默认规则拒绝删除，只能停用。
	响应 = 发送JSON(t, 客户端, http.MethodDelete, 服务.URL+"/api/rules/xss.script", nil)
	if 响应.StatusCode != http.StatusBadRequest {
		t.Fatalf("default delete status=%d, want 400", 响应.StatusCode)
	}
	默认体 := 读取体(t, 响应)
	_ = 响应.Body.Close()
	if !strings.Contains(默认体, "cannot be deleted") {
		t.Fatalf("default delete body=%s", 默认体)
	}

	响应 = 发送JSON(t, 客户端, http.MethodDelete, 服务.URL+"/api/rules/custom.evil", nil)
	if 响应.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/rules/versions", nil)
	体 := 读取体(t, 响应)
	if 响应.StatusCode != http.StatusOK || !strings.Contains(体, `"versions"`) {
		t.Fatalf("versions status=%d body=%s", 响应.StatusCode, 体)
	}

	响应 = 发送JSON(t, 客户端, http.MethodPost,
		fmt.Sprintf("%s/api/rules/versions/%d/rollback", 服务.URL, 第一版本), nil)
	var 回滚 struct {
		U版本 int64      `json:"version"`
		U规则 []存储包.U规则行 `json:"rules"`
	}
	解码(t, 响应, http.StatusOK, &回滚)
	if 回滚.U版本 <= 第一版本 {
		t.Fatalf("rollback version=%d, original=%d", 回滚.U版本, 第一版本)
	}
	找到 := false
	for _, 行 := range 回滚.U规则 {
		if 行.UID == "custom.evil" {
			找到 = true
		}
	}
	if !找到 {
		t.Fatal("rollback did not restore the custom rule")
	}

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/status", nil)
	var 状态 U状态响应
	解码(t, 响应, http.StatusOK, &状态)
	if 状态.U规则集版本 != 回滚.U版本 {
		t.Fatalf("engine version=%d, rollback version=%d", 状态.U规则集版本, 回滚.U版本)
	}
}

func Test访问列表限流与站点API(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 登录客户端(t, 服务.URL)

	响应 := 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/access-lists", nil)
	var 列表 struct {
		U仅允许 bool       `json:"allow_only"`
		U条目  []存储包.U规则行 `json:"entries"`
	}
	_ = 列表
	体 := 读取体(t, 响应)
	if 响应.StatusCode != http.StatusOK || !strings.Contains(体, `"allow_only"`) {
		t.Fatalf("access list status=%d body=%s", 响应.StatusCode, 体)
	}

	条目 := map[string]any{"id": "office", "action": "allow", "address": "203.0.113.0/24", "enabled": true}
	响应 = 发送JSON(t, 客户端, http.MethodPost, 服务.URL+"/api/access-lists", 条目)
	if 响应.StatusCode != http.StatusCreated {
		t.Fatalf("create entry status=%d body=%s", 响应.StatusCode, 读取体(t, 响应))
	}
	_ = 响应.Body.Close()

	响应 = 发送JSON(t, 客户端, http.MethodPost, 服务.URL+"/api/access-lists",
		map[string]any{"id": "bad", "action": "allow", "address": "not-an-address", "enabled": true})
	if 响应.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid entry status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()

	响应 = 发送JSON(t, 客户端, http.MethodDelete, 服务.URL+"/api/access-lists/office", nil)
	if 响应.StatusCode != http.StatusNoContent {
		t.Fatalf("delete entry status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/rate-limit", nil)
	var 限流 map[string]any
	解码(t, 响应, http.StatusOK, &限流)
	if _, 存在 := 限流["requests_per_minute"]; !存在 {
		t.Fatalf("rate limit body=%+v", 限流)
	}

	响应 = 发送JSON(t, 客户端, http.MethodPut, 服务.URL+"/api/rate-limit",
		map[string]any{"enabled": true, "requests_per_minute": 60, "burst": 5})
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("rate put status=%d body=%s", 响应.StatusCode, 读取体(t, 响应))
	}
	_ = 响应.Body.Close()

	响应 = 发送JSON(t, 客户端, http.MethodPut, 服务.URL+"/api/rate-limit",
		map[string]any{"enabled": true, "requests_per_minute": 0, "burst": 0})
	if 响应.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid rate status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/sites", nil)
	体 = 读取体(t, 响应)
	if 响应.StatusCode != http.StatusOK || !strings.Contains(体, `"upstream"`) || !strings.Contains(体, `"mode":"enforce"`) {
		t.Fatalf("sites status=%d body=%s", 响应.StatusCode, 体)
	}

	响应 = 发送JSON(t, 客户端, http.MethodPut, 服务.URL+"/api/sites/app.example.com/mode",
		map[string]any{"mode": "shadow"})
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("site mode status=%d body=%s", 响应.StatusCode, 读取体(t, 响应))
	}
	_ = 响应.Body.Close()

	攻击 := 新代理请求(t, "http://app.example.com/search?q=1%20union%20select%201")
	记录 := 记录代理响应(管理器, 攻击)
	if 记录.Code != http.StatusBadGateway {
		t.Fatalf("shadow mode did not forward: status=%d", 记录.Code)
	}

	响应 = 发送JSON(t, 客户端, http.MethodPut, 服务.URL+"/api/sites/app.example.com/mode",
		map[string]any{"mode": "invalid"})
	if 响应.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid mode status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
}

func Test统计与日志API(t *testing.T) {
	服务, 存储, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	客户端 := 登录客户端(t, 服务.URL)

	现在 := time.Now().UTC().UnixMilli()
	记录 := []存储包.U记录{
		构造日志(现在-3000, "/api/products/123456", "203.0.113.1", "allow", ""),
		构造日志(现在-2000, "/api/products/654321", "203.0.113.2", "block", "sqli.union-select"),
		构造日志(现在-1000, "/search", "203.0.113.3", "shadow", "xss.script"),
		构造日志(现在-500, "/search", "203.0.113.1", "ratelimited", "ratelimit"),
	}
	if 错误 := 存储.U直接写入(记录); 错误 != nil {
		t.Fatalf("insert logs: %v", 错误)
	}

	响应 := 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/stats/overview?range=1h", nil)
	var 概览 U概览响应
	解码(t, 响应, http.StatusOK, &概览)
	if 概览.UKPI["total_requests"].U值 < 4 {
		t.Fatalf("total requests=%v", 概览.UKPI["total_requests"].U值)
	}
	if len(概览.U序列) == 0 {
		t.Fatal("empty series")
	}
	if len(概览.U表格.U顶级IP) == 0 || len(概览.U表格.U最新拦截) == 0 {
		t.Fatalf("tables empty: %+v", 概览.U表格)
	}
	if !strings.Contains(strings.Join(概览.U范围.U备注, ","), "histogram") {
		t.Fatalf("latency note missing: %+v", 概览.U范围)
	}

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/stats/overview?range=7d", nil)
	解码(t, 响应, http.StatusOK, &概览)
	if !概览.U范围.U近似 {
		t.Fatal("7d range was not marked approximate")
	}

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/stats/overview?range=2d", nil)
	if 响应.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid range status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/logs?limit=2", nil)
	var 第一页 U日志列表响应
	解码(t, 响应, http.StatusOK, &第一页)
	if len(第一页.U条目) != 2 || 第一页.U下一游标 == "" {
		t.Fatalf("first page=%+v", 第一页)
	}
	响应 = 发送JSON(t, 客户端, http.MethodGet,
		服务.URL+"/api/logs?limit=2&cursor="+第一页.U下一游标, nil)
	var 第二页 U日志列表响应
	解码(t, 响应, http.StatusOK, &第二页)
	if len(第二页.U条目) != 2 {
		t.Fatalf("second page items=%d", len(第二页.U条目))
	}

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/logs?action=block", nil)
	var 拦截页 U日志列表响应
	解码(t, 响应, http.StatusOK, &拦截页)
	if len(拦截页.U条目) != 1 || 拦截页.U条目[0].U动作 != "block" {
		t.Fatalf("blocked filter=%+v", 拦截页)
	}

	响应 = 发送JSON(t, 客户端, http.MethodGet,
		fmt.Sprintf("%s/api/logs/%d", 服务.URL, 拦截页.U条目[0].UID), nil)
	var 详情 存储包.U日志条目
	解码(t, 响应, http.StatusOK, &详情)
	if 详情.U请求体 == "" || 详情.U请求头 == "" {
		t.Fatalf("detail missing stored copies: %+v", 详情)
	}

	响应 = 发送JSON(t, 客户端, http.MethodGet, 服务.URL+"/api/logs?cursor=not-a-cursor", nil)
	if 响应.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad cursor status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
}

func Test管理端点未认证(t *testing.T) {
	服务, _, 管理器, _ := 新建测试服务(t, false)
	defer 管理器.Close()
	defer 服务.Close()
	for _, 路径 := range []string{"/api/rules", "/api/logs", "/api/stats/overview?range=1h", "/api/access-lists", "/api/rate-limit", "/api/sites"} {
		响应, 错误 := http.Get(服务.URL + 路径)
		if 错误 != nil {
			t.Fatalf("%s: %v", 路径, 错误)
		}
		_ = 响应.Body.Close()
		if 响应.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s status=%d", 路径, 响应.StatusCode)
		}
	}
}

func 登录客户端(t *testing.T, 基准 string) *http.Client {
	t.Helper()
	客户端 := 新建客户端(t)
	响应 := 发送登录(t, 客户端, 基准, "admin", 测试密码, "")
	if 响应.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d", 响应.StatusCode)
	}
	_ = 响应.Body.Close()
	return 客户端
}

func 发送JSON(t *testing.T, 客户端 *http.Client, 方法, 地址 string, 负载 any) *http.Response {
	t.Helper()
	var 体 io.Reader
	if 负载 != nil {
		数据, 错误 := json.Marshal(负载)
		if 错误 != nil {
			t.Fatalf("marshal: %v", 错误)
		}
		体 = bytes.NewReader(数据)
	}
	请求, 错误 := http.NewRequest(方法, 地址, 体)
	if 错误 != nil {
		t.Fatalf("request: %v", 错误)
	}
	if 负载 != nil {
		请求.Header.Set("Content-Type", "application/json")
	}
	响应, 错误 := 客户端.Do(请求)
	if 错误 != nil {
		t.Fatalf("send: %v", 错误)
	}
	return 响应
}

func 解码(t *testing.T, 响应 *http.Response, 期望状态 int, 目标 any) {
	t.Helper()
	defer func() { _ = 响应.Body.Close() }()
	数据, 错误 := io.ReadAll(响应.Body)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if 响应.StatusCode != 期望状态 {
		t.Fatalf("status=%d want=%d body=%s", 响应.StatusCode, 期望状态, 数据)
	}
	if 目标 == nil {
		return
	}
	if 错误 := json.Unmarshal(数据, 目标); 错误 != nil {
		t.Fatalf("decode: %v body=%s", 错误, 数据)
	}
}

func 新代理请求(t *testing.T, 地址 string) *http.Request {
	t.Helper()
	请求 := httptest.NewRequest(http.MethodGet, 地址, nil)
	return 请求
}

func 记录代理响应(管理器 *U管理器, 请求 *http.Request) *httptest.ResponseRecorder {
	记录 := httptest.NewRecorder()
	管理器.代理.ServeHTTP(记录, 请求)
	return 记录
}

func 构造日志(时间 int64, 路径, 地址, 动作, 规则 string) 存储包.U记录 {
	类别 := ""
	if 规则 != "" {
		类别 = "sqli"
	}
	return 存储包.U记录{
		U请求ID: fmt.Sprintf("req-%d", 时间), U时间: 时间, USite: "app.example.com",
		U方法: "GET", U路径: 路径, U查询: "q=payload", U状态: 200, U动作: 动作,
		U类别: 类别, U规则ID: 规则, U客户端IP: 地址, U用户代理: "curl/8",
		U耗时毫秒: 3, U请求头: `{"User-Agent":["curl/8"]}`, U请求体: `q=payload`,
	}
}

func Test管理响应JSON往返(t *testing.T) {
	变化值 := 12.5
	概览 := U概览响应{
		U范围: U范围信息{U从: "2026-11-06T00:00:00Z", U到: "2026-11-06T01:00:00Z", U桶: "1m", U近似: false, U备注: []string{"note"}},
		UKPI: map[string]UKPI值{
			"total_requests": {U值: 10, U上一值: 8, U变化百分比: &变化值, U序列: []float64{1, 2}},
		},
		U序列: []U时间点{{U时间: 1, U请求: 10, U拦截: 2, U记录: 1, U限流: 0}},
		U分布: U分布{
			U类别: []U计数项{{U键: "sqli", U数量: 2}},
			U规则: []U计数项{{U键: "sqli.union-select", U数量: 2}},
			U动作: []U计数项{{U键: "block", U数量: 2}},
			U站点: []U计数项{{U键: "app.example.com", U数量: 10}},
		},
		U表格: U表格{
			U顶级IP: []UIP行{{U地址: "203.0.113.1", U请求: 5, U拦截: 1}},
			U顶级路径: []U路径行{{U路径: "/search", U请求: 4, U拦截: 1}},
			U最新拦截: []存储包.U日志条目{{UID: 1, U动作: "block", U类别: "sqli"}},
		},
	}
	数据, 错误 := json.Marshal(概览)
	if 错误 != nil {
		t.Fatalf("marshal overview: %v", 错误)
	}
	var 还原 U概览响应
	if 错误 := json.Unmarshal(数据, &还原); 错误 != nil {
		t.Fatalf("unmarshal overview: %v", 错误)
	}
	if !reflect.DeepEqual(概览, 还原) {
		t.Fatalf("overview round trip mismatch")
	}

	站点 := U站点视图{
		U主机: "app.example.com", U上游: "http://127.0.0.1:3000", U模式: "shadow",
		U健康检查路径: "/", U最大请求体: 1 << 20, U健康状态: "up", U延迟毫秒: 3,
	}
	数据, 错误 = json.Marshal(站点)
	if 错误 != nil {
		t.Fatalf("marshal site: %v", 错误)
	}
	var 站点还原 U站点视图
	if 错误 := json.Unmarshal(数据, &站点还原); 错误 != nil {
		t.Fatalf("unmarshal site: %v", 错误)
	}
	if !reflect.DeepEqual(站点, 站点还原) {
		t.Fatalf("site round trip mismatch")
	}
	if !strings.Contains(string(数据), `"max_body_bytes"`) {
		t.Fatalf("site JSON keys: %s", 数据)
	}

	规则 := U规则输入{UID: "x", U名称: "n", U类别: "sqli", U正则: "p", U启用: true, UScore: 3}
	数据, _ = json.Marshal(规则)
	var 规则还原 U规则输入
	if 错误 := json.Unmarshal(数据, &规则还原); 错误 != nil || !reflect.DeepEqual(规则, 规则还原) {
		t.Fatalf("rule round trip: %v %+v", 错误, 规则还原)
	}
}
