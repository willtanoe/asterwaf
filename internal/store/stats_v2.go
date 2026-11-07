package 存储

import (
	"context"
	"fmt"
	"os"
)

// 本文件汇集概览扩展面板与页面级统计的查询。原始日志维度（状态类、方法、
// UA、字节）只在 24 小时内提供；调用方对更长窗口返回"仅短窗口可用"。
// 所有查询必须走索引：EXPLAIN QUERY PLAN 由 Test概览扩展查询走索引 守护。

// U检测序列点 是某桶某类别的检出数量。
type U检测序列点 struct {
	U桶  int64
	U类别 string
	U数量 int64
}

// U检测序列 供堆叠柱状图使用；只统计非空类别（即检出）。
func (存储 *U存储) U检测序列(上下文 context.Context, 从, 到 int64, 站点 string, 桶毫秒 int64) ([]U检测序列点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (minute / ?) * ? AS bucket, category, SUM(requests)
		FROM rollup_minute WHERE minute >= ? AND minute < ? AND category != ''
		AND (? = '' OR site = ?)
		GROUP BY bucket, category ORDER BY bucket`, 桶毫秒, 桶毫秒, 从, 到, 站点, 站点)
	if 错误 != nil {
		return nil, fmt.Errorf("store: detection series: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U检测序列点, 0, 128)
	for 行列表.Next() {
		var 点 U检测序列点
		if 错误 := 行列表.Scan(&点.U桶, &点.U类别, &点.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan detection series: %w", 错误)
		}
		结果 = append(结果, 点)
	}
	return 结果, nil
}

// U状态类序列 是 2xx-5xx 的桶级计数；原始日志维度，仅短窗口。
type U状态类序列点 struct {
	U桶  int64
	U类  string
	U数量 int64
}

func (存储 *U存储) U状态类序列(上下文 context.Context, 从, 到 int64, 站点 string, 桶毫秒 int64) ([]U状态类序列点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (ts / ?) * ? AS bucket, status / 100 AS class, COUNT(*)
		FROM logs WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?)
		GROUP BY bucket, class ORDER BY bucket`, 桶毫秒, 桶毫秒, 从, 到, 站点, 站点)
	if 错误 != nil {
		return nil, fmt.Errorf("store: status class series: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U状态类序列点, 0, 128)
	for 行列表.Next() {
		var 点 U状态类序列点
		var 类 int64
		if 错误 := 行列表.Scan(&点.U桶, &类, &点.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan status class series: %w", 错误)
		}
		点.U类 = fmt.Sprintf("%dxx", 类)
		结果 = append(结果, 点)
	}
	return 结果, nil
}

// U方法计数 原始日志维度，仅短窗口。
func (存储 *U存储) U方法计数(上下文 context.Context, 从, 到 int64, 站点 string, 限制 int) ([]U计数, error) {
	return 存储.取计数(上下文, `SELECT method, COUNT(*) FROM logs
		WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?)
		GROUP BY method ORDER BY COUNT(*) DESC LIMIT ?`, 从, 到, 站点, 站点, 限制)
}

// U字节序列 原始日志维度，仅短窗口。
type U字节序列点 struct {
	U桶   int64
	U字节入 int64
	U字节出 int64
}

func (存储 *U存储) U字节序列(上下文 context.Context, 从, 到 int64, 站点 string, 桶毫秒 int64) ([]U字节序列点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (ts / ?) * ? AS bucket, SUM(bytes_in), SUM(bytes_out)
		FROM logs WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?)
		GROUP BY bucket ORDER BY bucket`, 桶毫秒, 桶毫秒, 从, 到, 站点, 站点)
	if 错误 != nil {
		return nil, fmt.Errorf("store: byte series: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U字节序列点, 0, 128)
	for 行列表.Next() {
		var 点 U字节序列点
		if 错误 := 行列表.Scan(&点.U桶, &点.U字节入, &点.U字节出); 错误 != nil {
			return nil, fmt.Errorf("store: scan byte series: %w", 错误)
		}
		结果 = append(结果, 点)
	}
	return 结果, nil
}

// U影子拦截 每站点"影子模式本应拦截"的数量。
type U影子拦截 struct {
	U站点 string
	U数量 int64
}

func (存储 *U存储) U影子拦截(上下文 context.Context, 从, 到 int64, 限制 int) ([]U影子拦截, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT site, COUNT(*) FROM logs
		WHERE ts >= ? AND ts < ? AND action = 'shadow' AND rule_id != ''
		GROUP BY site ORDER BY COUNT(*) DESC LIMIT ?`, 从, 到, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: shadow would-block: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U影子拦截, 0, 限制)
	for 行列表.Next() {
		var 项 U影子拦截
		if 错误 := 行列表.Scan(&项.U站点, &项.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan shadow would-block: %w", 错误)
		}
		结果 = append(结果, 项)
	}
	return 结果, nil
}

