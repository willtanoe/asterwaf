//go:build calibration

package 校准_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	检查管线 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/pipeline"
)

// 语料只是回归关口：规模小，且与规则同一作者维护，结果偏乐观。

type 用例 struct {
	UID   string            `json:"id"`
	U类别   string            `json:"category"`
	U方法   string            `json:"method"`
	U路径   string            `json:"path"`
	U查询   string            `json:"query"`
	U体    string            `json:"body"`
	U内容类型 string            `json:"content_type"`
	U请求头  map[string]string `json:"headers"`
	U备注   string            `json:"note"`
}

type 语料 struct {
	U正常 []用例 `json:"normal"`
	U攻击 []用例 `json:"attack"`
}

type 单条结果 struct {
	用例   用例
	命中   bool
	规则ID string
}

const (
	最大误报率   = 0.005
	单规则最大误报 = 0.01
	最小检测率   = 0.95
)

func Test校准语料(t *testing.T) {
	语料值 := 读取语料(t)
	引擎, 错误 := 引擎.U新建(引擎.U默认规则())
	if 错误 != nil {
		t.Fatalf("compile defaults: %v", 错误)
	}

	正常结果 := 评估(t, 引擎, 语料值.U正常)
	攻击结果 := 评估(t, 引擎, 语料值.U攻击)

	误报 := 0
	按规则误报 := map[string]int{}
	for _, 项 := range 正常结果 {
		if 项.命中 {
			误报++
			按规则误报[项.规则ID]++
		}
	}
	检测 := 0
	漏报 := make([]单条结果, 0)
	按类别 := map[string][2]int{}
	for _, 项 := range 攻击结果 {
		计数 := 按类别[项.用例.U类别]
		计数[1]++
		if 项.命中 {
			检测++
			计数[0]++
			按类别[项.用例.U类别] = 计数
			continue
		}
		按类别[项.用例.U类别] = 计数
		漏报 = append(漏报, 项)
	}

	误报率 := float64(误报) / float64(len(正常结果))
	检测率 := float64(检测) / float64(len(攻击结果))
	规则总数 := 引擎.U规则数量()

	写报告(t, 正常结果, 攻击结果, 误报, 检测, 误报率, 检测率, 按类别, 漏报, 按规则误报, 规则总数)

	if 误报率 > 最大误报率 {
		t.Errorf("false positive rate %.4f exceeds %.4f (%d of %d)", 误报率, 最大误报率, 误报, len(正常结果))
	}
	for 规则ID, 数量 := range 按规则误报 {
		if float64(数量) > 单规则最大误报*float64(len(正常结果)) {
			t.Errorf("rule %s produced %d false positives, above the per-rule threshold", 规则ID, 数量)
		}
	}
	if 检测率 < 最小检测率 {
		t.Errorf("detection rate %.4f is below %.2f (%d of %d)", 检测率, 最小检测率, 检测, len(攻击结果))
	}
}

func Test校准语料可解析(t *testing.T) {
	语料值 := 读取语料(t)
	if len(语料值.U正常) != 300 {
		t.Fatalf("normal cases=%d", len(语料值.U正常))
	}
	if len(语料值.U攻击) != 150 {
		t.Fatalf("attack cases=%d", len(语料值.U攻击))
	}
	见ID := map[string]struct{}{}
	for _, 项 := range append(append([]用例{}, 语料值.U正常...), 语料值.U攻击...) {
		if 项.UID == "" || 项.U方法 == "" || 项.U路径 == "" {
			t.Fatalf("incomplete case: %+v", 项)
		}
		if _, 重复 := 见ID[项.UID]; 重复 {
			t.Fatalf("duplicate case id %s", 项.UID)
		}
		见ID[项.UID] = struct{}{}
		if 项.U查询 != "" {
			if _, 错误 := url.ParseQuery(项.U查询); 错误 != nil {
				// 一部分攻击载荷本身就不是合法 query，仍必须能被管线读取。
				t.Logf("case %s has a deliberately malformed query: %v", 项.UID, 错误)
			}
		}
	}
}

func 评估(t *testing.T, 规则引擎 *引擎.U引擎, 用例列表 []用例) []单条结果 {
	t.Helper()
	管线 := 检查管线.U新建([]string{"User-Agent", "Referer", "X-Forwarded-For"})
	结果 := make([]单条结果, 0, len(用例列表))
	for _, 项 := range 用例列表 {
		地址 := 项.U路径
		if 项.U查询 != "" {
			地址 += "?" + 项.U查询
		}
		请求 := httptest.NewRequest(项.U方法, "http://calibration.test"+地址, strings.NewReader(项.U体))
		if 项.U内容类型 != "" {
			请求.Header.Set("Content-Type", 项.U内容类型)
		}
		for 键, 值 := range 项.U请求头 {
			请求.Header.Set(键, 值)
		}
		快照, 错误 := 管线.U读取(httptest.NewRecorder(), 请求, 1<<20)
		if 错误 != nil {
			t.Fatalf("case %s: read request: %v", 项.UID, 错误)
		}
		决策 := 规则引擎.U决策("enforce", 快照.U目标列表())
		结果 = append(结果, 单条结果{
			用例:   项,
			命中:   决策.U动作 == 引擎.U动作拦截,
			规则ID: 决策.U规则ID,
		})
	}
	return 结果
}

func 读取语料(t *testing.T) 语料 {
	t.Helper()
	数据, 错误 := os.ReadFile(filepath.Join("testdata", "corpus.json"))
	if 错误 != nil {
		t.Fatalf("read corpus: %v", 错误)
	}
	var 语料值 语料
	if 错误 := json.Unmarshal(数据, &语料值); 错误 != nil {
		t.Fatalf("parse corpus: %v", 错误)
	}
	return 语料值
}

