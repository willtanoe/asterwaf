package 限流

import (
	"net/netip"
	"sync"
	"testing"
	"time"
)

func Test并发修改配置不产生竞态(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 600, U突发: 100}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()

	var 组 sync.WaitGroup
	组.Add(2)
	go func() {
		defer 组.Done()
		for 次数 := 0; 次数 < 200; 次数++ {
			_ = 限流器.U修改配置(U配置{U启用: 次数%2 == 0, U每分钟: 600, U突发: 100})
		}
	}()
	go func() {
		defer 组.Done()
		for 次数 := 0; 次数 < 200; 次数++ {
			_, _ = 限流器.U允许(netip.MustParseAddr("203.0.113.9"))
			_ = 限流器.U启用()
		}
	}()
	组.Wait()
}
