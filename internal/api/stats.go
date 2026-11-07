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

type U检测序列项 struct {
	U键  string   `json:"key"`
	U点集 []U计数时间点 `json:"points"`
}

type U计数时间点 struct {
	U时间 int64 `json:"ts"`
	U数量 int64 `json:"count"`
}

type U状态序列项 struct {
	U键  string   `json:"key"`
	U点集 []U计数时间点 `json:"points"`
}

type U字节序列项 struct {
	U时间  int64 `json:"ts"`
	U字节入 int64 `json:"in"`
	U字节出 int64 `json:"out"`
}

type U影子行 struct {
	U站点 string `json:"site"`
	U数量 int64  `json:"count"`
}

type U站点聚合行 struct {
	U站点    string  `json:"site"`
	U请求    int64   `json:"requests"`
	U拦截    int64   `json:"blocked"`
	U拦截率   float64 `json:"block_rate"`
	U延迟P95 float64 `json:"p95"`
}

type U规则行 struct {
	U规则ID string `json:"rule_id"`
	U类别   string `json:"category"`
	U命中   int64  `json:"hits"`
	U拦截   int64  `json:"blocked"`
	U记录   int64  `json:"recorded"`
	U最后命中 int64  `json:"last_seen"`
}

type U热图点 struct {
	U周几 int64 `json:"dow"`
	U小时 int64 `json:"hour"`
	U数量 int64 `json:"count"`
}

type U系统信息 struct {
	U数据库字节 int64          `json:"db_size_bytes"`
	U保留    map[string]int `json:"retention_days"`
	U限流    map[string]any `json:"rate_limit"`
}

type U概览响应 struct {
	U范围    U范围信息            `json:"range"`
	UKPI   map[string]UKPI值 `json:"kpis"`
	U序列    []U时间点           `json:"series"`
	U分布    U分布              `json:"breakdowns"`
	U表格    U表格              `json:"tables"`
	U限流已启用 bool             `json:"rate_limit_enabled"`
	U检测序列  []U检测序列项         `json:"detections_series"`
	U延迟序列  []U延迟点           `json:"latency_series"`
	U状态序列  []U状态序列项         `json:"status_series"`
	U方法    []U计数项           `json:"methods"`
	U字节序列  []U字节序列项         `json:"bytes_series"`
	U影子拦截  []U影子行           `json:"shadow_wouldblock"`
	U每站点   []U站点聚合行         `json:"per_site"`
	U顶级规则  []U规则行           `json:"top_rules"`
	U顶级UA  []U计数项           `json:"top_user_agents"`
	U热图    []U热图点           `json:"heatmap"`
	U原始可用  bool             `json:"raw_available"`
	U系统    U系统信息            `json:"system"`
}