// U站点行 是每站点聚合行（rollup）。
type U站点行 struct {
	U站点  string
	U请求  int64
	U拦截  int64
	U延迟和 int64
	U桶值  [12]int64
}

func (存储 *U存储) U每站点(上下文 context.Context, 从, 到 int64) ([]U站点行, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT site, SUM(requests),
		SUM(CASE WHEN action IN ('block','ratelimited') THEN requests ELSE 0 END),
		SUM(latency_sum_ms),
		SUM(b0), SUM(b1), SUM(b2), SUM(b3), SUM(b4), SUM(b5),
		SUM(b6), SUM(b7), SUM(b8), SUM(b9), SUM(b10), SUM(b11)
		FROM rollup_minute WHERE minute >= ? AND minute < ? AND site != ''
		GROUP BY site ORDER BY SUM(requests) DESC`, 从, 到)
	if 错误 != nil {
		return nil, fmt.Errorf("store: per site: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U站点行, 0, 16)
	for 行列表.Next() {
		var 行 U站点行
		if 错误 := 行列表.Scan(&行.U站点, &行.U请求, &行.U拦截, &行.U延迟和,
			&行.U桶值[0], &行.U桶值[1], &行.U桶值[2], &行.U桶值[3], &行.U桶值[4], &行.U桶值[5],
			&行.U桶值[6], &行.U桶值[7], &行.U桶值[8], &行.U桶值[9], &行.U桶值[10], &行.U桶值[11]); 错误 != nil {
			return nil, fmt.Errorf("store: scan per site: %w", 错误)
		}
		结果 = append(结果, 行)
	}
	return 结果, nil
}

// U规则行统计 供 Rules 页与概览 top rules：原始日志维度（索引 rule_id+ts）。
type U规则统计 struct {
	U规则ID string
	U类别   string
	U命中   int64
	U拦截   int64
	U记录数  int64
	U最后命中 int64
}

func (存储 *U存储) U规则统计(上下文 context.Context, 从, 到 int64, 站点 string, 限制 int) ([]U规则统计, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT rule_id, MAX(category), COUNT(*),
		SUM(CASE WHEN action = 'block' THEN 1 ELSE 0 END),
		SUM(CASE WHEN action = 'shadow' THEN 1 ELSE 0 END),
		MAX(ts) FROM logs
		WHERE ts >= ? AND ts < ? AND rule_id != '' AND (? = '' OR site = ?)
		GROUP BY rule_id ORDER BY COUNT(*) DESC LIMIT ?`, 从, 到, 站点, 站点, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: rule stats: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U规则统计, 0, 限制)
	for 行列表.Next() {
		var 行 U规则统计
		if 错误 := 行列表.Scan(&行.U规则ID, &行.U类别, &行.U命中, &行.U拦截, &行.U记录数, &行.U最后命中); 错误 != nil {
			return nil, fmt.Errorf("store: scan rule stats: %w", 错误)
		}
		结果 = append(结果, 行)
	}
	return 结果, nil
}

// UUA计数 原始日志维度，仅短窗口。
func (存储 *U存储) U用户代理计数(上下文 context.Context, 从, 到 int64, 站点 string, 限制 int) ([]U计数, error) {
	return 存储.取计数(上下文, `SELECT user_agent, COUNT(*) FROM logs
		WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?) AND user_agent != ''
		GROUP BY user_agent ORDER BY COUNT(*) DESC LIMIT ?`, 从, 到, 站点, 站点, 限制)
}

// U热图点 是周几×小时的请求计数，供 7 天热力图。
type U热图点 struct {
	U周几 int64 `json:"dow"`
	U小时 int64 `json:"hour"`
	U数量 int64 `json:"count"`
}

