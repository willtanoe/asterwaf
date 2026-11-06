package 限流

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sync"
	"time"
)

const (
	U默认每分钟请求 = 600
	U默认突发    = 100
	空闲淘汰时长   = 10 * time.Minute
)

// 测试需要缩小上限来验证淘汰路径，生产默认十万。
var 最大条目数 = 100000

// U配置 是可在运行时修改的限流参数。
type U配置 struct {
	U启用  bool `yaml:"enabled" json:"enabled"`
	U每分钟 int  `yaml:"requests_per_minute" json:"requests_per_minute"`
	U突发  int  `yaml:"burst" json:"burst"`
}

type 桶 struct {
	令牌   float64
	上次补充 time.Time
	最后访问 time.Time
	键    netip.Prefix
}

// U限流器 用互斥锁保护令牌桶表，条目按最近访问顺序排列以便淘汰最旧。
type U限流器 struct {
	配置 U配置
	时钟 func() time.Time
	互斥 sync.Mutex
	桶表 map[netip.Prefix]*list.Element
	顺序 *list.List
	停止 chan struct{}
	完成 chan struct{}
	关闭 sync.Once
}

// U新建 启动后台清理，生产入口不需要传入时钟。
func U新建(配置值 U配置) (*U限流器, error) {
	return 新建(配置值, time.Now, time.Minute)
}

func 新建(配置值 U配置, 时钟 func() time.Time, 清理间隔 time.Duration) (*U限流器, error) {
	if !配置值.U启用 {
		if 配置值.U每分钟 <= 0 {
			配置值.U每分钟 = U默认每分钟请求
		}
		if 配置值.U突发 <= 0 {
			配置值.U突发 = U默认突发
		}
	}
	if 配置值.U每分钟 <= 0 {
		return nil, errors.New("rate limit: requests_per_minute must be positive")
	}
	if 配置值.U突发 <= 0 {
		return nil, errors.New("rate limit: burst must be positive")
	}
	限流器 := &U限流器{
		配置: 配置值,
		时钟: 时钟,
		桶表: make(map[netip.Prefix]*list.Element),
		顺序: list.New(),
		停止: make(chan struct{}),
		完成: make(chan struct{}),
	}
	if 清理间隔 <= 0 {
		close(限流器.完成)
		return 限流器, nil
	}
	go 限流器.清理循环(清理间隔)
	return 限流器, nil
}

// U启用 报告是否需要调用 U允许。
func (限流器 *U限流器) U启用() bool {
	限流器.互斥.Lock()
	defer 限流器.互斥.Unlock()
	return 限流器.配置.U启用
}

// U允许 消耗一个令牌；拒绝时返回建议的 Retry-After。
func (限流器 *U限流器) U允许(地址 netip.Addr) (bool, time.Duration) {
	键, 有效 := 桶键(地址)
	if !有效 {
		return true, 0
	}
	现在 := 限流器.时钟()

	限流器.互斥.Lock()
	defer 限流器.互斥.Unlock()

	// 配置可在运行时替换，必须与写入方共用同一把锁。
	配置值 := 限流器.配置
	if !配置值.U启用 {
		return true, 0
	}
	速率 := float64(配置值.U每分钟) / 60.0
	if 速率 <= 0 {
		return true, 0
	}

	元素, 存在 := 限流器.桶表[键]
	if !存在 {
		if len(限流器.桶表) >= 最大条目数 {
			限流器.淘汰最旧()
		}
		桶值 := &桶{
			令牌:   float64(配置值.U突发),
			上次补充: 现在,
			最后访问: 现在,
			键:    键,
		}
		元素 = 限流器.顺序.PushBack(桶值)
		限流器.桶表[键] = 元素
	} else {
		限流器.顺序.MoveToBack(元素)
	}

	桶值 := 元素.Value.(*桶)
	经过 := 现在.Sub(桶值.上次补充).Seconds()
	if 经过 > 0 {
		桶值.令牌 = math.Min(float64(配置值.U突发), 桶值.令牌+经过*速率)
		桶值.上次补充 = 现在
	}
	桶值.最后访问 = 现在
	if 桶值.令牌 >= 1 {
		桶值.令牌--
		return true, 0
	}
	缺少 := 1 - 桶值.令牌
	等待 := time.Duration(math.Ceil(缺少/速率)) * time.Second
	if 等待 < time.Second {
		等待 = time.Second
	}
	return false, 等待
}

// U修改配置 原子替换参数；令牌桶保留，避免修改配置瞬间清空惩罚。
func (限流器 *U限流器) U修改配置(配置值 U配置) error {
	if 配置值.U每分钟 <= 0 || 配置值.U突发 <= 0 {
		return fmt.Errorf("rate limit: requests_per_minute and burst must be positive")
	}
	限流器.互斥.Lock()
	defer 限流器.互斥.Unlock()
	限流器.配置 = 配置值
	return nil
}

// Close 停止清理协程，可重复调用。
func (限流器 *U限流器) Close() error {
	限流器.关闭.Do(func() {
		close(限流器.停止)
	})
	<-限流器.完成
	return nil
}

func (限流器 *U限流器) 清理循环(间隔 time.Duration) {
	defer close(限流器.完成)
	计时器 := time.NewTicker(间隔)
	defer 计时器.Stop()
	for {
		select {
		case <-计时器.C:
			限流器.清理(限流器.时钟())
		case <-限流器.停止:
			return
		}
	}
}

func (限流器 *U限流器) 清理(现在 time.Time) {
	限流器.互斥.Lock()
	defer 限流器.互斥.Unlock()
	for 元素 := 限流器.顺序.Front(); 元素 != nil; 元素 = 限流器.顺序.Front() {
		桶值 := 元素.Value.(*桶)
		if 现在.Sub(桶值.最后访问) <= 空闲淘汰时长 {
			return
		}
		限流器.顺序.Remove(元素)
		delete(限流器.桶表, 桶值.键)
	}
}

func (限流器 *U限流器) 淘汰最旧() {
	元素 := 限流器.顺序.Front()
	if 元素 == nil {
		return
	}
	桶值 := 元素.Value.(*桶)
	限流器.顺序.Remove(元素)
	delete(限流器.桶表, 桶值.键)
}

func 桶键(地址 netip.Addr) (netip.Prefix, bool) {
	if !地址.IsValid() {
		return netip.Prefix{}, false
	}
	地址 = 地址.Unmap()
	if 地址.Is4() {
		return netip.PrefixFrom(地址, 32), true
	}
	// IPv6 按 /64 聚合，避免攻击者在同一前缀内轮换地址绕过限流。
	return netip.PrefixFrom(地址, 64).Masked(), true
}
