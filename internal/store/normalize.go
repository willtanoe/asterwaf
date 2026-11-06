package 存储

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const 路径长度上限 = 200

var (
	UUID段 = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	长Hex段 = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
	数字段   = regexp.MustCompile(`^[0-9]{6,}$`)
)

// 规范化路径 把高基数路径段折叠为 :id，让 Top paths 不被 id 淹没。
func 规范化路径(原始 string) string {
	路径 := 原始
	if 位置 := strings.IndexByte(路径, '?'); 位置 >= 0 {
		路径 = 路径[:位置]
	}
	if 位置 := strings.IndexByte(路径, '#'); 位置 >= 0 {
		路径 = 路径[:位置]
	}
	if 路径 == "" {
		return "/"
	}
	段列表 := strings.Split(路径, "/")
	for 序号, 段 := range 段列表 {
		基础 := 段
		if 点 := strings.IndexByte(段, '.'); 点 > 0 {
			基础 = 段[:点]
		}
		if UUID段.MatchString(段) || 长Hex段.MatchString(基础) || 数字段.MatchString(基础) {
			段列表[序号] = ":id"
		}
	}
	结果 := strings.Join(段列表, "/")
	if 结果 == "" {
		return "/"
	}
	return 截断UTF8(结果, 路径长度上限)
}

func 截断UTF8(值 string, 上限 int) string {
	if len(值) <= 上限 {
		return 值
	}
	截 := 值[:上限]
	for len(截) > 0 && !utf8.ValidString(截) {
		截 = 截[:len(截)-1]
	}
	return 截
}
