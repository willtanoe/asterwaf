package 引擎

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	检查管线 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/pipeline"
)

func Test默认规则行为(t *testing.T) {
	样本 := 默认规则样本()
	for _, 规则 := range U默认规则() {
		样本项 := 样本[规则.UID]
		引擎, 错误 := U新建([]U规则{规则})
		if 错误 != nil {
			t.Fatalf("rule %s: compile: %v", 规则.UID, 错误)
		}
		for _, 载荷 := range 样本项.恶意 {
			决策 := 引擎.U决策("enforce", 目标(载荷))
			if 决策.U动作 != U动作拦截 {
				t.Fatalf("rule %s missed malicious sample %q", 规则.UID, 载荷)
			}
			if 决策.U规则ID != 规则.UID {
				t.Fatalf("rule %s matched %s", 规则.UID, 决策.U规则ID)
			}
		}
		for _, 载荷 := range 样本项.正常 {
			决策 := 引擎.U决策("enforce", 目标(载荷))
			if 决策.U动作 != U动作允许 {
				t.Fatalf("rule %s false positive on %q (target %+v)", 规则.UID, 载荷, 决策.U目标)
			}
		}
	}
}

func Test模式决策(t *testing.T) {
	规则 := U规则{UID: "test.mode", U名称: "mode test", U类别: "sqli", U正则: "attack", U启用: true, UScore: 9}
	引擎, 错误 := U新建([]U规则{规则})
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}

	允许 := 引擎.U决策("off", 目标("attack"))
	if 允许.U动作 != U动作允许 || 允许.U规则ID != "" {
		t.Fatalf("off mode decided %+v", 允许)
	}

	记录 := 引擎.U决策("shadow", 目标("attack"))
	if 记录.U动作 != U动作记录 || 记录.U规则ID != "test.mode" || 记录.U模式 != "shadow" {
		t.Fatalf("shadow mode decided %+v", 记录)
	}

	拦截 := 引擎.U决策("enforce", 目标("attack"))
	if 拦截.U动作 != U动作拦截 || 拦截.U规则ID != "test.mode" || 拦截.UScore != 9 {
		t.Fatalf("enforce mode decided %+v", 拦截)
	}

	未命中 := 引擎.U决策("enforce", 目标("benign"))
	if 未命中.U动作 != U动作允许 {
		t.Fatalf("benign decided %+v", 未命中)
	}
}

func Test首个命中规则胜出(t *testing.T) {
	引擎, 错误 := U新建([]U规则{
		{UID: "a.first", U名称: "first", U类别: "sqli", U正则: "shared", U启用: true},
		{UID: "b.second", U名称: "second", U类别: "xss", U正则: "shared", U启用: true},
	})
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	决策 := 引擎.U决策("enforce", 目标("shared value"))
	if 决策.U规则ID != "a.first" {
		t.Fatalf("first match was %s", 决策.U规则ID)
	}
}

func Test原子替换(t *testing.T) {
	引擎, 错误 := U新建([]U规则{{UID: "old.rule", U名称: "old", U类别: "sqli", U正则: "old", U启用: true}})
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	if 版本 := 引擎.U当前版本(); 版本 != 1 {
		t.Fatalf("version=%d", 版本)
	}
	if 决策 := 引擎.U决策("enforce", 目标("old")); 决策.U动作 != U动作拦截 {
		t.Fatalf("old rule inactive: %+v", 决策)
	}

	if 错误 := 引擎.U替换([]U规则{{UID: "new.rule", U名称: "new", U类别: "xss", U正则: "new", U启用: true}}); 错误 != nil {
		t.Fatalf("replace: %v", 错误)
	}
	if 版本 := 引擎.U当前版本(); 版本 != 2 {
		t.Fatalf("version=%d", 版本)
	}
	if 决策 := 引擎.U决策("enforce", 目标("old")); 决策.U动作 != U动作允许 {
		t.Fatalf("old rule still active: %+v", 决策)
	}
	if 决策 := 引擎.U决策("enforce", 目标("new")); 决策.U动作 != U动作拦截 {
		t.Fatalf("new rule inactive: %+v", 决策)
	}

	if 错误 := 引擎.U替换([]U规则{{UID: "bad.rule", U名称: "bad", U类别: "sqli", U正则: "(", U启用: true}}); 错误 == nil {
		t.Fatal("expected invalid replacement to fail")
	}
	if 版本 := 引擎.U当前版本(); 版本 != 2 {
		t.Fatalf("failed replacement changed version to %d", 版本)
	}
	if 决策 := 引擎.U决策("enforce", 目标("new")); 决策.U动作 != U动作拦截 {
		t.Fatalf("failed replacement changed active rules: %+v", 决策)
	}
}

