package 管理接口

import (
	"context"
	"net/http"
	"strconv"
	"time"

	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

// 本文件承载页面级统计端点：规则页、访问列表页、站点抽屉与日志摘要。
// 原始日志维度只在 24 小时窗口内返回；更长窗口返回 raw_available=false。

type U规则序列点 struct {
	U时间 int64   `json:"ts"`
	U值  []int64 `json:"values"`
}

type U规则统计响应 struct {
	U范围   U范围信息    `json:"range"`
	U规则   []U规则统计项 `json:"rules"`
	U总规则  int      `json:"total_rules"`
	U原始可用 bool     `json:"raw_available"`
}

type U规则统计项 struct {
	U规则ID string    `json:"rule_id"`
	U类别   string    `json:"category"`
	U命中   int64     `json:"hits"`
	U拦截   int64     `json:"blocked"`
	U记录数  int64     `json:"recorded"`
	U最后命中 int64     `json:"last_seen"`
	U序列   []float64 `json:"series"`
}

func (管理器 *U管理器) 规则统计(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	范围, 错误 := 解析概览范围(请求)
	if 错误 != nil {
		写错误(写, http.StatusBadRequest, 错误请求, 错误.Error())
		return
	}
	站点 := 请求.URL.Query().Get("site")
	上下文 := 请求.Context()
	统计, 错误 := 管理器.存储.U规则统计(上下文, 范围.从, 范围.到, 站点, 30)
	if 错误 != nil {
		管理器.日志.Error("rule stats failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rule statistics")
		return
	}
	序列, 错误 := 管理器.存储.U规则序列(上下文, 范围.从, 范围.到, 站点, 范围.桶毫秒)
	if 错误 != nil {
		管理器.日志.Error("rule series failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rule statistics")
		return
	}
	起点 := 范围.从 / 范围.桶毫秒 * 范围.桶毫秒
	桶数 := int((范围.到 - 范围.从) / 范围.桶毫秒)
	索引 := make(map[string][]float64)
	次序 := make([]string, 0, len(统计))
	for _, 行 := range 统计 {
		索引[行.U规则ID] = make([]float64, 桶数)
		次序 = append(次序, 行.U规则ID)
	}
	_ = 次序
	for _, 点 := range 序列 {
		下标 := int((点.U桶 - 起点) / 范围.桶毫秒)
		if 下标 < 0 || 下标 >= 桶数 {
			continue
		}
		if 值, 存在 := 索引[点.U规则ID]; 存在 {
			值[下标] = float64(点.U数量)
		}
	}
	总规则 := 0
	if 全部, 错误 := 管理器.存储.U读取规则(上下文); 错误 == nil {
		总规则 = len(全部)
	}
	结果 := make([]U规则统计项, 0, len(统计))
	for _, 行 := range 统计 {
		结果 = append(结果, U规则统计项{
			U规则ID: 行.U规则ID, U类别: 行.U类别, U命中: 行.U命中,
			U拦截: 行.U拦截, U记录数: 行.U记录数, U最后命中: 行.U最后命中,
			U序列: 索引[行.U规则ID],
		})
	}
	写JSON(写, http.StatusOK, U规则统计响应{
		U范围:   U范围信息{U从: time.UnixMilli(范围.从).UTC().Format(time.RFC3339), U到: time.UnixMilli(范围.到).UTC().Format(time.RFC3339), U桶: 范围.桶, U近似: 范围.近似, U备注: 范围.备注},
		U规则:   结果,
		U总规则:  总规则,
		U原始可用: !范围.近似,
	})
}

type U访问统计响应 struct {
	U范围    U范围信息    `json:"range"`
	U条目    []U访问条目项 `json:"entries"`
	U拒绝请求数 int64    `json:"denied_requests"`
	U限流序列  []U限流序列项 `json:"ratelimited_series"`
	U限流总数  int64    `json:"ratelimited_total"`
	U顶级违规  []UIP行   `json:"top_offenders"`
	U限流已启用 bool     `json:"rate_limit_enabled"`
	U限流桶数  int      `json:"rate_limit_buckets"`
	U允许数   int      `json:"allow_count"`
	U拒绝数   int      `json:"deny_count"`
}

type U访问条目项 struct {
	U规则ID string `json:"rule_id"`
	U命中   int64  `json:"hits"`
	U最后命中 int64  `json:"last_hit"`
}

type U限流序列项 struct {
	U时间 int64 `json:"ts"`
	U数量 int64 `json:"count"`
}

func (管理器 *U管理器) 访问统计(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	范围, 错误 := 解析概览范围(请求)
	if 错误 != nil {
		写错误(写, http.StatusBadRequest, 错误请求, 错误.Error())
		return
	}
	上下文 := 请求.Context()
	条目统计, 错误 := 管理器.存储.U访问条目统计(上下文, 范围.从, 范围.到, 50)
	if 错误 != nil {
		管理器.日志.Error("access entry stats failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access statistics")
		return
	}
	拒绝请求数, 错误 := 管理器.拒绝请求数(上下文, 范围)
	if 错误 != nil {
		管理器.日志.Error("denied requests failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access statistics")
		return
	}
	限流序列, 错误 := 管理器.存储.U限制序列(上下文, 范围.从, 范围.到, 范围.桶毫秒)
	if 错误 != nil {
		管理器.日志.Error("ratelimit series failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access statistics")
		return
	}
	限流项 := make([]U限流序列项, 0, len(限流序列))
	限流总数 := int64(0)
	for _, 点 := range 限流序列 {
		限流项 = append(限流项, U限流序列项{U时间: 点.U桶, U数量: 点.U数量})
		限流总数 += 点.U数量
	}
	违规, 错误 := 管理器.存储.U顶级拦截IP(上下文, 范围.从, 范围.到, 10)
	if 错误 != nil {
		管理器.日志.Error("top offenders failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access statistics")
		return
	}
	违规行 := make([]UIP行, 0, len(违规))
	for _, 项 := range 违规 {
		违规行 = append(违规行, UIP行{U地址: 项.U地址, U请求: 项.U请求})
	}
	列表, 错误 := 管理器.存储.U读取访问列表(上下文)
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access statistics")
		return
	}
	允许数, 拒绝数 := 0, 0
	for _, 条目 := range 列表.U条目 {
		if 条目.U动作 == "allow" {
			允许数++
		} else {
			拒绝数++
		}
	}
	限流配置, 错误 := 管理器.存储.U读取限流(上下文)
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access statistics")
		return
	}
	条目项 := make([]U访问条目项, 0, len(条目统计))
	for _, 行 := range 条目统计 {
		条目项 = append(条目项, U访问条目项{U规则ID: 行.U规则ID, U命中: 行.U命中, U最后命中: 行.U最后命中})
	}
	写JSON(写, http.StatusOK, U访问统计响应{
		U范围:    U范围信息{U从: time.UnixMilli(范围.从).UTC().Format(time.RFC3339), U到: time.UnixMilli(范围.到).UTC().Format(time.RFC3339), U桶: 范围.桶, U近似: 范围.近似, U备注: 范围.备注},
		U条目:    条目项,
		U拒绝请求数: 拒绝请求数,
		U限流序列:  限流项,
		U限流总数:  限流总数,
		U顶级违规:  违规行,
		U限流已启用: 限流配置.U启用,
		U限流桶数:  管理器.代理.U限流桶数(),
		U允许数:   允许数,
		U拒绝数:   拒绝数,
	})
}

