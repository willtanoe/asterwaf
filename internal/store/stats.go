package 存储

import (
	"context"
	"fmt"
	"math"
	"time"
)

// U计数 是规则或路径的聚合计数。
type U计数 struct {
	U名称 string
	U数量 int64
}

// U摘要 是 CLI 与 API 共用的基础统计。
type U摘要 struct {
	U请求数   int64
	U允许数   int64
	U拦截数   int64
	U记录数   int64
	U限流数   int64
	U唯一IP数 int64
	U规则    []U计数
	U路径    []U计数
}

// U摘要统计 从分钟与小时聚合读取，唯一 IP 在 24 小时内仍用原始日志。
func (存储 *U存储) U摘要统计(上下文 context.Context, 从, 到 time.Time) (U摘要, error) {
	分钟起 := 从.UnixMilli() / 60_000 * 60_000
	分钟止 := 到.UnixMilli()/60_000*60_000 + 60_000
	小时起 := 从.UnixMilli() / 3_600_000 * 3_600_000
	小时止 := 到.UnixMilli()/3_600_000*3_600_000 + 3_600_000

	var 摘要 U摘要
	if 错误 := 存储.数据库.QueryRowContext(上下文, `SELECT
		COALESCE(SUM(requests), 0),
		COALESCE(SUM(CASE WHEN action = 'allow' THEN requests ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN action = 'block' THEN requests ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN action = 'shadow' THEN requests ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN action = 'ratelimited' THEN requests ELSE 0 END), 0)
		FROM rollup_minute WHERE minute >= ? AND minute < ?`, 分钟起, 分钟止).Scan(
		&摘要.U请求数, &摘要.U允许数, &摘要.U拦截数, &摘要.U记录数, &摘要.U限流数,
	); 错误 != nil {
		return U摘要{}, fmt.Errorf("store: summary totals: %w", 错误)
	}
	if 错误 := 存储.数据库.QueryRowContext(上下文, `SELECT COUNT(DISTINCT client_ip) FROM logs
		WHERE ts >= ? AND ts < ?`, 从.UnixMilli(), 到.UnixMilli()).Scan(&摘要.U唯一IP数); 错误 != nil {
		return U摘要{}, fmt.Errorf("store: summary unique ips: %w", 错误)
	}
	规则, 错误 := 存储.取计数(上下文, `SELECT rule_id, SUM(requests) AS total FROM rollup_minute
		WHERE minute >= ? AND minute < ? AND rule_id != ''
		GROUP BY rule_id ORDER BY total DESC LIMIT 10`, 分钟起, 分钟止)
	if 错误 != nil {
		return U摘要{}, 错误
	}
	摘要.U规则 = 规则
	路径, 错误 := 存储.取计数(上下文, `SELECT path, SUM(requests) AS total FROM rollup_hour_path
		WHERE hour >= ? AND hour < ? AND path != ''
		GROUP BY path ORDER BY total DESC LIMIT 10`, 小时起, 小时止)
	if 错误 != nil {
		return U摘要{}, 错误
	}
	摘要.U路径 = 路径
	return 摘要, nil
}

func (存储 *U存储) 取计数(上下文 context.Context, 查询 string, 参数 ...any) ([]U计数, error) {
	行, 错误 := 存储.数据库.QueryContext(上下文, 查询, 参数...)
	if 错误 != nil {
		return nil, fmt.Errorf("store: summary breakdown: %w", 错误)
	}
	defer func() { _ = 行.Close() }()
	结果 := make([]U计数, 0, 10)
	for 行.Next() {
		var 项 U计数
		if 错误 := 行.Scan(&项.U名称, &项.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan breakdown: %w", 错误)
		}
		结果 = append(结果, 项)
	}
	if 错误 := 行.Err(); 错误 != nil {
		return nil, fmt.Errorf("store: iterate breakdown: %w", 错误)
	}
	return 结果, nil
}

// U聚合点 是某桶某动作的合计与延迟直方图。
type U聚合点 struct {
	U桶   int64
	U动作  string
	U请求  int64
	U延迟和 int64
	U桶值  [12]int64
}

