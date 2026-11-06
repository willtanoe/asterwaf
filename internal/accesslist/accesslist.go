package 访问列表

import (
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"sync/atomic"
)

var 合法标识 = regexp.MustCompile(`^[a-z0-9._-]+$`)

// U条目 是一条允许或拒绝规则。地址可以是单个 IP 或 CIDR。
type U条目 struct {
	UID     string `yaml:"id" json:"id"`
	U动作     string `yaml:"action" json:"action"`
	U地址     string `yaml:"address" json:"address"`
	U备注     string `yaml:"note,omitempty" json:"note,omitempty"`
	U启用     bool   `yaml:"enabled" json:"enabled"`
	U来源日志ID int64  `yaml:"source_log_id,omitempty" json:"source_log_id,omitempty"`
}

// U列表 是完整的访问列表配置。allow_only 打开时只放行明确允许的地址。
type U列表 struct {
	U仅允许 bool  `yaml:"allow_only" json:"allow_only"`
	U条目  []U条目 `yaml:"entries" json:"entries"`
}

// U结果 描述访问列表结论。U白名单 为真时后续阶段跳过限流。
type U结果 struct {
	U拦截   bool   `json:"blocked"`
	U白名单  bool   `json:"allowlisted"`
	U规则ID string `json:"rule_id,omitempty"`
}

type 编译条目 struct {
	允许 bool
	前缀 netip.Prefix
}

type 状态 struct {
	仅允许 bool
	条目  []编译条目
}

// U检查器 持有不可变编译结果，替换时整体换指针。
type U检查器 struct {
	当前 atomic.Pointer[状态]
}

// U新建 编译访问列表；错误会让启动失败，而不是悄悄放行。
func U新建(列表 U列表) (*U检查器, error) {
	检查器 := &U检查器{}
	if 错误 := 检查器.U替换(列表); 错误 != nil {
		return nil, 错误
	}
	return 检查器, nil
}

// U替换 校验并发布新列表，拒绝半有效状态。
func (检查器 *U检查器) U替换(列表 U列表) error {
	状态值, 错误 := 编译(列表)
	if 错误 != nil {
		return 错误
	}
	检查器.当前.Store(状态值)
	return nil
}

// U检查 先看 deny，再判断 allow 与 allow_only；deny 永远优先。
func (检查器 *U检查器) U检查(地址 netip.Addr) U结果 {
	状态值 := 检查器.当前.Load()
	if 状态值 == nil {
		return U结果{}
	}
	地址 = 地址.Unmap()
	白名单 := false
	for _, 条目 := range 状态值.条目 {
		if !条目.前缀.Contains(地址) {
			continue
		}
		if !条目.允许 {
			return U结果{U拦截: true, U规则ID: "acl:deny"}
		}
		白名单 = true
	}
	if 白名单 {
		return U结果{U白名单: true, U规则ID: "acl:allow"}
	}
	if 状态值.仅允许 {
		return U结果{U拦截: true, U规则ID: "acl:allow-only"}
	}
	return U结果{}
}

func 编译(列表 U列表) (*状态, error) {
	已见 := make(map[string]struct{}, len(列表.U条目))
	结果 := &状态{仅允许: 列表.U仅允许, 条目: make([]编译条目, 0, len(列表.U条目))}
	for 序号, 条目 := range 列表.U条目 {
		if 条目.UID == "" {
			return nil, fmt.Errorf("access list entry %d: id must not be empty", 序号)
		}
		if len(条目.UID) > 64 || !合法标识.MatchString(条目.UID) {
			return nil, fmt.Errorf("access list entry %d: id must match [a-z0-9._-]{1,64}", 序号)
		}
		if _, 重复 := 已见[条目.UID]; 重复 {
			return nil, fmt.Errorf("access list entry %q: duplicate id", 条目.UID)
		}
		已见[条目.UID] = struct{}{}
		if 条目.U动作 != "allow" && 条目.U动作 != "deny" {
			return nil, fmt.Errorf("access list entry %q: action must be allow or deny", 条目.UID)
		}
		if len(条目.U备注) > 200 {
			return nil, fmt.Errorf("access list entry %q: note exceeds 200 bytes", 条目.UID)
		}
		前缀, 错误 := 解析地址(条目.U地址)
		if 错误 != nil {
			return nil, fmt.Errorf("access list entry %q: %w", 条目.UID, 错误)
		}
		if !条目.U启用 {
			continue
		}
		结果.条目 = append(结果.条目, 编译条目{允许: 条目.U动作 == "allow", 前缀: 前缀})
	}
	return 结果, nil
}

func 解析地址(原文 string) (netip.Prefix, error) {
	值 := strings.TrimSpace(原文)
	if 值 == "" {
		return netip.Prefix{}, errors.New("address must not be empty")
	}
	if 前缀, 错误 := netip.ParsePrefix(值); 错误 == nil {
		return 前缀.Masked(), nil
	}
	地址, 错误 := netip.ParseAddr(值)
	if 错误 != nil {
		return netip.Prefix{}, fmt.Errorf("invalid address %q", 值)
	}
	地址 = 地址.Unmap()
	return netip.PrefixFrom(地址, 地址.BitLen()), nil
}