func (管理器 *U管理器) 拒绝请求数(上下文 context.Context, 范围 概览范围) (int64, error) {
	return 管理器.存储.U动作计数(上下文, 范围.从, 范围.到, "block")
}

type U站点详情响应 struct {
	U主机   string      `json:"host"`
	U健康历史 []探测样本      `json:"health_history"`
	U模式历史 []U模式变更项    `json:"mode_history"`
	U最近拦截 []存储包.U日志条目 `json:"recent_blocked"`
}

type U模式变更项 struct {
	U时间  int64  `json:"ts"`
	U操作者 string `json:"actor"`
	U前   string `json:"before"`
	U后   string `json:"after"`
}

func (管理器 *U管理器) 站点详情(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	主机 := 请求.URL.Query().Get("host")
	if 主机 == "" {
		写错误(写, http.StatusBadRequest, 错误请求, "host is required")
		return
	}
	上下文 := 请求.Context()
	历史 := 管理器.探测器.U历史(主机)
	变更原始, 错误 := 管理器.存储.U模式历史(上下文, 主机, 20)
	if 错误 != nil {
		管理器.日志.Error("site mode history failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read site detail")
		return
	}
	变更 := make([]U模式变更项, 0, len(变更原始))
	for _, 项 := range 变更原始 {
		变更 = append(变更, U模式变更项{U时间: 项.U时间, U操作者: 项.U操作者, U前: 项.U前, U后: 项.U后})
	}
	拦截, 错误 := 管理器.存储.U最新拦截(上下文, 主机, 10)
	if 错误 != nil {
		管理器.日志.Error("recent blocked failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read site detail")
		return
	}
	写JSON(写, http.StatusOK, U站点详情响应{
		U主机:   主机,
		U健康历史: 历史,
		U模式历史: 变更,
		U最近拦截: 拦截,
	})
}

func (管理器 *U管理器) 日志摘要(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	查询 := 日志查询参数(请求)
	桶毫秒 := int64(60_000)
	if 跨度 := 查询.U到 - 查询.U从; 跨度 > 6*3_600_000 {
		桶毫秒 = int64(15 * 60_000)
	} else if 跨度 > 3_600_000 {
		桶毫秒 = int64(5 * 60_000)
	}
	摘要, 错误 := 管理器.存储.U日志摘要(请求.Context(), 查询, 桶毫秒)
	if 错误 != nil {
		管理器.日志.Error("log summary failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read log summary")
		return
	}
	写JSON(写, http.StatusOK, 摘要)
}

// 日志查询参数 复用日志列表的过滤解析（不含分页）。
func 日志查询参数(请求 *http.Request) 存储包.U日志查询 {
	查询 := 请求.URL.Query()
	解析毫秒 := func(名称 string) int64 {
		值, 错误 := time.Parse(time.RFC3339, 查询.Get(名称))
		if 错误 != nil {
			return 0
		}
		return 值.UnixMilli()
	}
	状态, _ := strconv.Atoi(查询.Get("status"))
	return 存储包.U日志查询{
		U从:    解析毫秒("from"),
		U到:    解析毫秒("to"),
		USite: 查询.Get("site"),
		U动作:   查询.Get("action"),
		U类别:   查询.Get("category"),
		U规则ID: 查询.Get("rule_id"),
		U地址:   查询.Get("ip"),
		U路径:   查询.Get("path"),
		U状态:   状态,
	}
}
