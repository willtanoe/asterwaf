package 引擎

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	检查管线 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/pipeline"
)

const (
	U默认规则版本 int = 1

	U动作允许 = "allow"
	U动作拦截 = "block"
	U动作记录 = "shadow"

	最大模式长度 = 1024
	最大名称长度 = 120
	最大ID长度 = 64
)

var 合法类别 = map[string]struct{}{
	"sqli":      {},
	"xss":       {},
	"traversal": {},
	"rce":       {},
	"scanner":   {},
}

var 合法ID = regexp.MustCompile(`^[a-z0-9._-]+$`)

// U规则 是可持久化定义。Score 为保留字段，当前不参与决策。
type U规则 struct {
	UID    string `json:"id"`
	U名称    string `json:"name"`
	U类别    string `json:"category"`
	U正则    string `json:"pattern"`
	U启用    bool   `json:"enabled"`
	UScore int    `json:"score"`
}

// U决策 是规则引擎对一次请求的唯一结论。
type U决策 struct {
	U动作    string   `json:"action"`
	U模式    string   `json:"mode"`
	U规则ID  string   `json:"rule_id,omitempty"`
	U规则名称  string   `json:"rule_name,omitempty"`
	U类别    string   `json:"category,omitempty"`
	U目标    检查管线.U目标 `json:"target"`
	UScore int      `json:"score,omitempty"`
}

type 编译规则 struct {
	定义 U规则
	正则 *regexp.Regexp
}

// U规则集 不可变；替换时整体换指针，读者不会看到半更新状态。
type U规则集 struct {
	U版本 int64
	规则  []编译规则
}

// U引擎 持有当前规则集。
type U引擎 struct {
	当前   atomic.Pointer[U规则集]
	版本计数 atomic.Int64
}

// U新建 编译规则并安装首个规则集。
func U新建(规则 []U规则) (*U引擎, error) {
	引擎 := &U引擎{}
	if 错误 := 引擎.U替换(规则); 错误 != nil {
		return nil, 错误
	}
	return 引擎, nil
}

// U替换 只在全部规则编译成功后发布新规则集。
func (引擎 *U引擎) U替换(规则 []U规则) error {
	编译后, 错误 := 编译(规则)
	if 错误 != nil {
		return 错误
	}
	版本 := 引擎.版本计数.Add(1)
	引擎.当前.Store(&U规则集{U版本: 版本, 规则: 编译后})
	return nil
}

// U安装 用显式版本替换规则集，使内存版本与数据库版本一致。
func (引擎 *U引擎) U安装(规则 []U规则, 版本 int64) error {
	编译后, 错误 := 编译(规则)
	if 错误 != nil {
		return 错误
	}
	引擎.版本计数.Store(版本)
	引擎.当前.Store(&U规则集{U版本: 版本, 规则: 编译后})
	return nil
}

// U当前版本 返回规则集代号，API 用它判断是否需要提示刷新。
func (引擎 *U引擎) U当前版本() int64 {
	规则集 := 引擎.当前.Load()
	if 规则集 == nil {
		return 0
	}
	return 规则集.U版本
}

// U规则数量 返回当前规则集内的规则总数，包括停用规则。
func (引擎 *U引擎) U规则数量() int {
	规则集 := 引擎.当前.Load()
	if 规则集 == nil {
		return 0
	}
	return len(规则集.规则)
}

// U决策 按模式返回允许、记录或拦截。扫描顺序即优先级，首个命中胜出。
func (引擎 *U引擎) U决策(模式 string, 目标列表 []检查管线.U目标) U决策 {
	决策 := U决策{U动作: U动作允许, U模式: 模式}
	if 模式 != "shadow" && 模式 != "enforce" {
		return 决策
	}
	规则集 := 引擎.当前.Load()
	if 规则集 == nil {
		return 决策
	}
	for _, 编译项 := range 规则集.规则 {
		if !编译项.定义.U启用 {
			continue
		}
		for _, 目标 := range 目标列表 {
			if !编译项.正则.MatchString(目标.U值) {
				continue
			}
			决策.U规则ID = 编译项.定义.UID
			决策.U规则名称 = 编译项.定义.U名称
			决策.U类别 = 编译项.定义.U类别
			决策.U目标 = 目标
			决策.UScore = 编译项.定义.UScore
			if 模式 == "enforce" {
				决策.U动作 = U动作拦截
			} else {
				决策.U动作 = U动作记录
			}
			return 决策
		}
	}
	return 决策
}

// U匹配 是一次规则测试中的单条命中。U匹配片段 来自被测试的输入本身，
// 不经过脱敏：调用方只应把它展示给提交输入的人。
type U匹配 struct {
	U规则ID string `json:"rule_id"`
	U类别   string `json:"category"`
	U名称    string `json:"name"`
	U匹配片段 string `json:"match"`
}

// U匹配列表 返回当前规则集中所有命中目标值的启用规则，供规则测试器展示
// 完整命中清单；与 U决策 的首个命中语义不同。
func (引擎 *U引擎) U匹配列表(目标列表 []检查管线.U目标) []U匹配 {
	规则集 := 引擎.当前.Load()
	if 规则集 == nil {
		return nil
	}
	结果 := make([]U匹配, 0)
	for _, 编译项 := range 规则集.规则 {
		if !编译项.定义.U启用 {
			continue
		}
		for _, 目标 := range 目标列表 {
			片段 := 编译项.正则.FindString(目标.U值)
			if 片段 == "" {
				continue
			}
			结果 = append(结果, U匹配{
				U规则ID: 编译项.定义.UID,
				U类别:   编译项.定义.U类别,
				U名称:   编译项.定义.U名称,
				U匹配片段: 片段,
			})
			break
		}
	}
	return 结果
}

func 编译(规则 []U规则) ([]编译规则, error) {
	已见 := make(map[string]struct{}, len(规则))
	结果 := make([]编译规则, 0, len(规则))
	for 序号, 定义 := range 规则 {
		if 错误 := 校验(定义); 错误 != nil {
			return nil, fmt.Errorf("rule %d: %w", 序号, 错误)
		}
		if _, 重复 := 已见[定义.UID]; 重复 {
			return nil, fmt.Errorf("rule %q: duplicate id", 定义.UID)
		}
		已见[定义.UID] = struct{}{}
		编译项, 错误 := regexp.Compile(定义.U正则)
		if 错误 != nil {
			return nil, fmt.Errorf("rule %q: invalid pattern: %w", 定义.UID, 错误)
		}
		结果 = append(结果, 编译规则{定义: 定义, 正则: 编译项})
	}
	return 结果, nil
}

func 校验(定义 U规则) error {
	if 定义.UID == "" {
		return errors.New("id must not be empty")
	}
	if len(定义.UID) > 最大ID长度 {
		return fmt.Errorf("id exceeds %d bytes", 最大ID长度)
	}
	if !合法ID.MatchString(定义.UID) {
		return errors.New("id must match [a-z0-9._-]+")
	}
	if strings.TrimSpace(定义.U名称) == "" {
		return errors.New("name must not be empty")
	}
	if len(定义.U名称) > 最大名称长度 {
		return fmt.Errorf("name exceeds %d bytes", 最大名称长度)
	}
	if _, 合法 := 合法类别[定义.U类别]; !合法 {
		return fmt.Errorf("category %q is not supported", 定义.U类别)
	}
	if 定义.U正则 == "" {
		return errors.New("pattern must not be empty")
	}
	if len(定义.U正则) > 最大模式长度 {
		return fmt.Errorf("pattern exceeds %d bytes", 最大模式长度)
	}
	return nil
}
