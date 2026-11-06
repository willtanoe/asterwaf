package 管理接口

import (
	"testing"
	"time"
)

func Test会话绝对过期(t *testing.T) {
	当前 := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	管理 := 新建会话(func() time.Time { return 当前 }, 0)
	defer 管理.Close()

	令牌, 到期, 错误 := 管理.U创建("admin")
	if 错误 != nil {
		t.Fatalf("create: %v", 错误)
	}
	if 令牌 == "" || 到期.Sub(当前) != 会话有效期 {
		t.Fatalf("token=%q expiry=%v", 令牌, 到期)
	}
	if _, 可以 := 管理.U获取(令牌); !可以 {
		t.Fatal("new session not found")
	}
	当前 = 当前.Add(11 * time.Hour)
	if _, 可以 := 管理.U获取(令牌); !可以 {
		t.Fatal("session expired too early")
	}
	当前 = 当前.Add(2 * time.Hour)
	if _, 可以 := 管理.U获取(令牌); 可以 {
		t.Fatal("session survived absolute expiry")
	}

	令牌2, _, _ := 管理.U创建("admin")
	管理.U删除(令牌2)
	if _, 可以 := 管理.U获取(令牌2); 可以 {
		t.Fatal("deleted session still valid")
	}
}

func Test会话令牌唯一(t *testing.T) {
	管理 := 新建会话(time.Now, 0)
	defer 管理.Close()
	甲, _, _ := 管理.U创建("admin")
	乙, _, _ := 管理.U创建("admin")
	if 甲 == 乙 {
		t.Fatal("session tokens repeat")
	}
}