func (存储 *U存储) U聚合区间(上下文 context.Context, 从, 到 int64, 站点 string, 桶毫秒 int64) ([]U聚合点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (minute / ?) * ? AS bucket, action,
		SUM(requests), SUM(latency_sum_ms),
		SUM(b0), SUM(b1), SUM(b2), SUM(b3), SUM(b4), SUM(b5),
		SUM(b6), SUM(b7), SUM(b8), SUM(b9), SUM(b10), SUM(b11)
		FROM rollup_minute WHERE minute >= ? AND minute < ? AND (? = '' OR site = ?)
		GROUP BY bucket, action ORDER BY bucket`, 桶毫秒, 桶毫秒, 从, 到, 站点, 站点)
	if 错误 != nil {
		return nil, fmt.Errorf("store: aggregate stats: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U聚合点, 0, 128)
	for 行列表.Next() {
		var 点 U聚合点
		if 错误 := 行列表.Scan(&点.U桶, &点.U动作, &点.U请求, &点.U延迟和,
			&点.U桶值[0], &点.U桶值[1], &点.U桶值[2], &点.U桶值[3], &点.U桶值[4], &点.U桶值[5],
			&点.U桶值[6], &点.U桶值[7], &点.U桶值[8], &点.U桶值[9], &点.U桶值[10], &点.U桶值[11]); 错误 != nil {
			return nil, fmt.Errorf("store: scan aggregate: %w", 错误)
		}
		结果 = append(结果, 点)
	}
	if 错误 := 行列表.Err(); 错误 != nil {
		return nil, fmt.Errorf("store: iterate aggregates: %w", 错误)
	}
	return 结果, nil
}

func (存储 *U存储) U唯一IP数(上下文 context.Context, 从, 到 int64, 站点 string, 近似 bool) (int64, error) {
	var 数量 int64
	if 近似 {
		if 错误 := 存储.数据库.QueryRowContext(上下文, `SELECT COUNT(DISTINCT client_ip) FROM rollup_hour_ip
			WHERE hour >= ? AND hour < ? AND (? = '' OR site = ?)`,
			从/3_600_000*3_600_000, 到/3_600_000*3_600_000+3_600_000, 站点, 站点).Scan(&数量); 错误 != nil {
			return 0, fmt.Errorf("store: approximate unique ips: %w", 错误)
		}
		return 数量, nil
	}
	if 错误 := 存储.数据库.QueryRowContext(上下文, `SELECT COUNT(DISTINCT client_ip) FROM logs
		WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?)`, 从, 到, 站点, 站点).Scan(&数量); 错误 != nil {
		return 0, fmt.Errorf("store: unique ips: %w", 错误)
	}
	return 数量, nil
}

// U唯一IP序列 按桶统计去重客户端数；仅用于原始日志可用的短窗口。
func (存储 *U存储) U唯一IP序列(上下文 context.Context, 从, 到 int64, 站点 string, 桶毫秒 int64) ([]UIP序列点, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT (ts / ?) * ? AS bucket, COUNT(DISTINCT client_ip)
		FROM logs WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?)
		GROUP BY bucket ORDER BY bucket`, 桶毫秒, 桶毫秒, 从, 到, 站点, 站点)
	if 错误 != nil {
		return nil, fmt.Errorf("store: unique ip series: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]UIP序列点, 0, 64)
	for 行列表.Next() {
		var 点 UIP序列点
		if 错误 := 行列表.Scan(&点.U桶, &点.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan unique ip series: %w", 错误)
		}
		结果 = append(结果, 点)
	}
	return 结果, nil
}

// U分布统计 的维度必须在白名单内，避免把请求参数拼进 SQL。
func (存储 *U存储) U分布统计(上下文 context.Context, 从, 到 int64, 站点, 维度 string, 限制 int) ([]U计数, error) {
	var 列 string
	switch 维度 {
	case "category":
		列 = "category"
	case "rule":
		列 = "rule_id"
	case "action":
		列 = "action"
	case "site":
		列 = "site"
	default:
		return nil, fmt.Errorf("store: unsupported distribution dimension %q", 维度)
	}
	查询 := `SELECT ` + 列 + `, SUM(requests) AS total FROM rollup_minute
		WHERE minute >= ? AND minute < ? AND (? = '' OR site = ?) AND ` + 列 + ` != ''
		GROUP BY ` + 列 + ` ORDER BY total DESC LIMIT ?`
	行列表, 错误 := 存储.数据库.QueryContext(上下文, 查询, 从, 到, 站点, 站点, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: distribution %s: %w", 维度, 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U计数, 0, 限制)
	for 行列表.Next() {
		var 项 U计数
		if 错误 := 行列表.Scan(&项.U名称, &项.U数量); 错误 != nil {
			return nil, fmt.Errorf("store: scan distribution: %w", 错误)
		}
		结果 = append(结果, 项)
	}
	return 结果, nil
}

type UIP序列点 struct {
	U桶  int64
	U数量 int64
}

type UIP计数 struct {
	U地址 string
	U请求 int64
	U拦截 int64
}

type U路径计数 struct {
	U路径 string
	U请求 int64
	U拦截 int64
}

func (存储 *U存储) U排行IP(上下文 context.Context, 从, 到 int64, 站点 string, 近似 bool, 限制 int) ([]UIP计数, error) {
	var 查询 string
	if 近似 {
		查询 = `SELECT client_ip, SUM(requests), SUM(blocked) FROM rollup_hour_ip
			WHERE hour >= ? AND hour < ? AND (? = '' OR site = ?)
			GROUP BY client_ip ORDER BY SUM(requests) DESC LIMIT ?`
		从 = 从 / 3_600_000 * 3_600_000
		到 = 到/3_600_000*3_600_000 + 3_600_000
	} else {
		查询 = `SELECT client_ip, COUNT(*), SUM(CASE WHEN action IN ('block','ratelimited') THEN 1 ELSE 0 END)
			FROM logs WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?)
			GROUP BY client_ip ORDER BY COUNT(*) DESC LIMIT ?`
	}
	行列表, 错误 := 存储.数据库.QueryContext(上下文, 查询, 从, 到, 站点, 站点, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: top ips: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]UIP计数, 0, 限制)
	for 行列表.Next() {
		var 项 UIP计数
		if 错误 := 行列表.Scan(&项.U地址, &项.U请求, &项.U拦截); 错误 != nil {
			return nil, fmt.Errorf("store: scan top ip: %w", 错误)
		}
		结果 = append(结果, 项)
	}
	return 结果, nil
}

func (存储 *U存储) U排行路径(上下文 context.Context, 从, 到 int64, 站点 string, 近似 bool, 限制 int) ([]U路径计数, error) {
	var 查询 string
	if 近似 {
		查询 = `SELECT path, SUM(requests), SUM(blocked) FROM rollup_hour_path
			WHERE hour >= ? AND hour < ? AND (? = '' OR site = ?)
			GROUP BY path ORDER BY SUM(requests) DESC LIMIT ?`
		从 = 从 / 3_600_000 * 3_600_000
		到 = 到/3_600_000*3_600_000 + 3_600_000
	} else {
		查询 = `SELECT path, COUNT(*), SUM(CASE WHEN action IN ('block','ratelimited') THEN 1 ELSE 0 END)
			FROM logs WHERE ts >= ? AND ts < ? AND (? = '' OR site = ?)
			GROUP BY path ORDER BY COUNT(*) DESC LIMIT ?`
	}
	行列表, 错误 := 存储.数据库.QueryContext(上下文, 查询, 从, 到, 站点, 站点, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: top paths: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U路径计数, 0, 限制)
	for 行列表.Next() {
		var 项 U路径计数
		if 错误 := 行列表.Scan(&项.U路径, &项.U请求, &项.U拦截); 错误 != nil {
			return nil, fmt.Errorf("store: scan top path: %w", 错误)
		}
		结果 = append(结果, 项)
	}
	return 结果, nil
}

func (存储 *U存储) U最新拦截(上下文 context.Context, 站点 string, 限制 int) ([]U日志条目, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT id, ts, request_id, site, method, path, query,
		status, action, category, rule_id, shadow, client_ip, user_agent, duration_ms, seed
		FROM logs WHERE action = 'block' AND (? = '' OR site = ?)
		ORDER BY ts DESC, id DESC LIMIT ?`, 站点, 站点, 限制)
	if 错误 != nil {
		return nil, fmt.Errorf("store: latest blocked: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U日志条目, 0, 限制)
	for 行列表.Next() {
		var 条目 U日志条目
		var 影子, 种子 int
		if 错误 := 行列表.Scan(&条目.UID, &条目.U时间, &条目.U请求ID, &条目.USite, &条目.U方法,
			&条目.U路径, &条目.U查询, &条目.U状态, &条目.U动作, &条目.U类别, &条目.U规则ID,
			&影子, &条目.U客户端IP, &条目.U用户代理, &条目.U耗时毫秒, &种子); 错误 != nil {
			return nil, fmt.Errorf("store: scan latest blocked: %w", 错误)
		}
		条目.U影子 = 影子 == 1
		条目.U种子 = 种子 == 1
		结果 = append(结果, 条目)
	}
	return 结果, nil
}

// U估算百分位 用直方图桶上界估算；超过最后一桶按 2500ms 计。
func U估算百分位(桶 [12]int64, 分位 float64) float64 {
	总数 := int64(0)
	for _, 值 := range 桶 {
		总数 += 值
	}
	if 总数 == 0 {
		return 0
	}
	目标 := int64(math.Ceil(float64(总数) * 分位))
	if 目标 < 1 {
		目标 = 1
	}
	累计 := int64(0)
	上界 := []int64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500}
	for 序号, 值 := range 桶 {
		累计 += 值
		if 累计 >= 目标 {
			if 序号 < len(上界) {
				return float64(上界[序号])
			}
			return 2500
		}
	}
	return 2500
}
