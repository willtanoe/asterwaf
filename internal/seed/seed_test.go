package 合成流量

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func Test生成确定性(t *testing.T) {
	参数 := U参数{
		U结束时间:  time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		U天数:    2,
		U站点:    []string{"app.example.com", "api.example.com"},
		U每小时系数: 1,
	}
	第一次 := U生成(参数)
	第二次 := U生成(参数)
	if !reflect.DeepEqual(第一次, 第二次) {
		t.Fatal("generator is not deterministic")
	}
	if len(第一次) == 0 {
		t.Fatal("no records generated")
	}
}

func Test生成内容分布(t *testing.T) {
	参数 := U参数{
		U结束时间:  time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		U天数:    3,
		U站点:    []string{"app.example.com", "api.example.com"},
		U每小时系数: 2,
	}
	记录列表 := U生成(参数)
	动作 := map[string]int{}
	站点 := map[string]int{}
	见过V6 := false
	有过查询 := false
	for _, 记录 := range 记录列表 {
		if !记录.U种子 {
			t.Fatal("record is not marked as seed")
		}
		if 记录.U请求ID == "" || 记录.U客户端IP == "" {
			t.Fatalf("record missing identity: %+v", 记录)
		}
		动作[记录.U动作]++
		站点[记录.USite]++
		if strings.Contains(记录.U客户端IP, ":") {
			见过V6 = true
		}
		if strings.Contains(记录.U路径, "?") {
			t.Fatalf("path contains query: %s", 记录.U路径)
		}
		if 记录.U查询 != "" {
			有过查询 = true
		}
		开始 := 参数.U结束时间.Truncate(24 * time.Hour).Add(-72 * time.Hour)
		if 记录.U时间 < 开始.UnixMilli() || 记录.U时间 > 参数.U结束时间.UnixMilli() {
			t.Fatalf("record outside window: %d", 记录.U时间)
		}
	}
	for _, 名称 := range []string{"allow", "block", "shadow", "ratelimited"} {
		if 动作[名称] == 0 {
			t.Fatalf("action %s missing from generated traffic: %+v", 名称, 动作)
		}
	}
	if len(站点) != 2 {
		t.Fatalf("site distribution=%+v", 站点)
	}
	if !见过V6 {
		t.Fatal("no IPv6 client generated")
	}
	if !有过查询 {
		t.Fatal("no query generated")
	}
}

func Test日内曲线(t *testing.T) {
	记录列表 := U生成(U参数{
		U结束时间:  time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC),
		U天数:    2,
		U站点:    []string{"app.example.com"},
		U每小时系数: 1,
	})
	每小时 := map[int]int{}
	for _, 记录 := range 记录列表 {
		时刻 := time.UnixMilli(记录.U时间).UTC()
		每小时[时刻.Hour()]++
	}
	if 每小时[14] <= 每小时[3] {
		t.Fatalf("day curve missing: hour14=%d hour3=%d", 每小时[14], 每小时[3])
	}
}
