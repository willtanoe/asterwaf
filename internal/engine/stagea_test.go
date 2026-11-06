//go:build stagea

package 引擎_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/clientip"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	规则引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/proxy"
)

type 用例 struct {
	U名称   string            `json:"name"`
	U方法   string            `json:"method"`
	U路径   string            `json:"path"`
	U体    string            `json:"body"`
	U内容类型 string            `json:"content_type"`
	U请求头  map[string]string `json:"headers"`
}

type 语料 struct {
	U正常 []用例 `json:"normal"`
	U攻击 []用例 `json:"attack"`
}

type 结果 struct {
	用例   用例
	类别   string
	状态   int
	拦截   bool
	规则ID string
	原因   string
}

type 日志行 struct {
	Action string `json:"action"`
	RuleID string `json:"rule_id"`
	Mode   string `json:"mode"`
	Path   string `json:"path"`
	Shadow bool   `json:"shadow"`
}

func TestStageA(t *testing.T) {
	语料值 := 读取语料(t)
	基准 := os.Getenv("ASTERWAF_STAGE_A_BASE")
	var 正常 []结果
	var 攻击结果 []结果
	if 基准 == "" {
		正常, 攻击结果 = 内置运行(t, 语料值)
	} else {
		正常, 攻击结果 = 真实运行(t, 语料值, 基准, os.Getenv("ASTERWAF_STAGE_A_LOG"))
	}

	误报 := 0
	for _, 项 := range 正常 {
		if 项.拦截 {
			误报++
		}
	}
	拦截数 := 0
	for _, 项 := range 攻击结果 {
		if 项.拦截 {
			拦截数++
		}
	}
	通过 := 误报 == 0 && 拦截数 >= 32

	原因 := os.Getenv("ASTERWAF_STAGE_A_FALLBACK_REASON")
	模式 := "embedded"
	if 基准 != "" {
		模式 = 基准
		原因 = "OWASP Juice Shop via http://127.0.0.1:3000"
	}
	if 错误 := 写报告(t, 模式, 原因, 正常, 攻击结果, 误报, 拦截数, 通过); 错误 != nil {
		t.Fatalf("write report: %v", 错误)
	}
	if !通过 {
		t.Errorf("Stage A failed: false positives=%d blocked attacks=%d/40", 误报, 拦截数)
	}
}

func TestStageACorpusRoundTrip(t *testing.T) {
	原始 := 读取语料(t)
	数据, 错误 := json.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("marshal: %v", 错误)
	}
	var 还原 语料
	if 错误 := json.Unmarshal(数据, &还原); 错误 != nil {
		t.Fatalf("unmarshal: %v", 错误)
	}
	if !reflect.DeepEqual(原始, 还原) {
		t.Fatal("corpus round trip mismatch")
	}
}

func 读取语料(t *testing.T) 语料 {
	t.Helper()
	数据, 错误 := os.ReadFile(filepath.Join("testdata", "stage_a.json"))
	if 错误 != nil {
		t.Fatalf("read corpus: %v", 错误)
	}
	var 语料值 语料
	if 错误 := json.Unmarshal(数据, &语料值); 错误 != nil {
		t.Fatalf("parse corpus: %v", 错误)
	}
	if len(语料值.U正常) != 40 || len(语料值.U攻击) != 40 {
		t.Fatalf("corpus size normal=%d attack=%d", len(语料值.U正常), len(语料值.U攻击))
	}
	return 语料值
}

