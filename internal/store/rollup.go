package 存储

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	原始保留时长 = 14 * 24 * time.Hour
	分钟保留时长 = 30 * 24 * time.Hour
	小时保留时长 = 180 * 24 * time.Hour
	审计保留时长 = 365 * 24 * time.Hour
	删除批次大小 = 1000
)

var 延迟上界 = []int64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500}

// 延迟桶 返回 12 个直方图桶中的下标，最后一桶表示大于 2500ms。
func 延迟桶(毫秒 int64) int {
	for 序号, 上界 := range 延迟上界 {
		if 毫秒 <= 上界 {
			return 序号
		}
	}
	return len(延迟上界)
}

func 聚合时刻(毫秒 int64, 粒度 int64) int64 {
	return 毫秒 / 粒度 * 粒度
}

func 是否拦截(动作 string) int {
	if 动作 == "block" || 动作 == "ratelimited" {
		return 1
	}
	return 0
}

func 写入聚合(事务 *sql.Tx, 记录 U记录) error {
	分钟 := 聚合时刻(记录.U时间, 60_000)
	小时 := 聚合时刻(记录.U时间, 3_600_000)
	桶 := 延迟桶(记录.U耗时毫秒)
	桶值 := make([]any, 12)
	for 序号 := range 桶值 {
		if 序号 == 桶 {
			桶值[序号] = 1
		} else {
			桶值[序号] = 0
		}
	}
	分钟参数 := []any{分钟, 记录.USite, 记录.U动作, 记录.U类别, 记录.U规则ID, 记录.U耗时毫秒}
	分钟参数 = append(分钟参数, 桶值...)
	if _, 错误 := 事务.Exec(`INSERT INTO rollup_minute (
		minute, site, action, category, rule_id, requests, latency_sum_ms,
		b0, b1, b2, b3, b4, b5, b6, b7, b8, b9, b10, b11
	) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(minute, site, action, category, rule_id) DO UPDATE SET
		requests = requests + 1,
		latency_sum_ms = latency_sum_ms + excluded.latency_sum_ms,
		b0 = b0 + excluded.b0, b1 = b1 + excluded.b1, b2 = b2 + excluded.b2,
		b3 = b3 + excluded.b3, b4 = b4 + excluded.b4, b5 = b5 + excluded.b5,
		b6 = b6 + excluded.b6, b7 = b7 + excluded.b7, b8 = b8 + excluded.b8,
		b9 = b9 + excluded.b9, b10 = b10 + excluded.b10, b11 = b11 + excluded.b11`,
		分钟参数...); 错误 != nil {
		return fmt.Errorf("store: minute rollup: %w", 错误)
	}

	拦截 := 是否拦截(记录.U动作)
	if _, 错误 := 事务.Exec(`INSERT INTO rollup_hour_ip (hour, site, client_ip, requests, blocked)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(hour, site, client_ip) DO UPDATE SET
			requests = requests + 1, blocked = blocked + excluded.blocked`,
		小时, 记录.USite, 记录.U客户端IP, 拦截); 错误 != nil {
		return fmt.Errorf("store: ip rollup: %w", 错误)
	}
	if _, 错误 := 事务.Exec(`INSERT INTO rollup_hour_path (hour, site, path, requests, blocked)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(hour, site, path) DO UPDATE SET
			requests = requests + 1, blocked = blocked + excluded.blocked`,
		小时, 记录.USite, 规范化路径(记录.U路径), 拦截); 错误 != nil {
		return fmt.Errorf("store: path rollup: %w", 错误)
	}
	return nil
}

// U清理保留 按小时分批删除过期原始日志，并清掉过期聚合与审计。
func (存储 *U存储) U清理保留(现在 time.Time) error {
	原始截止 := 现在.Add(-原始保留时长).UnixMilli()
	for {
		结果, 错误 := 存储.数据库.Exec(`DELETE FROM logs WHERE id IN (
			SELECT id FROM logs WHERE ts < ? LIMIT ?
		)`, 原始截止, 删除批次大小)
		if 错误 != nil {
			return fmt.Errorf("store: delete expired logs: %w", 错误)
		}
		数量, 错误 := 结果.RowsAffected()
		if 错误 != nil {
			return fmt.Errorf("store: count deleted logs: %w", 错误)
		}
		if 数量 < 删除批次大小 {
			break
		}
	}
	if _, 错误 := 存储.数据库.Exec(`DELETE FROM rollup_minute WHERE minute < ?`,
		现在.Add(-分钟保留时长).UnixMilli()); 错误 != nil {
		return fmt.Errorf("store: delete minute rollups: %w", 错误)
	}
	if _, 错误 := 存储.数据库.Exec(`DELETE FROM rollup_hour_ip WHERE hour < ?`,
		现在.Add(-小时保留时长).UnixMilli()); 错误 != nil {
		return fmt.Errorf("store: delete hour ip rollups: %w", 错误)
	}
	if _, 错误 := 存储.数据库.Exec(`DELETE FROM rollup_hour_path WHERE hour < ?`,
		现在.Add(-小时保留时长).UnixMilli()); 错误 != nil {
		return fmt.Errorf("store: delete hour path rollups: %w", 错误)
	}
	审计截止 := 现在.Add(-审计保留时长).UnixMilli()
	for {
		结果, 错误 := 存储.数据库.Exec(`DELETE FROM audit_log WHERE id IN (
			SELECT id FROM audit_log WHERE ts < ? LIMIT ?
		)`, 审计截止, 删除批次大小)
		if 错误 != nil {
			return fmt.Errorf("store: delete audit rows: %w", 错误)
		}
		数量, 错误 := 结果.RowsAffected()
		if 错误 != nil {
			return fmt.Errorf("store: count deleted audit rows: %w", 错误)
		}
		if 数量 < 删除批次大小 {
			break
		}
	}
	return nil
}

func (存储 *U存储) 保留循环() {
	defer close(存储.保留完成)
	计时器 := time.NewTicker(time.Hour)
	defer 计时器.Stop()
	for {
		select {
		case <-计时器.C:
			if 错误 := 存储.U清理保留(time.Now()); 错误 != nil {
				存储.日志.Error("retention cleanup failed", "error", 错误)
			}
		case <-存储.保留停止:
			return
		}
	}
}
