package 存储

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/accesslist"
	规则引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/ratelimit"
)

// U未找到错误 供 API 区分 404 与内部错误；用类型而不是包级变量避免命名冲突。
type U未找到错误 struct{}

func (U未找到错误) Error() string { return "store: not found" }

// U规则行 是可持久化规则及其管理元数据。
type U规则行 struct {
	UID    string `json:"id"`
	U名称    string `json:"name"`
	U类别    string `json:"category"`
	U正则    string `json:"pattern"`
	U启用    bool   `json:"enabled"`
	UScore int    `json:"score"`
	U来源    string `json:"source"`
	U默认版本  int    `json:"defaults_version"`
	U顺序    int    `json:"position"`
	U创建时间  int64  `json:"created_at"`
	U更新时间  int64  `json:"updated_at"`
}

// U版本信息 是规则版本列表项。
type U版本信息 struct {
	UID   int64  `json:"id"`
	U创建时间 int64  `json:"created_at"`
	U操作者  string `json:"actor"`
	U备注   string `json:"note"`
	U规则数  int    `json:"rule_count"`
}

func (行 U规则行) U转为规则() 规则引擎.U规则 {
	return 规则引擎.U规则{
		UID:    行.UID,
		U名称:    行.U名称,
		U类别:    行.U类别,
		U正则:    行.U正则,
		U启用:    行.U启用,
		UScore: 行.UScore,
	}
}