func 写报告(
	t *testing.T,
	正常结果 []单条结果,
	攻击结果 []单条结果,
	误报, 检测 int,
	误报率, 检测率 float64,
	按类别 map[string][2]int,
	漏报 []单条结果,
	按规则误报 map[string]int,
	规则总数 int,
) {
	t.Helper()
	var 输出 strings.Builder
	输出.WriteString("# Calibration\n\n")
	输出.WriteString("This corpus is a regression gate, not a measurement of production accuracy. It has 300 normal and 150 labelled attack requests, is small, and was written by the same person who wrote the rules, so the numbers below are optimistic.\n\n")
	输出.WriteString("## Thresholds\n\n")
	输出.WriteString("| Metric | Threshold | Result |\n| --- | --- | --- |\n")
	输出.WriteString(fmt.Sprintf("| False positive rate | <= %.1f%% | %.2f%% (%d of %d) |\n", 最大误报率*100, 误报率*100, 误报, len(正常结果)))
	输出.WriteString(fmt.Sprintf("| Detection rate | >= %.0f%% | %.2f%% (%d of %d) |\n", 最小检测率*100, 检测率*100, 检测, len(攻击结果)))
	输出.WriteString(fmt.Sprintf("| Rules evaluated | n/a | %d |\n", 规则总数))
	输出.WriteString("\n## Detection by category\n\n| Category | Blocked | Total | Rate |\n| --- | --- | --- | --- |\n")
	类别键 := make([]string, 0, len(按类别))
	for 键 := range 按类别 {
		类别键 = append(类别键, 键)
	}
	sort.Strings(类别键)
	for _, 键 := range 类别键 {
		值 := 按类别[键]
		率 := 0.0
		if 值[1] > 0 {
			率 = float64(值[0]) / float64(值[1]) * 100
		}
		输出.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", 键, 值[0], 值[1], 率))
	}

	输出.WriteString("\n## Missed attacks\n\n")
	if len(漏报) == 0 {
		输出.WriteString("None.\n")
	} else {
		输出.WriteString("| ID | Category | Request | Note |\n| --- | --- | --- | --- |\n")
		for _, 项 := range 漏报 {
			地址 := 项.用例.U路径
			if 项.用例.U查询 != "" {
				地址 += "?" + 项.用例.U查询
			}
			输出.WriteString(fmt.Sprintf("| %s | %s | `%s %s` | %s |\n", 项.用例.UID, 项.用例.U类别, 项.用例.U方法, 地址, 项.用例.U备注))
		}
	}

	输出.WriteString("\nMiss reasons:\n\n")
	输出.WriteString("- `a072` sends `;` inside the query value. Go's `url.ParseQuery` rejects the semicolon form, so the pipeline records `invalid query encoding` and has no decoded query targets to test. `traversal.dotdot` would match the value if it were parsed; the limitation is in query parsing, not in the pattern.\n")
	输出.WriteString("- `a082` uses overlong UTF-8 (`%c0%af`). Decoding produces bytes that are not a valid `/`, so the decoded target contains no encoded separator and the pattern deliberately does not guess at illegal UTF-8. A backend that accepts overlong sequences is out of scope for these rules.\n")
	输出.WriteString("- `a103` is only a file name (`name=shell.php`). Blocking every `.php` name would flag legitimate uploads, so no rule targets it; the request is recorded and the upstream must validate uploads.\n")

	输出.WriteString("\n## False positives\n\n")
	有误报 := false
	for _, 项 := range 正常结果 {
		if !项.命中 {
			continue
		}
		有误报 = true
	}
	if !有误报 {
		输出.WriteString("None.\n")
	} else {
		输出.WriteString("| ID | Method | Request | Rule |\n| --- | --- | --- | --- |\n")
		for _, 项 := range 正常结果 {
			if !项.命中 {
				continue
			}
			地址 := 项.用例.U路径
			if 项.用例.U查询 != "" {
				地址 += "?" + 项.用例.U查询
			}
			输出.WriteString(fmt.Sprintf("| %s | %s | `%s` | %s |\n", 项.用例.UID, 项.用例.U方法, 地址, 项.规则ID))
		}
	}

	输出.WriteString("\n## Per-rule false positives\n\n")
	if len(按规则误报) == 0 {
		输出.WriteString("No rule produced a false positive.\n")
	} else {
		规则 := make([]string, 0, len(按规则误报))
		for 键 := range 按规则误报 {
			规则 = append(规则, 键)
		}
		sort.Strings(规则)
		输出.WriteString("| Rule | False positives |\n| --- | --- |\n")
		for _, 键 := range 规则 {
			输出.WriteString(fmt.Sprintf("| %s | %d |\n", 键, 按规则误报[键]))
		}
	}

	输出.WriteString("\n## Before pointing it at a real site\n\n")
	输出.WriteString("1. Run asterwaf in `shadow` mode for several days. Nothing is blocked; every rule decision is recorded.\n")
	输出.WriteString("2. Review the logs page filtered by action `Recorded`, grouped by rule and path. Investigate each rule that fires on legitimate traffic.\n")
	输出.WriteString("3. Adjust or disable those rules through the UI, then roll back if the change makes things worse. Each change creates a new ruleset version.\n")
	输出.WriteString("4. Move a single site to `enforce` only after its shadow decisions have been reviewed for a full business cycle, and keep the access list ready for fast exceptions.\n")

	路径 := filepath.Join("..", "..", "docs", "calibration.md")
	if 错误 := os.WriteFile(路径, []byte(输出.String()), 0o644); 错误 != nil {
		t.Fatalf("write report: %v", 错误)
	}
}
