package 限流

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func Test令牌桶突发与补充(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 60, U突发: 2}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()
	地址 := netip.MustParseAddr("203.0.113.1")

	for 次数 := 0; 次数 < 2; 次数++ {
		if 放行, _ := 限流器.U允许(地址); !放行 {
			t.Fatalf("burst request %d denied", 次数)
		}
	}
	放行, 等待 := 限流器.U允许(地址)
	if 放行 {
		t.Fatal("request beyond burst was allowed")
	}
	if 等待 != time.Second {
		t.Fatalf("retry after=%v, want 1s", 等待)
	}

	当前 = 当前.Add(500 * time.Millisecond)
	放行, 等待 = 限流器.U允许(地址)
	if 放行 || 等待 != time.Second {
		t.Fatalf("half refill allowed=%v wait=%v", 放行, 等待)
	}

	当前 = 当前.Add(500 * time.Millisecond)
	if 放行, _ := 限流器.U允许(地址); !放行 {
		t.Fatal("full refill was denied")
	}
}

func Test补充不超过突发容量(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 600, U突发: 2}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()
	地址 := netip.MustParseAddr("203.0.113.2")
	当前 = 当前.Add(time.Hour)
	for 次数 := 0; 次数 < 2; 次数++ {
		if 放行, _ := 限流器.U允许(地址); !放行 {
			t.Fatalf("request %d denied after long idle", 次数)
		}
	}
	if 放行, _ := 限流器.U允许(地址); 放行 {
		t.Fatal("bucket exceeded burst capacity")
	}
}

func TestIPv6按64聚合(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 60, U突发: 1}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()

	if 放行, _ := 限流器.U允许(netip.MustParseAddr("2001:db8::1")); !放行 {
		t.Fatal("first address denied")
	}
	if 放行, _ := 限流器.U允许(netip.MustParseAddr("2001:db8::2")); 放行 {
		t.Fatal("second address in same /64 was not limited")
	}
	if 放行, _ := 限流器.U允许(netip.MustParseAddr("2001:db8:0:1::1")); !放行 {
		t.Fatal("different /64 was limited")
	}
}

func TestIPv4独立计桶(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 60, U突发: 1}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()
	if 放行, _ := 限流器.U允许(netip.MustParseAddr("203.0.113.1")); !放行 {
		t.Fatal("first address denied")
	}
	if 放行, _ := 限流器.U允许(netip.MustParseAddr("203.0.113.2")); !放行 {
		t.Fatal("second address shared bucket")
	}
	if 放行, _ := 限流器.U允许(netip.MustParseAddr("203.0.113.1")); 放行 {
		t.Fatal("same address not limited")
	}
}

func Test空闲清理(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 60, U突发: 5}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()
	_, _ = 限流器.U允许(netip.MustParseAddr("203.0.113.9"))
	if 条目数(限流器) != 1 {
		t.Fatalf("entries=%d", 条目数(限流器))
	}
	限流器.清理(当前.Add(9 * time.Minute))
	if 条目数(限流器) != 1 {
		t.Fatal("entry expired before idle TTL")
	}
	限流器.清理(当前.Add(11 * time.Minute))
	if 条目数(限流器) != 0 {
		t.Fatal("idle entry was not cleaned up")
	}
}

func Test上限淘汰最旧(t *testing.T) {
	原上限 := 最大条目数
	最大条目数 = 3
	defer func() { 最大条目数 = 原上限 }()

	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 60, U突发: 5}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()

	地址 := []netip.Addr{
		netip.MustParseAddr("203.0.113.1"),
		netip.MustParseAddr("203.0.113.2"),
		netip.MustParseAddr("203.0.113.3"),
		netip.MustParseAddr("203.0.113.4"),
	}
	for _, 值 := range 地址 {
		if 放行, _ := 限流器.U允许(值); !放行 {
			t.Fatalf("address %s denied", 值)
		}
	}
	if 条目数(限流器) != 3 {
		t.Fatalf("entries=%d, want 3", 条目数(限流器))
	}
	限流器.互斥.Lock()
	_, 最旧存在 := 限流器.桶表[netip.PrefixFrom(地址[0], 32)]
	_, 最新存在 := 限流器.桶表[netip.PrefixFrom(地址[3], 32)]
	限流器.互斥.Unlock()
	if 最旧存在 {
		t.Fatal("oldest entry was not evicted")
	}
	if !最新存在 {
		t.Fatal("newest entry is missing")
	}
}