func U读取规则(上下文 context.Context, 数据库 *sql.DB) ([]U规则行, error) {
	行列表, 错误 := 数据库.QueryContext(上下文, `SELECT id, name, category, pattern, enabled, score,
		source, defaults_version, position, created_at, updated_at FROM rules ORDER BY position, id`)
	if 错误 != nil {
		return nil, fmt.Errorf("store: read rules: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U规则行, 0, 24)
	for 行列表.Next() {
		var 行 U规则行
		var 启用 int
		if 错误 := 行列表.Scan(&行.UID, &行.U名称, &行.U类别, &行.U正则, &启用, &行.UScore,
			&行.U来源, &行.U默认版本, &行.U顺序, &行.U创建时间, &行.U更新时间); 错误 != nil {
			return nil, fmt.Errorf("store: scan rule: %w", 错误)
		}
		行.U启用 = 启用 == 1
		结果 = append(结果, 行)
	}
	if 错误 := 行列表.Err(); 错误 != nil {
		return nil, fmt.Errorf("store: iterate rules: %w", 错误)
	}
	return 结果, nil
}

func (存储 *U存储) U读取规则(上下文 context.Context) ([]U规则行, error) {
	return U读取规则(上下文, 存储.数据库)
}

// U种子规则 在规则表为空时写入默认规则，并确保至少存在一个版本。
func (存储 *U存储) U种子规则(上下文 context.Context, 默认 []规则引擎.U规则) (int64, error) {
	事务, 错误 := 存储.数据库.BeginTx(上下文, nil)
	if 错误 != nil {
		return 0, fmt.Errorf("store: begin rule seed: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()

	var 规则数 int
	if 错误 := 事务.QueryRowContext(上下文, `SELECT COUNT(*) FROM rules`).Scan(&规则数); 错误 != nil {
		return 0, fmt.Errorf("store: count rules: %w", 错误)
	}
	现在 := time.Now().UnixMilli()
	if 规则数 == 0 {
		语句, 错误 := 事务.PrepareContext(上下文, `INSERT INTO rules
			(id, name, category, pattern, enabled, score, source, defaults_version, position, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'default', ?, ?, ?, ?)`)
		if 错误 != nil {
			return 0, fmt.Errorf("store: prepare rule seed: %w", 错误)
		}
		defer func() { _ = 语句.Close() }()
		for 序号, 规则 := range 默认 {
			启用 := 0
			if 规则.U启用 {
				启用 = 1
			}
			if _, 错误 := 语句.ExecContext(上下文, 规则.UID, 规则.U名称, 规则.U类别, 规则.U正则,
				启用, 规则.UScore, 规则引擎.U默认规则版本, 序号, 现在, 现在); 错误 != nil {
				return 0, fmt.Errorf("store: seed rule %s: %w", 规则.UID, 错误)
			}
		}
	}
	var 最新版本 int64
	if 错误 := 事务.QueryRowContext(上下文, `SELECT COALESCE(MAX(id), 0) FROM ruleset_versions`).Scan(&最新版本); 错误 != nil {
		return 0, fmt.Errorf("store: latest rule version: %w", 错误)
	}
	if 最新版本 == 0 {
		规则列表, 错误 := 读取规则事务(上下文, 事务)
		if 错误 != nil {
			return 0, 错误
		}
		快照, 错误 := json.Marshal(规则快照(规则列表))
		if 错误 != nil {
			return 0, fmt.Errorf("store: encode rule snapshot: %w", 错误)
		}
		结果, 错误 := 事务.ExecContext(上下文, `INSERT INTO ruleset_versions(created_at, actor, note, rules_json)
			VALUES (?, 'system', 'initial rule set', ?)`, 现在, string(快照))
		if 错误 != nil {
			return 0, fmt.Errorf("store: create initial version: %w", 错误)
		}
		最新版本, 错误 = 结果.LastInsertId()
		if 错误 != nil {
			return 0, fmt.Errorf("store: initial version id: %w", 错误)
		}
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return 0, fmt.Errorf("store: commit rule seed: %w", 错误)
	}
	return 最新版本, nil
}

// U保存规则集 在一个事务里替换全部规则、创建新版本并写审计。
func (存储 *U存储) U保存规则集(上下文 context.Context, 规则列表 []U规则行, 审计 U审计记录, 说明 string) (int64, error) {
	事务, 错误 := 存储.数据库.BeginTx(上下文, nil)
	if 错误 != nil {
		return 0, fmt.Errorf("store: begin rule save: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()
	现在 := time.Now().UnixMilli()
	if _, 错误 := 事务.ExecContext(上下文, `DELETE FROM rules`); 错误 != nil {
		return 0, fmt.Errorf("store: clear rules: %w", 错误)
	}
	语句, 错误 := 事务.PrepareContext(上下文, `INSERT INTO rules
		(id, name, category, pattern, enabled, score, source, defaults_version, position, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if 错误 != nil {
		return 0, fmt.Errorf("store: prepare rule insert: %w", 错误)
	}
	defer func() { _ = 语句.Close() }()
	for 序号, 行 := range 规则列表 {
		启用 := 0
		if 行.U启用 {
			启用 = 1
		}
		创建 := 行.U创建时间
		if 创建 == 0 {
			创建 = 现在
		}
		if _, 错误 := 语句.ExecContext(上下文, 行.UID, 行.U名称, 行.U类别, 行.U正则, 启用, 行.UScore,
			行.U来源, 行.U默认版本, 序号, 创建, 现在); 错误 != nil {
			return 0, fmt.Errorf("store: insert rule %s: %w", 行.UID, 错误)
		}
	}
	快照, 错误 := json.Marshal(规则快照(规则列表))
	if 错误 != nil {
		return 0, fmt.Errorf("store: encode rule snapshot: %w", 错误)
	}
	结果, 错误 := 事务.ExecContext(上下文, `INSERT INTO ruleset_versions(created_at, actor, note, rules_json)
		VALUES (?, ?, ?, ?)`, 现在, 审计.U操作者, 说明, string(快照))
	if 错误 != nil {
		return 0, fmt.Errorf("store: create rule version: %w", 错误)
	}
	版本号, 错误 := 结果.LastInsertId()
	if 错误 != nil {
		return 0, fmt.Errorf("store: rule version id: %w", 错误)
	}
	审计.U时间 = 现在
	if 审计.U后 == "" {
		审计.U后 = fmt.Sprintf(`{"ruleset_version":%d}`, 版本号)
	}
	if 错误 := 写审计事务(上下文, 事务, 审计); 错误 != nil {
		return 0, 错误
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return 0, fmt.Errorf("store: commit rule save: %w", 错误)
	}
	return 版本号, nil
}

func (存储 *U存储) U版本列表(上下文 context.Context) ([]U版本信息, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT id, created_at, actor, note, rules_json
		FROM ruleset_versions ORDER BY id DESC LIMIT 100`)
	if 错误 != nil {
		return nil, fmt.Errorf("store: read rule versions: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U版本信息, 0, 16)
	for 行列表.Next() {
		var 信息 U版本信息
		var 快照 string
		if 错误 := 行列表.Scan(&信息.UID, &信息.U创建时间, &信息.U操作者, &信息.U备注, &快照); 错误 != nil {
			return nil, fmt.Errorf("store: scan rule version: %w", 错误)
		}
		var 规则 []规则引擎.U规则
		if 错误 := json.Unmarshal([]byte(快照), &规则); 错误 == nil {
			信息.U规则数 = len(规则)
		}
		结果 = append(结果, 信息)
	}
	if 错误 := 行列表.Err(); 错误 != nil {
		return nil, fmt.Errorf("store: iterate rule versions: %w", 错误)
	}
	return 结果, nil
}

func (存储 *U存储) U版本详情(上下文 context.Context, 版本 int64) ([]U规则行, U版本信息, error) {
	var 信息 U版本信息
	var 快照 string
	错误 := 存储.数据库.QueryRowContext(上下文, `SELECT id, created_at, actor, note, rules_json
		FROM ruleset_versions WHERE id = ?`, 版本).Scan(&信息.UID, &信息.U创建时间, &信息.U操作者, &信息.U备注, &快照)
	if errors.Is(错误, sql.ErrNoRows) {
		return nil, U版本信息{}, U未找到错误{}
	}
	if 错误 != nil {
		return nil, U版本信息{}, fmt.Errorf("store: read rule version: %w", 错误)
	}
	var 规则列表 []规则引擎.U规则
	if 错误 := json.Unmarshal([]byte(快照), &规则列表); 错误 != nil {
		return nil, U版本信息{}, fmt.Errorf("store: decode rule version: %w", 错误)
	}
	行列表 := make([]U规则行, 0, len(规则列表))
	for 序号, 规则 := range 规则列表 {
		行列表 = append(行列表, U规则行{
			UID: 规则.UID, U名称: 规则.U名称, U类别: 规则.U类别, U正则: 规则.U正则,
			U启用: 规则.U启用, UScore: 规则.UScore, U来源: "custom",
			U默认版本: 规则引擎.U默认规则版本, U顺序: 序号,
			U创建时间: 信息.U创建时间, U更新时间: 信息.U创建时间,
		})
	}
	信息.U规则数 = len(行列表)
	return 行列表, 信息, nil
}

func 读取规则事务(上下文 context.Context, 事务 *sql.Tx) ([]U规则行, error) {
	行列表, 错误 := 事务.QueryContext(上下文, `SELECT id, name, category, pattern, enabled, score,
		source, defaults_version, position, created_at, updated_at FROM rules ORDER BY position, id`)
	if 错误 != nil {
		return nil, fmt.Errorf("store: read rules in transaction: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U规则行, 0, 24)
	for 行列表.Next() {
		var 行 U规则行
		var 启用 int
		if 错误 := 行列表.Scan(&行.UID, &行.U名称, &行.U类别, &行.U正则, &启用, &行.UScore,
			&行.U来源, &行.U默认版本, &行.U顺序, &行.U创建时间, &行.U更新时间); 错误 != nil {
			return nil, fmt.Errorf("store: scan rule in transaction: %w", 错误)
		}
		行.U启用 = 启用 == 1
		结果 = append(结果, 行)
	}
	return 结果, nil
}

func 规则快照(规则列表 []U规则行) []规则引擎.U规则 {
	结果 := make([]规则引擎.U规则, 0, len(规则列表))
	for _, 行 := range 规则列表 {
		结果 = append(结果, 行.U转为规则())
	}
	return 结果
}

// U种子配置 仅在对应表为空时写入 YAML 值；site_modes 逐行 INSERT OR IGNORE。
func (存储 *U存储) U种子配置(上下文 context.Context, 访问 访问列表.U列表, 限流 限流.U配置, 站点模式 map[string]string) error {
	事务, 错误 := 存储.数据库.BeginTx(上下文, nil)
	if 错误 != nil {
		return fmt.Errorf("store: begin config seed: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()
	现在 := time.Now().UnixMilli()

	var 设置数 int
	if 错误 := 事务.QueryRowContext(上下文, `SELECT COUNT(*) FROM access_settings`).Scan(&设置数); 错误 != nil {
		return fmt.Errorf("store: count access settings: %w", 错误)
	}
	if 设置数 == 0 {
		仅允许 := 0
		if 访问.U仅允许 {
			仅允许 = 1
		}
		if _, 错误 := 事务.ExecContext(上下文, `INSERT INTO access_settings(id, allow_only) VALUES (1, ?)`, 仅允许); 错误 != nil {
			return fmt.Errorf("store: seed access settings: %w", 错误)
		}
	}

	var 条目数 int
	if 错误 := 事务.QueryRowContext(上下文, `SELECT COUNT(*) FROM access_entries`).Scan(&条目数); 错误 != nil {
		return fmt.Errorf("store: count access entries: %w", 错误)
	}
	if 条目数 == 0 {
		for _, 条目 := range 访问.U条目 {
			启用 := 0
			if 条目.U启用 {
				启用 = 1
			}
			if _, 错误 := 事务.ExecContext(上下文, `INSERT INTO access_entries
				(id, action, address, note, enabled, source_log_id, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				条目.UID, 条目.U动作, 条目.U地址, 条目.U备注, 启用, 空整型(条目.U来源日志ID), 现在); 错误 != nil {
				return fmt.Errorf("store: seed access entry %s: %w", 条目.UID, 错误)
			}
		}
	}

	var 限流数 int
	if 错误 := 事务.QueryRowContext(上下文, `SELECT COUNT(*) FROM rate_limit_settings`).Scan(&限流数); 错误 != nil {
		return fmt.Errorf("store: count rate limit settings: %w", 错误)
	}
	if 限流数 == 0 {
		启用 := 0
		if 限流.U启用 {
			启用 = 1
		}
		if _, 错误 := 事务.ExecContext(上下文, `INSERT INTO rate_limit_settings
			(id, enabled, requests_per_minute, burst, updated_at) VALUES (1, ?, ?, ?, ?)`,
			启用, 限流.U每分钟, 限流.U突发, 现在); 错误 != nil {
			return fmt.Errorf("store: seed rate limit: %w", 错误)
		}
	}

	for 主机, 模式 := range 站点模式 {
		if _, 错误 := 事务.ExecContext(上下文, `INSERT OR IGNORE INTO site_modes(host, mode, updated_at)
			VALUES (?, ?, ?)`, 主机, 模式, 现在); 错误 != nil {
			return fmt.Errorf("store: seed site mode %s: %w", 主机, 错误)
		}
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return fmt.Errorf("store: commit config seed: %w", 错误)
	}
	return nil
}

func (存储 *U存储) U读取访问列表(上下文 context.Context) (访问列表.U列表, error) {
	var 仅允许 int
	错误 := 存储.数据库.QueryRowContext(上下文, `SELECT allow_only FROM access_settings WHERE id = 1`).Scan(&仅允许)
	if errors.Is(错误, sql.ErrNoRows) {
		仅允许 = 0
	} else if 错误 != nil {
		return 访问列表.U列表{}, fmt.Errorf("store: read access settings: %w", 错误)
	}
	结果 := 访问列表.U列表{U仅允许: 仅允许 == 1, U条目: []访问列表.U条目{}}
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT id, action, address, note, enabled, source_log_id, created_at
		FROM access_entries ORDER BY created_at, id`)
	if 错误 != nil {
		return 结果, fmt.Errorf("store: read access entries: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	for 行列表.Next() {
		var 条目 访问列表.U条目
		var 启用 int
		var 来源 sql.NullInt64
		var 创建 int64
		if 错误 := 行列表.Scan(&条目.UID, &条目.U动作, &条目.U地址, &条目.U备注, &启用, &来源, &创建); 错误 != nil {
			return 结果, fmt.Errorf("store: scan access entry: %w", 错误)
		}
		条目.U启用 = 启用 == 1
		if 来源.Valid {
			条目.U来源日志ID = 来源.Int64
		}
		条目.U创建时间 = 创建
		结果.U条目 = append(结果.U条目, 条目)
	}
	if 错误 := 行列表.Err(); 错误 != nil {
		return 结果, fmt.Errorf("store: iterate access entries: %w", 错误)
	}
	return 结果, nil
}

func (存储 *U存储) U新增访问条目(上下文 context.Context, 条目 访问列表.U条目, 审计 U审计记录) error {
	事务, 错误 := 存储.数据库.BeginTx(上下文, nil)
	if 错误 != nil {
		return fmt.Errorf("store: begin access insert: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()
	启用 := 0
	if 条目.U启用 {
		启用 = 1
	}
	if _, 错误 := 事务.ExecContext(上下文, `INSERT INTO access_entries
		(id, action, address, note, enabled, source_log_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		条目.UID, 条目.U动作, 条目.U地址, 条目.U备注, 启用, 空整型(条目.U来源日志ID), time.Now().UnixMilli()); 错误 != nil {
		return fmt.Errorf("store: insert access entry: %w", 错误)
	}
	if 错误 := 写审计事务(上下文, 事务, 审计); 错误 != nil {
		return 错误
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return fmt.Errorf("store: commit access insert: %w", 错误)
	}
	return nil
}

func (存储 *U存储) U删除访问条目(上下文 context.Context, 标识 string, 审计 U审计记录) error {
	事务, 错误 := 存储.数据库.BeginTx(上下文, nil)
	if 错误 != nil {
		return fmt.Errorf("store: begin access delete: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()
	结果, 错误 := 事务.ExecContext(上下文, `DELETE FROM access_entries WHERE id = ?`, 标识)
	if 错误 != nil {
		return fmt.Errorf("store: delete access entry: %w", 错误)
	}
	数量, 错误 := 结果.RowsAffected()
	if 错误 != nil {
		return fmt.Errorf("store: count deleted access entries: %w", 错误)
	}
	if 数量 == 0 {
		return U未找到错误{}
	}
	if 错误 := 写审计事务(上下文, 事务, 审计); 错误 != nil {
		return 错误
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return fmt.Errorf("store: commit access delete: %w", 错误)
	}
	return nil
}

func (存储 *U存储) U读取限流(上下文 context.Context) (限流.U配置, error) {
	var 配置 限流.U配置
	var 启用 int
	错误 := 存储.数据库.QueryRowContext(上下文, `SELECT enabled, requests_per_minute, burst
		FROM rate_limit_settings WHERE id = 1`).Scan(&启用, &配置.U每分钟, &配置.U突发)
	if errors.Is(错误, sql.ErrNoRows) {
		return 限流.U配置{
			U启用:  false,
			U每分钟: 限流.U默认每分钟请求,
			U突发:  限流.U默认突发,
		}, nil
	}
	if 错误 != nil {
		return 配置, fmt.Errorf("store: read rate limit: %w", 错误)
	}
	配置.U启用 = 启用 == 1
	return 配置, nil
}

func (存储 *U存储) U保存限流(上下文 context.Context, 配置 限流.U配置, 审计 U审计记录) error {
	事务, 错误 := 存储.数据库.BeginTx(上下文, nil)
	if 错误 != nil {
		return fmt.Errorf("store: begin rate limit save: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()
	启用 := 0
	if 配置.U启用 {
		启用 = 1
	}
	if _, 错误 := 事务.ExecContext(上下文, `INSERT INTO rate_limit_settings
		(id, enabled, requests_per_minute, burst, updated_at) VALUES (1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET enabled = excluded.enabled,
			requests_per_minute = excluded.requests_per_minute,
			burst = excluded.burst, updated_at = excluded.updated_at`,
		启用, 配置.U每分钟, 配置.U突发, time.Now().UnixMilli()); 错误 != nil {
		return fmt.Errorf("store: save rate limit: %w", 错误)
	}
	if 错误 := 写审计事务(上下文, 事务, 审计); 错误 != nil {
		return 错误
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return fmt.Errorf("store: commit rate limit save: %w", 错误)
	}
	return nil
}

func (存储 *U存储) U读取站点模式(上下文 context.Context) (map[string]string, error) {
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT host, mode FROM site_modes`)
	if 错误 != nil {
		return nil, fmt.Errorf("store: read site modes: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make(map[string]string)
	for 行列表.Next() {
		var 主机, 模式 string
		if 错误 := 行列表.Scan(&主机, &模式); 错误 != nil {
			return nil, fmt.Errorf("store: scan site mode: %w", 错误)
		}
		结果[主机] = 模式
	}
	if 错误 := 行列表.Err(); 错误 != nil {
		return nil, fmt.Errorf("store: iterate site modes: %w", 错误)
	}
	return 结果, nil
}

func (存储 *U存储) U保存站点模式(上下文 context.Context, 主机, 模式 string, 审计 U审计记录) error {
	事务, 错误 := 存储.数据库.BeginTx(上下文, nil)
	if 错误 != nil {
		return fmt.Errorf("store: begin site mode save: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()
	if _, 错误 := 事务.ExecContext(上下文, `INSERT INTO site_modes(host, mode, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(host) DO UPDATE SET mode = excluded.mode, updated_at = excluded.updated_at`,
		主机, 模式, time.Now().UnixMilli()); 错误 != nil {
		return fmt.Errorf("store: save site mode: %w", 错误)
	}
	if 错误 := 写审计事务(上下文, 事务, 审计); 错误 != nil {
		return 错误
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return fmt.Errorf("store: commit site mode save: %w", 错误)
	}
	return nil
}

func 写审计事务(上下文 context.Context, 事务 *sql.Tx, 记录 U审计记录) error {
	时间 := 记录.U时间
	if 时间 == 0 {
		时间 = time.Now().UnixMilli()
	}
	if _, 错误 := 事务.ExecContext(上下文, `INSERT INTO audit_log(ts, actor, action, before, after, client_ip)
		VALUES (?, ?, ?, ?, ?, ?)`, 时间, 记录.U操作者, 记录.U动作, 记录.U前, 记录.U后, 记录.U客户端IP); 错误 != nil {
		return fmt.Errorf("store: write audit row: %w", 错误)
	}
	return nil
}

func 空整型(值 int64) any {
	if 值 == 0 {
		return nil
	}
	return 值
}

func (存储 *U存储) U最新规则版本(上下文 context.Context) (int64, error) {
	var 版本 int64
	if 错误 := 存储.数据库.QueryRowContext(上下文, `SELECT COALESCE(MAX(id), 0) FROM ruleset_versions`).Scan(&版本); 错误 != nil {
		return 0, fmt.Errorf("store: latest rule version: %w", 错误)
	}
	return 版本, nil
}