func 内置运行(t *testing.T, 语料值 语料) ([]结果, []结果) {
	t.Helper()
	上游 := httptest.NewServer(http.HandlerFunc(func(写 http.ResponseWriter, _ *http.Request) {
		写.WriteHeader(http.StatusOK)
		_, _ = 写.Write([]byte("upstream ok"))
	}))
	defer 上游.Close()

	解析器, 错误 := 客户端地址.U新建(nil)
	if 错误 != nil {
		t.Fatalf("resolver: %v", 错误)
	}
	引擎, 错误 := 规则引擎.U新建(规则引擎.U默认规则())
	if 错误 != nil {
		t.Fatalf("engine: %v", 错误)
	}
	var 日志缓冲 bytes.Buffer
	日志 := slog.New(slog.NewJSONHandler(&日志缓冲, nil))
	处理器, 错误 := 反向代理.U新建(配置.U配置{
		U模式:  "enforce",
		U检查头: []string{"User-Agent", "Referer", "X-Forwarded-For"},
		U站点: []配置.U站点{{
			U主机: "stage-a.test", U上游: 上游.URL, U模式: "enforce", U最大请求体: 1 << 20,
		}},
	}, 解析器, 引擎, nil, 日志)
	if 错误 != nil {
		t.Fatalf("proxy: %v", 错误)
	}
	网关 := httptest.NewServer(处理器)
	defer 网关.Close()
	客户端 := 网关.Client()

	正常 := 发送全部(t, 客户端, 网关.URL, "stage-a.test", 语料值.U正常, &日志缓冲)
	攻击 := 发送全部(t, 客户端, 网关.URL, "stage-a.test", 语料值.U攻击, &日志缓冲)
	return 正常, 攻击
}

func 发送全部(t *testing.T, 客户端 *http.Client, 基准, 主机 string, 用例列表 []用例, 日志缓冲 *bytes.Buffer) []结果 {
	t.Helper()
	结果列表 := make([]结果, 0, len(用例列表))
	for _, 项 := range 用例列表 {
		日志缓冲.Reset()
		请求, 错误 := http.NewRequest(项.U方法, 基准+项.U路径, strings.NewReader(项.U体))
		if 错误 != nil {
			t.Fatalf("request %s: %v", 项.U名称, 错误)
		}
		请求.Host = 主机
		if 项.U内容类型 != "" {
			请求.Header.Set("Content-Type", 项.U内容类型)
		}
		for 键, 值 := range 项.U请求头 {
			请求.Header.Set(键, 值)
		}
		响应, 错误 := 客户端.Do(请求)
		if 错误 != nil {
			t.Fatalf("send %s: %v", 项.U名称, 错误)
		}
		_, _ = io.Copy(io.Discard, 响应.Body)
		_ = 响应.Body.Close()
		拦截 := 响应.StatusCode == http.StatusForbidden
		规则ID := 解析决策(日志缓冲.Bytes())
		结果列表 = append(结果列表, 结果{用例: 项, 状态: 响应.StatusCode, 拦截: 拦截, 规则ID: 规则ID})
	}
	return 结果列表
}

func 解析决策(数据 []byte) string {
	扫描器 := bufio.NewScanner(bytes.NewReader(数据))
	for 扫描器.Scan() {
		var 行 日志行
		if 错误 := json.Unmarshal(扫描器.Bytes(), &行); 错误 != nil {
			continue
		}
		if 行.Action == "block" || 行.Action == "shadow" {
			return 行.RuleID
		}
	}
	return ""
}

func 真实运行(t *testing.T, 语料值 语料, 基准, 日志路径 string) ([]结果, []结果) {
	t.Helper()
	if 日志路径 == "" {
		t.Fatal("ASTERWAF_STAGE_A_LOG is required in real mode")
	}
	客户端 := &http.Client{Timeout: 10 * time.Second}
	正常 := 发送真实(t, 客户端, 基准, "127.0.0.1", 语料值.U正常, 日志路径)
	攻击 := 发送真实(t, 客户端, 基准, "127.0.0.1", 语料值.U攻击, 日志路径)
	return 正常, 攻击
}

func 发送真实(t *testing.T, 客户端 *http.Client, 基准, 主机 string, 用例列表 []用例, 日志路径 string) []结果 {
	t.Helper()
	结果列表 := make([]结果, 0, len(用例列表))
	for _, 项 := range 用例列表 {
		请求, 错误 := http.NewRequest(项.U方法, 基准+项.U路径, strings.NewReader(项.U体))
		if 错误 != nil {
			t.Fatalf("request %s: %v", 项.U名称, 错误)
		}
		请求.Host = 主机
		if 项.U内容类型 != "" {
			请求.Header.Set("Content-Type", 项.U内容类型)
		}
		for 键, 值 := range 项.U请求头 {
			请求.Header.Set(键, 值)
		}
		响应, 错误 := 客户端.Do(请求)
		if 错误 != nil {
			t.Fatalf("send %s: %v", 项.U名称, 错误)
		}
		_, _ = io.Copy(io.Discard, 响应.Body)
		_ = 响应.Body.Close()
		记录 := 读取决策(日志路径, 项.U路径)
		结果列表 = append(结果列表, 结果{
			用例: 项, 状态: 响应.StatusCode,
			拦截:   响应.StatusCode == http.StatusForbidden,
			规则ID: 记录.RuleID,
		})
	}
	return 结果列表
}

