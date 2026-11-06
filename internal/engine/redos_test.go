package 引擎

import (
	"strings"
	"testing"
)

func Test超长模式与名称被拒绝(t *testing.T) {
	测试用例 := []U规则{
		{UID: "x", U名称: "n", U类别: "sqli", U正则: strings.Repeat("a", 1025)},
		{UID: strings.Repeat("a", 65), U名称: "n", U类别: "sqli", U正则: "a"},
		{UID: "x", U名称: strings.Repeat("n", 121), U类别: "sqli", U正则: "a"},
	}
	for _, 规则 := range 测试用例 {
		if _, 错误 := U新建([]U规则{规则}); 错误 == nil {
			t.Fatalf("rule %q with oversized field was accepted", 规则.UID)
		}
	}
}
