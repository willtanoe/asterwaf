package 管理接口

import (
	"testing"
	"time"
)

func Test按IP限速(t *testing.T) {
	当前 := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	限制 := 新建登录限制(func() time.Time { return 当前 }, 0)
	defer 限制.Close()

	for 次数 := 0; 次数 < 每IP上限; 次数++ {
		限制.U记录失败("203.0.113.7", "admin")
	}
	允许, 等待 := 限制.U检查("203.0.113.7", "admin")
	if 允许 {
		t.Fatal("IP limit not enforced")
	}
	if 等待 <= 0 || 等待 > 登录窗口 {
		t.Fatalf("retry wait=%v", 等待)
	}
	当前 = 当前.Add(登录窗口 + time.Second)
	if 允许, _ := 限制.U检查("203.0.113.7", "admin"); !允许 {
		t.Fatal("window did not reset")
	}
}

func Test按用户名限速(t *testing.T) {
	当前 := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	限制 := 新建登录限制(func() time.Time { return 当前 }, 0)
	defer 限制.Close()

	for 次数 := 0; 次数 < 每用户上限; 次数++ {
		限制.U记录失败("203.0.113."+string(rune('1'+次数)), "Admin")
	}
	允许, _ := 限制.U检查("198.51.100.1", "admin")
	if 允许 {
		t.Fatal("username limit not enforced across IPs")
	}
	限制.U清除("198.51.100.1", "admin")
	if 允许, _ := 限制.U检查("198.51.100.1", "admin"); !允许 {
		t.Fatal("successful login did not clear username limit")
	}
}

func Test登录限制清理(t *testing.T) {
	当前 := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	限制 := 新建登录限制(func() time.Time { return 当前 }, 0)
	defer 限制.Close()
	限制.U记录失败("203.0.113.7", "admin")
	限制.清理(当前.Add(登录窗口 + time.Second))
	限制.互斥.Lock()
	数量 := len(限制.记录)
	限制.互斥.Unlock()
	if 数量 != 0 {
		t.Fatalf("stale entries=%d", 数量)
	}
}