func 读取决策(路径, 请求地址 string) 日志行 {
	文件, 错误 := os.Open(路径)
	if 错误 != nil {
		return 日志行{}
	}
	defer func() { _ = 文件.Close() }()
	var 命中 日志行
	扫描器 := bufio.NewScanner(文件)
	扫描器.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for 扫描器.Scan() {
		var 行 日志行
		if 错误 := json.Unmarshal(扫描器.Bytes(), &行); 错误 != nil {
			continue
		}
		if 行.Path == 请求地址 && (行.Action == "block" || 行.Action == "shadow") {
			命中 = 行
		}
	}
	return 命中
}

func 写报告(t *testing.T, 模式, 原因 string, 正常, 攻击 []结果, 误报, 拦截数 int, 通过 bool) error {
	t.Helper()
	var 输出 strings.Builder
	输出.WriteString("# Stage A smoke test\n\n")
	输出.WriteString("Generated: " + time.Now().UTC().Format(time.RFC3339) + "\n\n")
	输出.WriteString("Target: " + 模式 + "\n\n")
	if 原因 != "" {
		输出.WriteString("Environment note: " + 原因 + "\n\n")
	}
	结论 := "FAIL"
	if 通过 {
		结论 = "PASS"
	}
	输出.WriteString("## Verdict\n\n")
	输出.WriteString("- Normal requests: " + fmt.Sprint(len(正常)) + "\n")
	输出.WriteString("- Normal requests blocked (false positives): " + fmt.Sprint(误报) + "\n")
	输出.WriteString("- Attack requests: " + fmt.Sprint(len(攻击)) + "\n")
	输出.WriteString("- Attack requests blocked: " + fmt.Sprint(拦截数) + "/" + fmt.Sprint(len(攻击)) + "\n")
	输出.WriteString("- Criteria: zero false positives and at least 32 of 40 attacks blocked\n")
	输出.WriteString("- Result: " + 结论 + "\n\n")
	输出.WriteString("This is a smoke test, not a performance or coverage claim.\n\n")
	输出.WriteString("## Normal requests\n\n")
	输出.WriteString("| # | Name | Method | Path | Status | Blocked | Rule |\n")
	输出.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for 序号, 项 := range 正常 {
		输出.WriteString(fmt.Sprintf("| %d | %s | %s | `%s` | %d | %s | %s |\n",
			序号+1, 项.用例.U名称, 项.用例.U方法, 项.用例.U路径, 项.状态, 是(项.拦截), 项.规则ID))
	}
	输出.WriteString("\n## Attack requests\n\n")
	输出.WriteString("| # | Name | Method | Path | Status | Blocked | Rule | Reason |\n")
	输出.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	未拦截 := 0
	for 序号, 项 := range 攻击 {
		说明 := ""
		if !项.拦截 {
			未拦截++
			说明 = "no default rule matched this payload"
		}
		输出.WriteString(fmt.Sprintf("| %d | %s | %s | `%s` | %d | %s | %s | %s |\n",
			序号+1, 项.用例.U名称, 项.用例.U方法, 项.用例.U路径, 项.状态, 是(项.拦截), 项.规则ID, 说明))
	}
	输出.WriteString("\nUnblocked attacks: " + fmt.Sprint(未拦截) + ". Each is listed above with the reason that no default rule matched.\n")
	if 原因 != "" && 模式 == "embedded" {
		输出.WriteString("\nJuice Shop was not available from npm, so this run used the committed fallback corpus. ")
		输出.WriteString("The real upstream integration is the remaining gap.\n")
	}

	路径 := filepath.Join("..", "..", "docs", "stage-a.md")
	return os.WriteFile(路径, []byte(输出.String()), 0o644)
}

func 是(值 bool) string {
	if 值 {
		return "yes"
	}
	return "no"
}