func Test停用与配置修改(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 60, U突发: 1}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()
	地址 := netip.MustParseAddr("203.0.113.5")
	if 放行, _ := 限流器.U允许(地址); !放行 {
		t.Fatal("first request denied")
	}
	if 放行, _ := 限流器.U允许(地址); 放行 {
		t.Fatal("second request allowed")
	}
	if 错误 := 限流器.U修改配置(U配置{U启用: false, U每分钟: 60, U突发: 1}); 错误 != nil {
		t.Fatalf("modify: %v", 错误)
	}
	if 放行, _ := 限流器.U允许(地址); !放行 {
		t.Fatal("disabled limiter denied a request")
	}
	if 错误 := 限流器.U修改配置(U配置{U启用: true, U每分钟: 0, U突发: 0}); 错误 == nil {
		t.Fatal("expected invalid config rejection")
	}
}

func Test非法配置(t *testing.T) {
	if _, 错误 := 新建(U配置{U启用: true, U每分钟: 0, U突发: 0}, time.Now, 0); 错误 == nil {
		t.Fatal("expected invalid config error")
	}
	限流器, 错误 := 新建(U配置{}, time.Now, 0)
	if 错误 != nil {
		t.Fatalf("disabled zero config rejected: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()
	if 放行, _ := 限流器.U允许(netip.MustParseAddr("203.0.113.1")); !放行 {
		t.Fatal("disabled limiter denied")
	}
}

func Test并发允许(t *testing.T) {
	当前 := time.Unix(0, 0)
	限流器, 错误 := 新建(U配置{U启用: true, U每分钟: 6000, U突发: 1000}, func() time.Time { return 当前 }, 0)
	if 错误 != nil {
		t.Fatalf("new: %v", 错误)
	}
	defer func() { _ = 限流器.Close() }()

	var 组 sync.WaitGroup
	for 序号 := 0; 序号 < 4; 序号++ {
		组.Add(1)
		go func() {
			defer 组.Done()
			for 次数 := 0; 次数 < 100; 次数++ {
				_, _ = 限流器.U允许(netip.MustParseAddr("203.0.113.50"))
			}
		}()
	}
	组.Wait()
}

func Test配置序列化往返(t *testing.T) {
	原始 := U配置{U启用: true, U每分钟: 300, U突发: 50}
	YAML, 错误 := yaml.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("yaml marshal: %v", 错误)
	}
	var YAML还原 U配置
	if 错误 := yaml.Unmarshal(YAML, &YAML还原); 错误 != nil {
		t.Fatalf("yaml unmarshal: %v", 错误)
	}
	if !reflect.DeepEqual(原始, YAML还原) {
		t.Fatalf("yaml round trip mismatch: %+v", YAML还原)
	}

	JSON, 错误 := json.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("json marshal: %v", 错误)
	}
	var JSON还原 U配置
	if 错误 := json.Unmarshal(JSON, &JSON还原); 错误 != nil {
		t.Fatalf("json unmarshal: %v", 错误)
	}
	if !reflect.DeepEqual(原始, JSON还原) {
		t.Fatalf("json round trip mismatch: %+v", JSON还原)
	}
}

func 条目数(限流器 *U限流器) int {
	限流器.互斥.Lock()
	defer 限流器.互斥.Unlock()
	return len(限流器.桶表)
}