func Test并发读者看到一致规则集(t *testing.T) {
	引擎, 错误 := U新建([]U规则{{UID: "one.rule", U名称: "one", U类别: "sqli", U正则: "one", U启用: true}})
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	var 组 sync.WaitGroup
	组.Add(4)
	for 序号 := 0; 序号 < 4; 序号++ {
		组.Add(0)
		go func() {
			defer 组.Done()
			for 次数 := 0; 次数 < 100; 次数++ {
				决策 := 引擎.U决策("enforce", 目标("one two"))
				if 决策.U规则ID != "one.rule" && 决策.U规则ID != "two.rule" {
					t.Errorf("unexpected rule id %q", 决策.U规则ID)
					return
				}
			}
		}()
	}
	for 次数 := 0; 次数 < 20; 次数++ {
		规则 := U规则{UID: "one.rule", U名称: "one", U类别: "sqli", U正则: "one", U启用: true}
		if 次数%2 == 1 {
			规则 = U规则{UID: "two.rule", U名称: "two", U类别: "xss", U正则: "two", U启用: true}
		}
		if 错误 := 引擎.U替换([]U规则{规则}); 错误 != nil {
			t.Fatalf("replace: %v", 错误)
		}
	}
	组.Wait()
}

func Test规则校验(t *testing.T) {
	基础 := U规则{UID: "ok.rule", U名称: "ok", U类别: "sqli", U正则: "ok", U启用: true}
	测试用例 := map[string][]U规则{
		"empty id":       {{UID: "", U名称: "n", U类别: "sqli", U正则: "x"}},
		"long id":        {{UID: strings.Repeat("a", 65), U名称: "n", U类别: "sqli", U正则: "x"}},
		"bad id charset": {{UID: "Bad ID", U名称: "n", U类别: "sqli", U正则: "x"}},
		"empty name":     {{UID: "r", U名称: "  ", U类别: "sqli", U正则: "x"}},
		"long name":      {{UID: "r", U名称: strings.Repeat("n", 121), U类别: "sqli", U正则: "x"}},
		"bad category":   {{UID: "r", U名称: "n", U类别: "custom", U正则: "x"}},
		"empty pattern":  {{UID: "r", U名称: "n", U类别: "sqli", U正则: ""}},
		"long pattern":   {{UID: "r", U名称: "n", U类别: "sqli", U正则: strings.Repeat("a", 1025)}},
		"bad regex":      {{UID: "r", U名称: "n", U类别: "sqli", U正则: "("}},
		"duplicate":      {基础, 基础},
	}
	for 名称, 规则列表 := range 测试用例 {
		t.Run(名称, func(t *testing.T) {
			if _, 错误 := U新建(规则列表); 错误 == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if _, 错误 := U新建([]U规则{基础}); 错误 != nil {
		t.Fatalf("valid rule rejected: %v", 错误)
	}
	if _, 错误 := U新建(U默认规则()); 错误 != nil {
		t.Fatalf("default rules rejected: %v", 错误)
	}
}

func Test规则与决策JSON往返(t *testing.T) {
	规则 := U规则{
		UID: "sqli.test", U名称: "test", U类别: "sqli", U正则: `union\s+select`,
		U启用: true, UScore: 42,
	}
	数据, 错误 := json.Marshal(规则)
	if 错误 != nil {
		t.Fatalf("marshal rule: %v", 错误)
	}
	for _, 键 := range []string{`"id"`, `"name"`, `"category"`, `"pattern"`, `"enabled"`, `"score"`} {
		if !strings.Contains(string(数据), 键) {
			t.Fatalf("rule JSON missing %s: %s", 键, 数据)
		}
	}
	var 还原 U规则
	if 错误 := json.Unmarshal(数据, &还原); 错误 != nil {
		t.Fatalf("unmarshal rule: %v", 错误)
	}
	if !reflect.DeepEqual(规则, 还原) {
		t.Fatalf("rule round trip mismatch: %+v", 还原)
	}

	决策 := U决策{
		U动作: U动作拦截, U模式: "enforce", U规则ID: "sqli.test", U规则名称: "test",
		U类别: "sqli", U目标: 检查管线.U目标{U类别: "query", U名称: "q", U值: "1 union select"},
		UScore: 42,
	}
	数据, 错误 = json.Marshal(决策)
	if 错误 != nil {
		t.Fatalf("marshal decision: %v", 错误)
	}
	var 决策还原 U决策
	if 错误 := json.Unmarshal(数据, &决策还原); 错误 != nil {
		t.Fatalf("unmarshal decision: %v", 错误)
	}
	if !reflect.DeepEqual(决策, 决策还原) {
		t.Fatalf("decision round trip mismatch: %+v", 决策还原)
	}
}

func 目标(值 string) []检查管线.U目标 {
	return []检查管线.U目标{{U类别: "query", U名称: "q", U值: 值}}
}
