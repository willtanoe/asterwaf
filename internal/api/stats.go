package 管理接口

import (
	"net/http"
	"time"

	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

type UKPI值 struct {
	U值     float64   `json:"value"`
	U上一值   float64   `json:"previous"`
	U变化百分比 *float64  `json:"delta_pct"`
	U序列    []float64 `json:"series,omitempty"`
}

type U计数项 struct {
	U键  string `json:"key"`
	U数量 int64  `json:"count"`
}

type U时间点 struct {
	U时间 int64 `json:"ts"`
	U请求 int64 `json:"requests"`
	U拦截 int64 `json:"blocked"`
	U记录 int64 `json:"shadow"`
	U限流 int64 `json:"ratelimited"`
}

type UIP行 struct {
	U地址 string `json:"ip"`
	U请求 int64  `json:"requests"`
	U拦截 int64  `json:"blocked"`
}

type U路径行 struct {
	U路径 string `json:"path"`
	U请求 int64  `json:"requests"`
	U拦截 int64  `json:"blocked"`
}

type U分布 struct {
	U类别 []U计数项 `json:"categories"`
	U规则 []U计数项 `json:"rules"`
	U动作 []U计数项 `json:"actions"`
	U站点 []U计数项 `json:"sites"`
}

type U表格 struct {
	U顶级IP []UIP行      `json:"top_ips"`
	U顶级路径 []U路径行      `json:"top_paths"`
	U最新拦截 []存储包.U日志条目 `json:"latest_blocked"`
}

type U范围信息 struct {
	U从  string   `json:"from"`
	U到  string   `json:"to"`
	U桶  string   `json:"bucket"`
	U近似 bool     `json:"approximate"`
	U备注 []string `json:"notes"`
}

type U概览响应 struct {
	U范围  U范围信息            `json:"range"`
	UKPI map[string]UKPI值 `json:"kpis"`
	U序列  []U时间点           `json:"series"`
	U分布  U分布              `json:"breakdowns"`
	U表格  U表格              `json:"tables"`
}

type 桶统计 struct {
	请求 int64
	拦截 int64
	记录 int64
	限流 int64
	延迟 [12]int64
}

func (管理器 *U管理器) 概览(写 http.ResponseWriter, 请求 *http.Request) {
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

	当前点, 错误 := 管理器.存储.U聚合区间(上下文, 范围.从, 范围.到, 站点, 范围.桶毫秒)
	if 错误 != nil {
		管理器.日志.Error("aggregate current window failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	上一从 := 范围.从 - (范围.到 - 范围.从)
	上一点, 错误 := 管理器.存储.U聚合区间(上下文, 上一从, 范围.从, 站点, 范围.桶毫秒)
	if 错误 != nil {
		管理器.日志.Error("aggregate previous window failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}

	序列, 桶表 := 构建序列(范围, 当前点)
	当前合计 := 合计桶(桶表)
	上一合计 := 合计点(上一点)
	近似 := 范围.近似
	当前IP, 错误 := 管理器.存储.U唯一IP数(上下文, 范围.从, 范围.到, 站点, 近似)
	if 错误 != nil {
		管理器.日志.Error("unique ip count failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	上一IP, 错误 := 管理器.存储.U唯一IP数(上下文, 上一从, 范围.从, 站点, 近似)
	if 错误 != nil {
		管理器.日志.Error("previous unique ip count failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}

	KPI := map[string]UKPI值{
		"total_requests":       {U值: float64(当前合计.请求), U上一值: float64(上一合计.请求), U变化百分比: 变化(float64(当前合计.请求), float64(上一合计.请求)), U序列: 序列字段(序列, "requests")},
		"blocked_requests":     {U值: float64(当前合计.拦截), U上一值: float64(上一合计.拦截), U变化百分比: 变化(float64(当前合计.拦截), float64(上一合计.拦截)), U序列: 序列字段(序列, "blocked")},
		"shadow_requests":      {U值: float64(当前合计.记录), U上一值: float64(上一合计.记录), U变化百分比: 变化(float64(当前合计.记录), float64(上一合计.记录)), U序列: 序列字段(序列, "shadow")},
		"ratelimited_requests": {U值: float64(当前合计.限流), U上一值: float64(上一合计.限流), U变化百分比: 变化(float64(当前合计.限流), float64(上一合计.限流)), U序列: 序列字段(序列, "ratelimited")},
		"block_rate":           {U值: 比率(当前合计.拦截, 当前合计.请求), U上一值: 比率(上一合计.拦截, 上一合计.请求), U变化百分比: 变化(比率(当前合计.拦截, 当前合计.请求), 比率(上一合计.拦截, 上一合计.请求)), U序列: 比率序列(序列)},
		"unique_ips":           {U值: float64(当前IP), U上一值: float64(上一IP), U变化百分比: 变化(float64(当前IP), float64(上一IP))},
		"latency_p50_ms":       {U值: 存储包.U估算百分位(当前合计.延迟, 0.50), U上一值: 存储包.U估算百分位(上一合计.延迟, 0.50), U变化百分比: 变化(存储包.U估算百分位(当前合计.延迟, 0.50), 存储包.U估算百分位(上一合计.延迟, 0.50)), U序列: 百分位序列(序列, 桶表, 0.50)},
		"latency_p95_ms":       {U值: 存储包.U估算百分位(当前合计.延迟, 0.95), U上一值: 存储包.U估算百分位(上一合计.延迟, 0.95), U变化百分比: 变化(存储包.U估算百分位(当前合计.延迟, 0.95), 存储包.U估算百分位(上一合计.延迟, 0.95)), U序列: 百分位序列(序列, 桶表, 0.95)},
	}

	分布, 错误 := 管理器.读取分布(请求, 范围, 站点)
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read breakdowns")
		return
	}
	表格, 错误 := 管理器.读取表格(请求, 范围, 站点)
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read tables")
		return
	}
	写JSON(写, http.StatusOK, U概览响应{
		U范围: U范围信息{
			U从:  范围.时间从.UTC().Format(time.RFC3339),
			U到:  范围.时间到.UTC().Format(time.RFC3339),
			U桶:  范围.桶,
			U近似: 范围.近似,
			U备注: 范围.备注,
		},
		UKPI: KPI,
		U序列:  序列,
		U分布:  分布,
		U表格:  表格,
	})
}

type 概览范围 struct {
	时间从 time.Time
	时间到 time.Time
	从   int64
	到   int64
	桶   string
	桶毫秒 int64
	近似  bool
	备注  []string
}

func 解析概览范围(请求 *http.Request) (概览范围, error) {
	查询 := 请求.URL.Query()
	现在 := time.Now().UTC()
	var 从, 到 time.Time
	var 错误 error
	范围值 := 查询.Get("range")
	if 范围值 != "" {
		if 查询.Get("from") != "" || 查询.Get("to") != "" {
			return 概览范围{}, 范围错误("range cannot be combined with from and to")
		}
		var 时长 time.Duration
		switch 范围值 {
		case "15m":
			时长 = 15 * time.Minute
		case "1h":
			时长 = time.Hour
		case "24h":
			时长 = 24 * time.Hour
		case "7d":
			时长 = 7 * 24 * time.Hour
		default:
			return 概览范围{}, 范围错误("range must be 15m, 1h, 24h or 7d")
		}
		到 = 现在
		从 = 到.Add(-时长)
	} else {
		if 查询.Get("from") == "" || 查询.Get("to") == "" {
			return 概览范围{}, 范围错误("provide range or both from and to")
		}
		从, 错误 = time.Parse(time.RFC3339, 查询.Get("from"))
		if 错误 != nil {
			return 概览范围{}, 范围错误("from must be RFC3339")
		}
		到, 错误 = time.Parse(time.RFC3339, 查询.Get("to"))
		if 错误 != nil {
			return 概览范围{}, 范围错误("to must be RFC3339")
		}
		从 = 从.UTC()
		到 = 到.UTC()
	}
	if !从.Before(到) {
		return 概览范围{}, 范围错误("from must be before to")
	}
	if 到.Sub(从) > 7*24*time.Hour {
		return 概览范围{}, 范围错误("range must not exceed 7 days")
	}
	桶名 := 查询.Get("bucket")
	if 桶名 == "" {
		桶名 = "auto"
	}
	桶毫秒, 错误 := 桶时长(桶名, 到.Sub(从))
	if 错误 != nil {
		return 概览范围{}, 错误
	}
	实际桶 := 桶名
	if 桶名 == "auto" {
		实际桶 = 自动桶名(到.Sub(从))
	}
	点数 := (到.UnixMilli() - 从.UnixMilli()) / 桶毫秒
	if 点数 > 2000 {
		return 概览范围{}, 范围错误("bucket produces more than 2000 points")
	}
	近似 := 到.Sub(从) > 24*time.Hour
	备注 := []string{"latency percentiles are histogram estimates"}
	if 近似 {
		备注 = append(备注, "unique IPs use hourly rollups")
	}
	return 概览范围{
		时间从: 从, 时间到: 到, 从: 从.UnixMilli(), 到: 到.UnixMilli(),
		桶: 实际桶, 桶毫秒: 桶毫秒, 近似: 近似, 备注: 备注,
	}, nil
}

func 自动桶名(跨度 time.Duration) string {
	switch {
	case 跨度 <= time.Hour:
		return "1m"
	case 跨度 <= 6*time.Hour:
		return "5m"
	case 跨度 <= 24*time.Hour:
		return "15m"
	case 跨度 <= 72*time.Hour:
		return "1h"
	default:
		return "6h"
	}
}

func 桶时长(名称 string, 跨度 time.Duration) (int64, error) {
	switch 名称 {
	case "auto":
		switch {
		case 跨度 <= time.Hour:
			return int64(time.Minute / time.Millisecond), nil
		case 跨度 <= 6*time.Hour:
			return int64(5 * time.Minute / time.Millisecond), nil
		case 跨度 <= 24*time.Hour:
			return int64(15 * time.Minute / time.Millisecond), nil
		case 跨度 <= 72*time.Hour:
			return int64(time.Hour / time.Millisecond), nil
		default:
			return int64(6 * time.Hour / time.Millisecond), nil
		}
	case "1m":
		return int64(time.Minute / time.Millisecond), nil
	case "5m":
		return int64(5 * time.Minute / time.Millisecond), nil
	case "15m":
		return int64(15 * time.Minute / time.Millisecond), nil
	case "1h":
		return int64(time.Hour / time.Millisecond), nil
	case "6h":
		return int64(6 * time.Hour / time.Millisecond), nil
	case "1d":
		return int64(24 * time.Hour / time.Millisecond), nil
	default:
		return 0, 范围错误("bucket must be auto, 1m, 5m, 15m, 1h, 6h or 1d")
	}
}

type 范围错误 string

func (错误 范围错误) Error() string { return string(错误) }

func 构建序列(范围 概览范围, 点集 []存储包.U聚合点) ([]U时间点, map[int64]*桶统计) {
	桶表 := make(map[int64]*桶统计)
	for _, 点 := range 点集 {
		统计 := 桶表[点.U桶]
		if 统计 == nil {
			统计 = &桶统计{}
			桶表[点.U桶] = 统计
		}
		switch 点.U动作 {
		case "block":
			统计.拦截 += 点.U请求
		case "shadow":
			统计.记录 += 点.U请求
		case "ratelimited":
			统计.限流 += 点.U请求
		}
		统计.请求 += 点.U请求
		for 序号, 值 := range 点.U桶值 {
			统计.延迟[序号] += 值
		}
	}
	起点 := 范围.从 / 范围.桶毫秒 * 范围.桶毫秒
	序列 := make([]U时间点, 0, 64)
	for 时刻 := 起点; 时刻 < 范围.到; 时刻 += 范围.桶毫秒 {
		统计 := 桶表[时刻]
		if 统计 == nil {
			统计 = &桶统计{}
		}
		序列 = append(序列, U时间点{
			U时间: 时刻, U请求: 统计.请求, U拦截: 统计.拦截, U记录: 统计.记录, U限流: 统计.限流,
		})
	}
	return 序列, 桶表
}

func 合计桶(桶表 map[int64]*桶统计) 桶统计 {
	var 合计 桶统计
	for _, 统计 := range 桶表 {
		合计.请求 += 统计.请求
		合计.拦截 += 统计.拦截
		合计.记录 += 统计.记录
		合计.限流 += 统计.限流
		for 序号, 值 := range 统计.延迟 {
			合计.延迟[序号] += 值
		}
	}
	return 合计
}

func 合计点(点集 []存储包.U聚合点) 桶统计 {
	var 合计 桶统计
	for _, 点 := range 点集 {
		合计.请求 += 点.U请求
		switch 点.U动作 {
		case "block":
			合计.拦截 += 点.U请求
		case "shadow":
			合计.记录 += 点.U请求
		case "ratelimited":
			合计.限流 += 点.U请求
		}
		for 序号, 值 := range 点.U桶值 {
			合计.延迟[序号] += 值
		}
	}
	return 合计
}

func 变化(当前, 上一 float64) *float64 {
	if 上一 == 0 {
		return nil
	}
	值 := (当前 - 上一) / 上一 * 100
	return &值
}

func 比率(拦截, 总数 int64) float64 {
	if 总数 == 0 {
		return 0
	}
	return float64(拦截) / float64(总数) * 100
}

func 序列字段(序列 []U时间点, 字段 string) []float64 {
	结果 := make([]float64, 0, len(序列))
	for _, 点 := range 序列 {
		switch 字段 {
		case "requests":
			结果 = append(结果, float64(点.U请求))
		case "blocked":
			结果 = append(结果, float64(点.U拦截))
		case "shadow":
			结果 = append(结果, float64(点.U记录))
		case "ratelimited":
			结果 = append(结果, float64(点.U限流))
		}
	}
	return 结果
}

func 比率序列(序列 []U时间点) []float64 {
	结果 := make([]float64, 0, len(序列))
	for _, 点 := range 序列 {
		结果 = append(结果, 比率(点.U拦截, 点.U请求))
	}
	return 结果
}

func 百分位序列(序列 []U时间点, 桶表 map[int64]*桶统计, 分位 float64) []float64 {
	结果 := make([]float64, 0, len(序列))
	for _, 点 := range 序列 {
		统计 := 桶表[点.U时间]
		if 统计 == nil {
			结果 = append(结果, 0)
			continue
		}
		结果 = append(结果, 存储包.U估算百分位(统计.延迟, 分位))
	}
	return 结果
}

func (管理器 *U管理器) 读取分布(请求 *http.Request, 范围 概览范围, 站点 string) (U分布, error) {
	结果 := U分布{U类别: []U计数项{}, U规则: []U计数项{}, U动作: []U计数项{}, U站点: []U计数项{}}
	类别, 错误 := 管理器.存储.U分布统计(请求.Context(), 范围.从, 范围.到, 站点, "category", 10)
	if 错误 != nil {
		return 结果, 错误
	}
	规则, 错误 := 管理器.存储.U分布统计(请求.Context(), 范围.从, 范围.到, 站点, "rule", 10)
	if 错误 != nil {
		return 结果, 错误
	}
	动作, 错误 := 管理器.存储.U分布统计(请求.Context(), 范围.从, 范围.到, 站点, "action", 10)
	if 错误 != nil {
		return 结果, 错误
	}
	站点分布, 错误 := 管理器.存储.U分布统计(请求.Context(), 范围.从, 范围.到, "", "site", 10)
	if 错误 != nil {
		return 结果, 错误
	}
	结果.U类别 = 转计数(类别)
	结果.U规则 = 转计数(规则)
	结果.U动作 = 转计数(动作)
	结果.U站点 = 转计数(站点分布)
	return 结果, nil
}

func (管理器 *U管理器) 读取表格(请求 *http.Request, 范围 概览范围, 站点 string) (U表格, error) {
	结果 := U表格{U顶级IP: []UIP行{}, U顶级路径: []U路径行{}, U最新拦截: []存储包.U日志条目{}}
	IP列表, 错误 := 管理器.存储.U排行IP(请求.Context(), 范围.从, 范围.到, 站点, 范围.近似, 10)
	if 错误 != nil {
		return 结果, 错误
	}
	for _, 项 := range IP列表 {
		结果.U顶级IP = append(结果.U顶级IP, UIP行{U地址: 项.U地址, U请求: 项.U请求, U拦截: 项.U拦截})
	}
	路径列表, 错误 := 管理器.存储.U排行路径(请求.Context(), 范围.从, 范围.到, 站点, 范围.近似, 10)
	if 错误 != nil {
		return 结果, 错误
	}
	for _, 项 := range 路径列表 {
		结果.U顶级路径 = append(结果.U顶级路径, U路径行{U路径: 项.U路径, U请求: 项.U请求, U拦截: 项.U拦截})
	}
	最新, 错误 := 管理器.存储.U最新拦截(请求.Context(), 站点, 10)
	if 错误 != nil {
		return 结果, 错误
	}
	结果.U最新拦截 = 最新
	return 结果, nil
}

func 转计数(列表 []存储包.U计数) []U计数项 {
	结果 := make([]U计数项, 0, len(列表))
	for _, 项 := range 列表 {
		结果 = append(结果, U计数项{U键: 项.U名称, U数量: 项.U数量})
	}
	return 结果
}