func (存储 *U存储) U热图(上下文 context.Context, 从, 到 int64) ([]U热图点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (strftime('%w', ts / 1000, 'unixepoch')) AS dow,
		(strftime('%H', ts / 1000, 'unixepoch')) AS hod, COUNT(*)
		FROM logs WHERE ts >= ? AND ts < ?
		GROUP BY (strftime('%w', ts / 1000, 'unixepoch')), (strftime('%H', ts / 1000, 'unixepoch'))`, 从, 到)
	if 错误 != nil {
		return nil, fmt.Errorf("store: heatmap: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U热图点, 0, 168)
	for 行列表.Next() {
		var 点 U热图点
		if 错误 := 行列表.Scan(&点.U周几, &点.U小时, &点.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan heatmap: %w", 错误)
		}
		结果 = append(结果, 点)
	}
	return 结果, nil
}

// U数据库大小字节 返回日志数据库文件大小；-1 表示无法读取。
func (存储 *U存储) U数据库大小字节() int64 {
	信息, 错误 := os.Stat(存储.数据库路径)
	if 错误 != nil {
		return -1
	}
	return 信息.Size()
}

// U访问条目统计 是访问列表条目的命中聚合（原始日志维度）。
type U访问条目统计 struct {
	U规则ID string
	U命中   int64
	U最后命中 int64
}

func (存储 *U存储) U访问条目统计(上下文 context.Context, 从, 到 int64, 限制 int) ([]U访问条目统计, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT rule_id, COUNT(*), MAX(ts) FROM logs
		WHERE ts >= ? AND ts < ? AND rule_id LIKE 'acl:%'
		GROUP BY rule_id ORDER BY COUNT(*) DESC LIMIT ?`, 从, 到, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: access entry stats: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U访问条目统计, 0, 限制)
	for 行列表.Next() {
		var 行 U访问条目统计
		if 错误 := 行列表.Scan(&行.U规则ID, &行.U命中, &行.U最后命中); 错误 != nil {
			return nil, fmt.Errorf("store: scan access entry stats: %w", 错误)
		}
		结果 = append(结果, 行)
	}
	return 结果, nil
}

// U限制序列 是限流决策的桶级计数（rollup 维度）。
type U限制序列点 struct {
	U桶  int64
	U数量 int64
}

