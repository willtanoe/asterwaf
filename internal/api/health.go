package 管理接口

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
)

const (
	探测间隔   = 5 * time.Second
	探测超时   = 2 * time.Second
	失败判定次数 = 3
)

// U站点健康 是状态接口返回的单项，JSON 键保持英文。
type U站点健康 struct {
	U主机   string `json:"host"`
	U状态   string `json:"status"`
	U延迟毫秒 int64  `json:"latency_ms"`
	U失败次数 int    `json:"failures"`
	U最后检查 int64  `json:"last_check"`
}

type 探测状态 struct {
	状态   string
	延迟毫秒 int64
	失败次数 int
	最后检查 int64
}

// 探测样本 是一次探测的历史记录；每个站点只保留最近 60 条。
type 探测样本 struct {
	U时刻   int64  `json:"ts"`
	U状态   string `json:"status"`
	U延迟毫秒 int64  `json:"latency_ms"`
}

const 探测历史上限 = 60

// U探测器 周期性访问每个站点的 health_path，连续三次失败才标记 down。
type U探测器 struct {
	站点   []配置.U站点
	互斥   sync.Mutex
	状态表  map[string]探测状态
	历史表  map[string][]探测样本
	客户端  *http.Client
	日志   *slog.Logger
	停止   chan struct{}
	完成   chan struct{}
	关闭一次 sync.Once
}

func U新建探测器(站点列表 []配置.U站点, 间隔 time.Duration, 日志 *slog.Logger) *U探测器 {
	if 日志 == nil {
		日志 = slog.Default()
	}
	探测器 := &U探测器{
		站点:  append([]配置.U站点(nil), 站点列表...),
		状态表: make(map[string]探测状态, len(站点列表)),
		历史表: make(map[string][]探测样本, len(站点列表)),
		客户端: &http.Client{
			Timeout: 探测超时,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		日志: 日志,
		停止: make(chan struct{}),
		完成: make(chan struct{}),
	}
	for _, 站点值 := range 站点列表 {
		探测器.状态表[站点值.U主机] = 探测状态{状态: "unknown"}
	}
	if 间隔 <= 0 {
		close(探测器.完成)
		return 探测器
	}
	go 探测器.循环(间隔)
	return 探测器
}

func (探测器 *U探测器) U状态() []U站点健康 {
	探测器.互斥.Lock()
	defer 探测器.互斥.Unlock()
	结果 := make([]U站点健康, 0, len(探测器.站点))
	for _, 站点值 := range 探测器.站点 {
		状态 := 探测器.状态表[站点值.U主机]
		结果 = append(结果, U站点健康{
			U主机:   站点值.U主机,
			U状态:   状态.状态,
			U延迟毫秒: 状态.延迟毫秒,
			U失败次数: 状态.失败次数,
			U最后检查: 状态.最后检查,
		})
	}
	return 结果
}

// U探测一次 供后台循环与测试共用。
func (探测器 *U探测器) U探测一次() {
	for _, 站点值 := range 探测器.站点 {
		开始 := time.Now()
		健康 := false
		目标 := 健康地址(站点值)
		if 目标 == "" {
			探测器.记录(站点值.U主机, 健康, time.Since(开始).Milliseconds())
			continue
		}
		请求, 错误 := http.NewRequest(http.MethodGet, 目标, nil)
		if 错误 != nil {
			探测器.记录(站点值.U主机, false, time.Since(开始).Milliseconds())
			continue
		}
		请求.Host = 站点值.U主机
		响应, 错误 := 探测器.客户端.Do(请求)
		if 错误 == nil {
			健康 = 响应.StatusCode < 500
			_, _ = io.Copy(io.Discard, 响应.Body)
			_ = 响应.Body.Close()
		}
		探测器.记录(站点值.U主机, 健康, time.Since(开始).Milliseconds())
	}
}

// U历史 返回站点最近探测样本（旧->新）。
func (探测器 *U探测器) U历史(主机 string) []探测样本 {
	探测器.互斥.Lock()
	defer 探测器.互斥.Unlock()
	样本 := 探测器.历史表[主机]
	结果 := make([]探测样本, len(样本))
	copy(结果, 样本)
	return 结果
}

func (探测器 *U探测器) 记录(主机 string, 健康 bool, 延迟 int64) {
	探测器.互斥.Lock()
	defer 探测器.互斥.Unlock()
	状态 := 探测器.状态表[主机]
	状态.延迟毫秒 = 延迟
	状态.最后检查 = time.Now().UnixMilli()
	if 健康 {
		状态.状态 = "up"
		状态.失败次数 = 0
	} else {
		状态.失败次数++
		if 状态.失败次数 >= 失败判定次数 {
			状态.状态 = "down"
		}
	}
	样本 := 探测样本{U时刻: 状态.最后检查, U延迟毫秒: 延迟, U状态: 状态.状态}
	历史 := append(探测器.历史表[主机], 样本)
	if len(历史) > 探测历史上限 {
		历史 = 历史[len(历史)-探测历史上限:]
	}
	探测器.历史表[主机] = 历史
	探测器.状态表[主机] = 状态
}

func (探测器 *U探测器) 循环(间隔 time.Duration) {
	defer close(探测器.完成)
	探测器.U探测一次()
	计时器 := time.NewTicker(间隔)
	defer 计时器.Stop()
	for {
		select {
		case <-计时器.C:
			探测器.U探测一次()
		case <-探测器.停止:
			return
		}
	}
}

func (探测器 *U探测器) Close() {
	探测器.关闭一次.Do(func() {
		close(探测器.停止)
	})
	<-探测器.完成
}

func 健康地址(站点值 配置.U站点) string {
	上游, 错误 := url.Parse(站点值.U上游)
	if 错误 != nil || 上游.Host == "" {
		return ""
	}
	路径 := 站点值.U健康检查路径
	if 路径 == "" || 路径 == "/" {
		路径 = "/"
	}
	基路径 := strings.TrimRight(上游.Path, "/")
	子路径 := "/" + strings.TrimLeft(路径, "/")
	上游.Path = 基路径 + 子路径
	上游.RawQuery = ""
	上游.Fragment = ""
	return 上游.String()
}
