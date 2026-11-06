package 访问列表

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func Test访问列表优先级(t *testing.T) {
	基础 := []U条目{
		{UID: "office", U动作: "allow", U地址: "203.0.113.0/24", U启用: true},
		{UID: "v6", U动作: "allow", U地址: "2001:db8::/32", U启用: true},
		{UID: "blocked", U动作: "deny", U地址: "198.51.100.9", U启用: true},
		{UID: "both", U动作: "allow", U地址: "198.51.100.9", U启用: true},
		{UID: "disabled", U动作: "deny", U地址: "192.0.2.1", U启用: false},
	}

	检查器, 错误 := U新建(U列表{U条目: 基础})
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	测试用例 := []struct {
		地址   string
		拦截   bool
		白名单  bool
		规则ID string
	}{
		{"198.51.100.9", true, false, "acl:deny"},
		{"203.0.113.55", false, true, "acl:allow"},
		{"2001:db8::5", false, true, "acl:allow"},
		{"192.0.2.1", false, false, ""},
		{"10.0.0.1", false, false, ""},
	}
	for _, 用例 := range 测试用例 {
		结果 := 检查器.U检查(netip.MustParseAddr(用例.地址))
		if 结果.U拦截 != 用例.拦截 || 结果.U白名单 != 用例.白名单 {
			t.Fatalf("%s: %+v", 用例.地址, 结果)
		}
		if 结果.U规则ID != 用例.规则ID {
			t.Fatalf("%s: rule=%q", 用例.地址, 结果.U规则ID)
		}
	}

	严格, 错误 := U新建(U列表{U仅允许: true, U条目: 基础})
	if 错误 != nil {
		t.Fatalf("new allow_only: %v", 错误)
	}
	if 结果 := 严格.U检查(netip.MustParseAddr("10.0.0.1")); !结果.U拦截 || 结果.U规则ID != "acl:allow-only" {
		t.Fatalf("allow_only did not block: %+v", 结果)
	}
	if 结果 := 严格.U检查(netip.MustParseAddr("203.0.113.55")); 结果.U拦截 || !结果.U白名单 {
		t.Fatalf("allowlisted address was blocked: %+v", 结果)
	}
}

func Test访问列表原子替换(t *testing.T) {
	检查器, 错误 := U新建(U列表{U条目: []U条目{
		{UID: "old", U动作: "deny", U地址: "10.0.0.1", U启用: true},
	}})
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	if 结果 := 检查器.U检查(netip.MustParseAddr("10.0.0.1")); !结果.U拦截 {
		t.Fatalf("old entry inactive: %+v", 结果)
	}

	if 错误 := 检查器.U替换(U列表{U条目: []U条目{
		{UID: "new", U动作: "deny", U地址: "10.0.0.2", U启用: true},
	}}); 错误 != nil {
		t.Fatalf("replace: %v", 错误)
	}
	if 结果 := 检查器.U检查(netip.MustParseAddr("10.0.0.1")); 结果.U拦截 {
		t.Fatalf("old entry still active: %+v", 结果)
	}
	if 结果 := 检查器.U检查(netip.MustParseAddr("10.0.0.2")); !结果.U拦截 {
		t.Fatalf("new entry inactive: %+v", 结果)
	}

	if 错误 := 检查器.U替换(U列表{U条目: []U条目{
		{UID: "bad", U动作: "deny", U地址: "not-an-ip", U启用: true},
	}}); 错误 == nil {
		t.Fatal("expected invalid replacement to fail")
	}
	if 结果 := 检查器.U检查(netip.MustParseAddr("10.0.0.2")); !结果.U拦截 {
		t.Fatalf("failed replacement changed state: %+v", 结果)
	}
}

func Test访问列表校验(t *testing.T) {
	测试用例 := map[string]U列表{
		"empty id":    {U条目: []U条目{{UID: "", U动作: "deny", U地址: "10.0.0.1", U启用: true}}},
		"bad id":      {U条目: []U条目{{UID: "Bad Id", U动作: "deny", U地址: "10.0.0.1", U启用: true}}},
		"bad action":  {U条目: []U条目{{UID: "x", U动作: "block", U地址: "10.0.0.1", U启用: true}}},
		"bad address": {U条目: []U条目{{UID: "x", U动作: "deny", U地址: "10.0.0.1/99", U启用: true}}},
		"long note":   {U条目: []U条目{{UID: "x", U动作: "deny", U地址: "10.0.0.1", U备注: strings.Repeat("n", 201), U启用: true}}},
		"duplicate id": {U条目: []U条目{
			{UID: "x", U动作: "deny", U地址: "10.0.0.1", U启用: true},
			{UID: "x", U动作: "allow", U地址: "10.0.0.2", U启用: true},
		}},
	}
	for 名称, 列表 := range 测试用例 {
		t.Run(名称, func(t *testing.T) {
			if _, 错误 := U新建(列表); 错误 == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func Test访问列表序列化往返(t *testing.T) {
	原始 := U列表{
		U仅允许: true,
		U条目: []U条目{
			{UID: "office", U动作: "allow", U地址: "203.0.113.0/24", U备注: "office", U启用: true, U来源日志ID: 42},
			{UID: "bad", U动作: "deny", U地址: "198.51.100.9", U启用: false},
		},
	}

	YAML, 错误 := yaml.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("yaml marshal: %v", 错误)
	}
	for _, 键 := range []string{"allow_only:", "entries:", "action:", "address:"} {
		if !strings.Contains(string(YAML), 键) {
			t.Fatalf("yaml missing %s: %s", 键, YAML)
		}
	}
	var YAML还原 U列表
	if 错误 := yaml.Unmarshal(YAML, &YAML还原); 错误 != nil {
		t.Fatalf("yaml unmarshal: %v", 错误)
	}
	if !reflect.DeepEqual(原始, YAML还原) {
		t.Fatalf("yaml round trip mismatch: %+v", YAML还原)
	}

	JSON, 错误 := json.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("json marshal: %v", 错误)
	}
	var JSON还原 U列表
	if 错误 := json.Unmarshal(JSON, &JSON还原); 错误 != nil {
		t.Fatalf("json unmarshal: %v", 错误)
	}
	if !reflect.DeepEqual(原始, JSON还原) {
		t.Fatalf("json round trip mismatch: %+v", JSON还原)
	}
}
