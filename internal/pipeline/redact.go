package 检查管线

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// 持久化副本只需足够定位问题，长度上限同时限制泄漏面。
	U存储体上限 = 2 << 10
	U匹配列上限 = 512
)

// 键名匹配来自需求，未加锚点，以便覆盖 access_token、card_number 等变体。
var 敏感名称 = regexp.MustCompile(`(?i)(pass(word)?|secret|token|authorization|card|ssn)`)

var 敏感头 = map[string]struct{}{
	"authorization": {},
	"cookie":        {},
	"set-cookie":    {},
	"x-api-key":     {},
}

// U敏感名称 报告名称是否应按敏感字段处理。
func U敏感名称(名称 string) bool {
	return 敏感名称.MatchString(名称)
}

// U脱敏头 返回副本；原请求头不受影响，因为转发必须使用原始值。
func U脱敏头(头 http.Header) http.Header {
	副本 := 头.Clone()
	if 副本 == nil {
		副本 = http.Header{}
	}
	for 名称 := range 副本 {
		if _, 敏感 := 敏感头[strings.ToLower(名称)]; 敏感 {
			值 := 副本[名称]
			for 下标 := range 值 {
				值[下标] = "<redacted>"
			}
		}
	}
	return 副本
}

// U脱敏查询 对持久化查询串按键名替换值。无法解析时宁可不存内容。
func U脱敏查询(原始 string) string {
	if 原始 == "" {
		return ""
	}
	值, 错误 := url.ParseQuery(原始)
	if 错误 != nil {
		return "<redacted>"
	}
	for 键 := range 值 {
		if !U敏感名称(键) {
			continue
		}
		列表 := 值[键]
		for 下标 := range 列表 {
			列表[下标] = "<redacted>"
		}
	}
	return 还原红acted(值.Encode())
}

// U脱敏体 生成用于落库的请求体副本，上限 2 KiB。
func U脱敏体(内容类型 string, 体 []byte) []byte {
	if len(体) == 0 {
		return nil
	}
	媒体, _, _ := mime.ParseMediaType(内容类型)
	switch {
	case 媒体 == "application/json" || strings.HasSuffix(媒体, "+json"):
		return 脱敏JSON(体)
	case 媒体 == "application/x-www-form-urlencoded":
		return 脱敏表单(体)
	default:
		return []byte(U截断文本(string(体), U存储体上限))
	}
}

// U截断文本 截到字节上限并保持 UTF-8 合法，避免 SQLite 文本写入失败。
func U截断文本(值 string, 上限 int) string {
	if 上限 <= 0 || len(值) <= 上限 {
		return 值
	}
	截断 := 值[:上限]
	for len(截断) > 0 && !utf8.ValidString(截断) {
		截断 = 截断[:len(截断)-1]
	}
	return 截断
}

func 脱敏JSON(体 []byte) []byte {
	解码器 := json.NewDecoder(bytes.NewReader(体))
	解码器.UseNumber()
	var 值 any
	if 错误 := 解码器.Decode(&值); 错误 != nil {
		return []byte("<body redacted: invalid JSON>")
	}
	清理 := 脱敏JSON值(值, 0)
	var 输出 bytes.Buffer
	编码器 := json.NewEncoder(&输出)
	编码器.SetEscapeHTML(false)
	if 错误 := 编码器.Encode(清理); 错误 != nil {
		return []byte("<body redacted: invalid JSON>")
	}
	return []byte(U截断文本(strings.TrimSuffix(输出.String(), "\n"), U存储体上限))
}

func 脱敏JSON值(值 any, 深度 int) any {
	if 深度 > 32 {
		return "<redacted: nested too deep>"
	}
	switch 值 := 值.(type) {
	case map[string]any:
		for 键, 子值 := range 值 {
			if U敏感名称(键) {
				值[键] = "<redacted>"
				continue
			}
			值[键] = 脱敏JSON值(子值, 深度+1)
		}
		return 值
	case []any:
		for 下标, 子值 := range 值 {
			值[下标] = 脱敏JSON值(子值, 深度+1)
		}
		return 值
	default:
		return 值
	}
}

func 脱敏表单(体 []byte) []byte {
	值, 错误 := url.ParseQuery(string(体))
	if 错误 != nil {
		return []byte("<body redacted: invalid form>")
	}
	for 键 := range 值 {
		if !U敏感名称(键) {
			continue
		}
		列表 := 值[键]
		for 下标 := range 列表 {
			列表[下标] = "<redacted>"
		}
	}
	return []byte(U截断文本(还原红acted(值.Encode()), U存储体上限))
}

func 还原红acted(编码 string) string {
	return strings.ReplaceAll(编码, "%3Credacted%3E", "<redacted>")
}
