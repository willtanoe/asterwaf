package 存储

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// U日志条目 同时用于列表和详情；列表不会填充请求头/体。
type U日志条目 struct {
	UID    int64  `json:"id"`
	U时间    int64  `json:"ts"`
	U请求ID  string `json:"request_id"`
	USite  string `json:"site"`
	U方法    string `json:"method"`
	U路径    string `json:"path"`
	U查询    string `json:"query"`
	U状态    int    `json:"status"`
	U动作    string `json:"action"`
	U类别    string `json:"category"`
	U规则ID  string `json:"rule_id"`
	U影子    bool   `json:"shadow"`
	U客户端IP string `json:"client_ip"`
	U用户代理  string `json:"user_agent"`
	U耗时毫秒  int64  `json:"duration_ms"`
	U字节入   int64  `json:"bytes_in"`
	U字节出   int64  `json:"bytes_out"`
	U请求头   string `json:"request_headers,omitempty"`
	U请求体   string `json:"request_body,omitempty"`
	U匹配    string `json:"matched,omitempty"`
	U种子    bool   `json:"seed"`
}

// U日志查询 的零值表示不限制；游标使用 (ts,id) 保证稳定翻页。
type U日志查询 struct {
	U从    int64
	U到    int64
	USite string
	U动作   string
	U类别   string
	U规则ID string
	U地址   string
	U路径   string
	U状态   int
	U游标时间 int64
	U游标ID int64
	U限制   int
}

// 日志查询条件 组装过滤 WHERE 与参数，供列表与摘要共用。
func 日志查询条件(查询 U日志查询) (string, []any) {
	条件 := `WHERE (? = 0 OR ts >= ?) AND (? = 0 OR ts < ?)
		AND (? = '' OR site = ?) AND (? = '' OR action = ?) AND (? = '' OR category = ?)
		AND (? = '' OR rule_id = ?) AND (? = '' OR client_ip = ?) AND (? = '' OR path LIKE ?)
		AND (? = 0 OR status = ?)`
	参数 := []any{
		查询.U从, 查询.U从, 查询.U到, 查询.U到,
		查询.USite, 查询.USite, 查询.U动作, 查询.U动作, 查询.U类别, 查询.U类别,
		查询.U规则ID, 查询.U规则ID, 查询.U地址, 查询.U地址, 查询.U路径, "%" + 查询.U路径 + "%",
		查询.U状态, 查询.U状态,
	}
	return 条件, 参数
}

func (存储 *U存储) U查询日志(上下文 context.Context, 查询 U日志查询) ([]U日志条目, error) {
	限制 := 查询.U限制
	if 限制 <= 0 {
		限制 = 50
	}
	if 限制 > 200 {
		限制 = 200
	}
	条件, 参数 := 日志查询条件(查询)
	if 查询.U游标时间 != 0 {
		条件 += ` AND (ts < ? OR (ts = ? AND id < ?))`
		参数 = append(参数, 查询.U游标时间, 查询.U游标时间, 查询.U游标ID)
	}
	参数 = append(参数, 限制+1)
	行列表, 错误 := 存储.数据库.QueryContext(上下文, `SELECT id, ts, request_id, site, method, path, query,
		status, action, category, rule_id, shadow, client_ip, user_agent, duration_ms, seed, bytes_in, bytes_out
		FROM logs `+条件+` ORDER BY ts DESC, id DESC LIMIT ?`, 参数...)
	if 错误 != nil {
		return nil, fmt.Errorf("store: query logs: %w", 错误)
	}
	defer func() { _ = 行列表.Close() }()
	结果 := make([]U日志条目, 0, 限制+1)
	for 行列表.Next() {
		var 条目 U日志条目
		var 影子, 种子 int
		if 错误 := 行列表.Scan(&条目.UID, &条目.U时间, &条目.U请求ID, &条目.USite, &条目.U方法,
			&条目.U路径, &条目.U查询, &条目.U状态, &条目.U动作, &条目.U类别, &条目.U规则ID,
			&影子, &条目.U客户端IP, &条目.U用户代理, &条目.U耗时毫秒, &种子, &条目.U字节入, &条目.U字节出); 错误 != nil {
			return nil, fmt.Errorf("store: scan log: %w", 错误)
		}
		条目.U影子 = 影子 == 1
		条目.U种子 = 种子 == 1
		结果 = append(结果, 条目)
	}
	if 错误 := 行列表.Err(); 错误 != nil {
		return nil, fmt.Errorf("store: iterate logs: %w", 错误)
	}
	return 结果, nil
}

func (存储 *U存储) U日志详情(上下文 context.Context, 标识 int64) (U日志条目, error) {
	var 条目 U日志条目
	var 影子, 种子 int
	错误 := 存储.数据库.QueryRowContext(上下文, `SELECT id, ts, request_id, site, method, path, query,
		status, action, category, rule_id, shadow, client_ip, user_agent, duration_ms,
		request_headers, request_body, matched, seed, bytes_in, bytes_out FROM logs WHERE id = ?`, 标识).Scan(
		&条目.UID, &条目.U时间, &条目.U请求ID, &条目.USite, &条目.U方法, &条目.U路径, &条目.U查询,
		&条目.U状态, &条目.U动作, &条目.U类别, &条目.U规则ID, &影子, &条目.U客户端IP,
		&条目.U用户代理, &条目.U耗时毫秒, &条目.U请求头, &条目.U请求体, &条目.U匹配, &种子,
		&条目.U字节入, &条目.U字节出,
	)
	if errors.Is(错误, sql.ErrNoRows) {
		return 条目, U未找到错误{}
	}
	if 错误 != nil {
		return 条目, fmt.Errorf("store: read log detail: %w", 错误)
	}
	条目.U影子 = 影子 == 1
	条目.U种子 = 种子 == 1
	return 条目, nil
}