type U延迟点 struct {
	U时间 int64   `json:"ts"`
	U50 float64 `json:"p50"`
	U95 float64 `json:"p95"`
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

	当前IP序列, 错误 := 管理器.存储.U唯一IP序列(上下文, 范围.从, 范围.到, 站点, 范围.桶毫秒)
	if 错误 != nil {
		管理器.日志.Error("unique ip series failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	IP序列表 := make(map[int64]int64, len(当前IP序列))
	for _, 点 := range 当前IP序列 {
		IP序列表[点.U桶] = 点.U数量
	}

	KPI := map[string]UKPI值{
		"total_requests":       {U值: float64(当前合计.请求), U上一值: float64(上一合计.请求), U变化百分比: 变化(float64(当前合计.请求), float64(上一合计.请求)), U序列: 序列字段(序列, "requests")},
		"blocked_requests":     {U值: float64(当前合计.拦截), U上一值: float64(上一合计.拦截), U变化百分比: 变化(float64(当前合计.拦截), float64(上一合计.拦截)), U序列: 序列字段(序列, "blocked")},
		"shadow_requests":      {U值: float64(当前合计.记录), U上一值: float64(上一合计.记录), U变化百分比: 变化(float64(当前合计.记录), float64(上一合计.记录)), U序列: 序列字段(序列, "shadow")},
		"ratelimited_requests": {U值: float64(当前合计.限流), U上一值: float64(上一合计.限流), U变化百分比: 变化(float64(当前合计.限流), float64(上一合计.限流)), U序列: 序列字段(序列, "ratelimited")},
		"block_rate":           {U值: 比率(当前合计.拦截, 当前合计.请求), U上一值: 比率(上一合计.拦截, 上一合计.请求), U变化百分比: 变化(比率(当前合计.拦截, 当前合计.请求), 比率(上一合计.拦截, 上一合计.请求)), U序列: 比率序列(序列)},
		"unique_ips":           {U值: float64(当前IP), U上一值: float64(上一IP), U变化百分比: 变化(float64(当前IP), float64(上一IP)), U序列: 唯一IP序列字段(序列, IP序列表)},
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
	限流配置, 错误 := 管理器.存储.U读取限流(上下文)
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rate limit")
		return
	}
	// 原始日志维度只在 24 小时窗口内提供；更长窗口返回 raw_available=false。
	原始可用 := !范围.近似
	检测序列, 错误 := 管理器.存储.U检测序列(上下文, 范围.从, 范围.到, 站点, 范围.桶毫秒)
	if 错误 != nil {
		管理器.日志.Error("detection series failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	检测组 := 分组检测(范围, 检测序列)
	延迟序列 := make([]U延迟点, 0, len(序列))
	for _, 点 := range 序列 {
		统计 := 桶表[点.U时间]
		if 统计 == nil {
			延迟序列 = append(延迟序列, U延迟点{U时间: 点.U时间})
			continue
		}
		延迟序列 = append(延迟序列, U延迟点{U时间: 点.U时间,
			U50: 存储包.U估算百分位(统计.延迟, 0.50), U95: 存储包.U估算百分位(统计.延迟, 0.95)})
	}
	var 状态序列 []U状态序列项
	var 方法 []U计数项
	var 字节序列 []U字节序列项
	var 顶级UA []U计数项
	if 原始可用 {
		状态原始, 错误 := 管理器.存储.U状态类序列(上下文, 范围.从, 范围.到, 站点, 范围.桶毫秒)
		if 错误 != nil {
			管理器.日志.Error("status series failed", "error", 错误)
			写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
			return
		}
		状态序列 = 分组状态(范围, 状态原始)
		方法原始, 错误 := 管理器.存储.U方法计数(上下文, 范围.从, 范围.到, 站点, 8)
		if 错误 != nil {
			管理器.日志.Error("method counts failed", "error", 错误)
			写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
			return
		}
		方法 = 转计数(方法原始)
		字节原始, 错误 := 管理器.存储.U字节序列(上下文, 范围.从, 范围.到, 站点, 范围.桶毫秒)
		if 错误 != nil {
			管理器.日志.Error("byte series failed", "error", 错误)
			写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
			return
		}
		字节序列 = make([]U字节序列项, 0, len(字节原始))
		for _, 点 := range 字节原始 {
			字节序列 = append(字节序列, U字节序列项{U时间: 点.U桶, U字节入: 点.U字节入, U字节出: 点.U字节出})
		}
		UA原始, 错误 := 管理器.存储.U用户代理计数(上下文, 范围.从, 范围.到, 站点, 10)
		if 错误 != nil {
			管理器.日志.Error("user agent counts failed", "error", 错误)
			写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
			return
		}
		顶级UA = 转计数(UA原始)
	}
	影子, 错误 := 管理器.存储.U影子拦截(上下文, 范围.从, 范围.到, 10)
	if 错误 != nil {
		管理器.日志.Error("shadow would-block failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	影子行 := make([]U影子行, 0, len(影子))
	for _, 项 := range 影子 {
		影子行 = append(影子行, U影子行{U站点: 项.U站点, U数量: 项.U数量})
	}
	站点行, 错误 := 管理器.存储.U每站点(上下文, 范围.从, 范围.到)
	if 错误 != nil {
		管理器.日志.Error("per-site stats failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	站点聚合 := make([]U站点聚合行, 0, len(站点行))
	for _, 行 := range 站点行 {
		站点聚合 = append(站点聚合, U站点聚合行{U站点: 行.U站点, U请求: 行.U请求, U拦截: 行.U拦截,
			U拦截率: 比率(行.U拦截, 行.U请求), U延迟P95: 存储包.U估算百分位(行.U桶值, 0.95)})
	}
	规则统计, 错误 := 管理器.存储.U规则统计(上下文, 范围.从, 范围.到, 站点, 10)
	if 错误 != nil {
		管理器.日志.Error("top rules failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	规则行 := make([]U规则行, 0, len(规则统计))
	for _, 行 := range 规则统计 {
		规则行 = append(规则行, U规则行{U规则ID: 行.U规则ID, U类别: 行.U类别, U命中: 行.U命中,
			U拦截: 行.U拦截, U记录: 行.U记录数, U最后命中: 行.U最后命中})
	}
	热图, 错误 := 管理器.存储.U热图(上下文, 范围.从, 范围.到)
	if 错误 != nil {
		管理器.日志.Error("heatmap failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read statistics")
		return
	}
	热图点 := make([]U热图点, 0, len(热图))
	for _, 点 := range 热图 {
		热图点 = append(热图点, U热图点{U周几: 点.U周几, U小时: 点.U小时, U数量: 点.U数量})
	}
	写JSON(写, http.StatusOK, U概览响应{
		U限流已启用: 限流配置.U启用,
		U检测序列:  检测组,
		U延迟序列:  延迟序列,
		U状态序列:  状态序列,
		U方法:    方法,
		U字节序列:  字节序列,
		U影子拦截:  影子行,
		U每站点:   站点聚合,
		U顶级规则:  规则行,
		U顶级UA:  顶级UA,
		U热图:    热图点,
		U原始可用:  原始可用,
		U系统: U系统信息{
			U数据库字节: 管理器.存储.U数据库大小字节(),
			U保留:    map[string]int{"raw_days": 14, "minute_rollups": 30, "hour_rollups": 180, "audit_days": 365},
			U限流:    map[string]any{"enabled": 限流配置.U启用, "requests_per_minute": 限流配置.U每分钟, "burst": 限流配置.U突发, "buckets": 管理器.代理.U限流桶数()},
		},
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

func 唯一IP序列字段(序列 []U时间点, 桶表 map[int64]int64) []float64 {
	结果 := make([]float64, 0, len(序列))
	for _, 点 := range 序列 {
		结果 = append(结果, float64(桶表[点.U时间]))
	}
	return 结果
}

func 分组检测(范围 概览范围, 点集 []存储包.U检测序列点) []U检测序列项 {
	起点 := 范围.从 / 范围.桶毫秒 * 范围.桶毫秒
	索引 := map[string]map[int64]int64{}
	for _, 点 := range 点集 {
		桶集, 存在 := 索引[点.U类别]
		if !存在 {
			桶集 = map[int64]int64{}
			索引[点.U类别] = 桶集
		}
		桶集[点.U桶] = 点.U数量
	}
	结果 := make([]U检测序列项, 0, len(索引))
	for 键, 桶集 := range 索引 {
		点列表 := make([]U计数时间点, 0, len(桶集))
		for 时刻 := 起点; 时刻 < 范围.到; 时刻 += 范围.桶毫秒 {
			if 值, 存在 := 桶集[时刻]; 存在 {
				点列表 = append(点列表, U计数时间点{U时间: 时刻, U数量: 值})
			}
		}
		结果 = append(结果, U检测序列项{U键: 键, U点集: 点列表})
	}
	return 结果
}

func 分组状态(范围 概览范围, 点集 []存储包.U状态类序列点) []U状态序列项 {
	起点 := 范围.从 / 范围.桶毫秒 * 范围.桶毫秒
	索引 := map[string]map[int64]int64{}
	for _, 点 := range 点集 {
		桶集, 存在 := 索引[点.U类]
		if !存在 {
			桶集 = map[int64]int64{}
			索引[点.U类] = 桶集
		}
		桶集[点.U桶] = 点.U数量
	}
	结果 := make([]U状态序列项, 0, len(索引))
	for 键, 桶集 := range 索引 {
		点列表 := make([]U计数时间点, 0, len(桶集))
		for 时刻 := 起点; 时刻 < 范围.到; 时刻 += 范围.桶毫秒 {
			if 值, 存在 := 桶集[时刻]; 存在 {
				点列表 = append(点列表, U计数时间点{U时间: 时刻, U数量: 值})
			}
		}
		结果 = append(结果, U状态序列项{U键: 键, U点集: 点列表})
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
