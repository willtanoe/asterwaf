//go:build !race

package 引擎

import (
	"strings"
	"testing"
	"time"

	检查管线 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/pipeline"
)

var 长输入载荷 = []string{
	strings.Repeat("a", 最大测试长度),
	strings.Repeat("union ", 最大测试长度/6),
	strings.Repeat("<svg ", 最大测试长度/5),
	strings.Repeat(".. /", 最大测试长度/4),
	strings.Repeat("on", 最大测试长度/2) + "=",
	strings.Repeat("%2e%2e%2f", 最大测试长度/9),
	strings.Repeat("\x00", 最大测试长度),
	strings.Repeat("1 OR 1=1 ", 最大测试长度/9),
}

const 最大测试长度 = 64 << 10

// 长且近乎匹配的输入是回溯型引擎的典型陷阱；RE2 应当保持线性时间。
// 竞态检测器会放大正则执行时间，因此这条计时断言只在非 -race 构建中运行。
func Test规则对长输入保持线性(t *testing.T) {
	引擎, 错误 := U新建(U默认规则())
	if 错误 != nil {
		t.Fatalf("compile: %v", 错误)
	}
	开始 := time.Now()
	for _, 值 := range 长输入载荷 {
		_ = 引擎.U决策("enforce", []检查管线.U目标{{U类别: "query", U名称: "q", U值: 值}})
	}
	耗时 := time.Since(开始)
	if 耗时 > 2*time.Second {
		t.Fatalf("rules took %v for %d large inputs; regex may backtrack", 耗时, len(长输入载荷))
	}
	t.Logf("evaluated %d large inputs in %v", len(长输入载荷), 耗时)
}
