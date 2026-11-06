package 存储

import (
	"context"
	"fmt"
	"time"
)

// U审计记录 记录管理面变更与登录事件。before/after 为 JSON 字符串。
type U审计记录 struct {
	U时间    int64
	U操作者   string
	U动作    string
	U前     string
	U后     string
	U客户端IP string
}

// U写审计 独立写入审计行；需要与业务变更原子提交的调用方应使用事务版本。
func (存储 *U存储) U写审计(上下文 context.Context, 记录 U审计记录) error {
	时间 := 记录.U时间
	if 时间 == 0 {
		时间 = time.Now().UnixMilli()
	}
	_, 错误 := 存储.数据库.ExecContext(上下文, `INSERT INTO audit_log(ts, actor, action, before, after, client_ip)
		VALUES (?, ?, ?, ?, ?, ?)`, 时间, 记录.U操作者, 记录.U动作, 记录.U前, 记录.U后, 记录.U客户端IP)
	if 错误 != nil {
		return fmt.Errorf("store: write audit row: %w", 错误)
	}
	return nil
}
