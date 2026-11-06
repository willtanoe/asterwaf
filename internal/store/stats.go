package 存储

import (
	"context"
	"fmt"
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