func (存储 *U存储) U限制序列(上下文 context.Context, 从, 到 int64, 桶毫秒 int64) ([]U限制序列点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (minute / ?) * ? AS bucket, SUM(requests)
		FROM rollup_minute WHERE minute >= ? AND minute < ? AND action = 'ratelimited'
		GROUP BY bucket ORDER BY bucket`, 桶毫秒, 桶毫秒, 从, 到)
	if 错误 != nil {
		return nil, fmt.Errorf("store: ratelimit series: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U限制序列点, 0, 128)
	for 行列表.Next() {
		var 点 U限制序列点
		if 错误 := 行列表.Scan(&点.U桶, &点.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan ratelimit series: %w", 错误)
		}
		结果 = append(结果, 点)
	}
	return 结果, nil
}

// U顶级拦截IP 供访问列表页"Recent top offenders"。
func (存储 *U存储) U顶级拦截IP(上下文 context.Context, 从, 到 int64, 限制 int) ([]UIP计数, error) {
	计数, 错误 := 存储.取计数(上下文, `SELECT client_ip, COUNT(*) FROM logs
		WHERE ts >= ? AND ts < ? AND action IN ('block','ratelimited')
		GROUP BY client_ip ORDER BY COUNT(*) DESC LIMIT ?`, 从, 到, 限制)
	if 错误 != nil {
		return nil, 错误
	}
	结果 := make([]UIP计数, 0, len(计数))
	for _, 项 := range 计数 {
		结果 = append(结果, UIP计数{U地址: 项.U名称, U请求: 项.U数量})
	}
	return 结果, nil
}

// U规则序列点 是某桶某规则的命中数，供规则页 sparkline。
type U规则序列点 struct {
	U桶    int64
	U规则ID string
	U数量   int64
}

func (存储 *U存储) U规则序列(上下文 context.Context, 从, 到 int64, 站点 string, 桶毫秒 int64) ([]U规则序列点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (minute / ?) * ? AS bucket, rule_id, SUM(requests)
		FROM rollup_minute WHERE minute >= ? AND minute < ? AND rule_id != ''
		AND (? = '' OR site = ?)
		GROUP BY bucket, rule_id ORDER BY bucket`, 桶毫秒, 桶毫秒, 从, 到, 站点, 站点)
	if 错误 != nil {
		return nil, fmt.Errorf("store: rule series: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U规则序列点, 0, 256)
	for 行列表.Next() {
		var 点 U规则序列点
		if 错误 := 行列表.Scan(&点.U桶, &点.U规则ID, &点.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan rule series: %w", 错误)
		}
		结果 = append(结果, 点)
	}
	return 结果, nil
}

// U动作计数 按动作统计原始日志行数（索引 action+ts）。
func (存储 *U存储) U动作计数(上下文 context.Context, 从, 到 int64, 动作 string) (int64, error) {
	var 数量 int64
	if 错误 := 存储.数据库.QueryRowContext(上下文, `SELECT COUNT(*) FROM logs
		WHERE ts >= ? AND ts < ? AND action = ?`, 从, 到, 动作).Scan(&数量); 错误 != nil {
		return 0, fmt.Errorf("store: action count: %w", 错误)
	}
	return 数量, nil
}

// U模式历史 从审计日志读取某站点的模式变更记录。
type U模式历史项 struct {
	U时间  int64  `json:"ts"`
	U操作者 string `json:"actor"`
	U前   string `json:"before"`
	U后   string `json:"after"`
}

func (存储 *U存储) U模式历史(上下文 context.Context, 主机 string, 限制 int) ([]U模式历史项, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT ts, actor, before, after FROM audit_log
		WHERE action = 'site_mode_update' AND after LIKE ?
		ORDER BY ts DESC, id DESC LIMIT ?`, `%"host":"`+主机+`"%`, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: site mode history: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U模式历史项, 0, 限制)
	for 行列表.Next() {
		var 项 U模式历史项
		if 错误 := 行列表.Scan(&项.U时间, &项.U操作者, &项.U前, &项.U后); 错误 != nil {
			return nil, fmt.Errorf("store: scan site mode history: %w", 错误)
		}
		结果 = append(结果, 项)
	}
	return 结果, nil
}

// U日志摘要结果 是日志页过滤视图的聚合；series 与过滤条件同源。
type U日志摘要结果 struct {
	U行数    int64    `json:"rows"`
	U拦截    int64    `json:"blocked"`
	U记录数   int64    `json:"recorded"`
	U唯一IP数 int64    `json:"unique_ips"`
	U最早    int64    `json:"first_ts"`
	U最晚    int64    `json:"last_ts"`
	U序列    []U日志摘要点 `json:"series"`
}

type U日志摘要点 struct {
	U桶  int64 `json:"ts"`
	U行数 int64 `json:"rows"`
	U拦截 int64 `json:"blocked"`
}

// U日志摘要 复用日志查询条件（不含分页），全部走 ts/site 索引。
func (存储 *U存储) U日志摘要(上下文 context.Context, 查询 U日志查询, 桶毫秒 int64) (U日志摘要结果, error) {
	条件, 参数 := 日志查询条件(查询)
	结果 := U日志摘要结果{U序列: []U日志摘要点{}}
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT COUNT(*),
		SUM(CASE WHEN action = 'block' THEN 1 ELSE 0 END),
		SUM(CASE WHEN action = 'shadow' THEN 1 ELSE 0 END),
		COUNT(DISTINCT client_ip), MIN(ts), MAX(ts) FROM logs `+条件, 参数...)
	if 错误 != nil {
		return 结果, fmt.Errorf("store: log summary: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	if !行列表.Next() {
		return 结果, fmt.Errorf("store: log summary: no row")
	}
	if 错误 := 行列表.Scan(&结果.U行数, &结果.U拦截, &结果.U记录数, &结果.U唯一IP数, &结果.U最早, &结果.U最晚); 错误 != nil {
		return 结果, fmt.Errorf("store: scan log summary: %w", 错误)
	}
	if 错误 := 行列表.Err(); 错误 != nil {
		return 结果, fmt.Errorf("store: log summary rows: %w", 错误)
	}
	if 结果.U行数 == 0 {
		return 结果, nil
	}
	序列, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (ts / ?) * ? AS bucket, COUNT(*),
		SUM(CASE WHEN action = 'block' THEN 1 ELSE 0 END) FROM logs `+条件+
		` GROUP BY bucket ORDER BY bucket`, append([]any{桶毫秒, 桶毫秒}, 参数...)...)
	if 错误 != nil {
		return 结果, fmt.Errorf("store: log summary series: %w", 错误)
	}
	defer func() { _ = 序列.Close() }()
	for 序列.Next() {
		var 点 U日志摘要点
		if 错误 := 序列.Scan(&点.U桶, &点.U行数, &点.U拦截); 错误 != nil {
			return 结果, fmt.Errorf("store: scan log summary series: %w", 错误)
		}
		结果.U序列 = append(结果.U序列, 点)
	}
	return 结果, nil
}
