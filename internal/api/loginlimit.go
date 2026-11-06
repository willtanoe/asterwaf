package 管理接口

import (
	"strings"
	"sync"
	"time"
)

const (
	登录窗口  = 15 * time.Minute
	每IP上限 = 5
	每用户上限 = 10
)

type 尝试记录 struct {
	次数   int
	窗口开始 time.Time
}

// U登录限制 用固定窗口限制失败尝试，成功登录会同时清除 IP 与用户名计数。
type U登录限制 struct {
	互斥   sync.Mutex
	记录   map[string]尝试记录
	时钟   func() time.Time
	停止   chan struct{}
	完成   chan struct{}
	关闭一次 sync.Once
}

func U新建登录限制() *U登录限制 {
	return 新建登录限制(time.Now, 5*time.Minute)
}

func 新建登录限制(时钟 func() time.Time, 清理间隔 time.Duration) *U登录限制 {
	限制 := &U登录限制{
		记录: make(map[string]尝试记录),
		时钟: 时钟,
		停止: make(chan struct{}),
		完成: make(chan struct{}),
	}
	if 清理间隔 <= 0 {
		close(限制.完成)
		return 限制
	}
	go 限制.清理循环(清理间隔)
	return 限制
}

func (限制 *U登录限制) U检查(地址, 用户名 string) (bool, time.Duration) {
	现在 := 限制.时钟()
	限制.互斥.Lock()
	defer 限制.互斥.Unlock()
	等待 := time.Duration(0)
	for _, 键 := range []string{ip键(地址), 用户键(用户名)} {
		记录, 存在 := 限制.记录[键]
		if !存在 {
			continue
		}
		if 现在.Sub(记录.窗口开始) >= 登录窗口 {
			delete(限制.记录, 键)
			continue
		}
		上限 := 每IP上限
		if strings.HasPrefix(键, "user:") {
			上限 = 每用户上限
		}
		if 记录.次数 >= 上限 {
			剩余 := 记录.窗口开始.Add(登录窗口).Sub(现在)
			if 剩余 > 等待 {
				等待 = 剩余
			}
		}
	}
	if 等待 <= 0 {
		return true, 0
	}
	if 等待 < time.Second {
		等待 = time.Second
	}
	return false, 等待
}

func (限制 *U登录限制) U记录失败(地址, 用户名 string) {
	现在 := 限制.时钟()
	限制.互斥.Lock()
	defer 限制.互斥.Unlock()
	限制.增加(ip键(地址), 现在)
	限制.增加(用户键(用户名), 现在)
}

func (限制 *U登录限制) U清除(地址, 用户名 string) {
	限制.互斥.Lock()
	delete(限制.记录, ip键(地址))
	delete(限制.记录, 用户键(用户名))
	限制.互斥.Unlock()
}

func (限制 *U登录限制) Close() {
	限制.关闭一次.Do(func() {
		close(限制.停止)
	})
	<-限制.完成
}

func (限制 *U登录限制) 增加(键 string, 现在 time.Time) {
	记录, 存在 := 限制.记录[键]
	if !存在 || 现在.Sub(记录.窗口开始) >= 登录窗口 {
		记录 = 尝试记录{窗口开始: 现在}
	}
	记录.次数++
	限制.记录[键] = 记录
}

func (限制 *U登录限制) 清理循环(间隔 time.Duration) {
	defer close(限制.完成)
	计时器 := time.NewTicker(间隔)
	defer 计时器.Stop()
	for {
		select {
		case <-计时器.C:
			限制.清理(限制.时钟())
		case <-限制.停止:
			return
		}
	}
}

func (限制 *U登录限制) 清理(现在 time.Time) {
	限制.互斥.Lock()
	defer 限制.互斥.Unlock()
	for 键, 记录 := range 限制.记录 {
		if 现在.Sub(记录.窗口开始) >= 登录窗口 {
			delete(限制.记录, 键)
		}
	}
}

func ip键(地址 string) string {
	return "ip:" + 地址
}

func 用户键(用户名 string) string {
	return "user:" + strings.ToLower(strings.TrimSpace(用户名))
}
