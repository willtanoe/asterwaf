package 检查管线

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

const (
	// 解压后的检查副本单独设限，避免压缩炸弹把内存放大到不可控。
	最大解压体积   int64 = 4 << 20
	最大JSON深度       = 8
	最大文本段    int64 = 64 << 10
	最大分段数          = 1000
)

// U目标 是规则匹配的输入单元，类别与名称只用于日志和测试定位。
type U目标 struct {
	U类别 string `json:"category"`
	U名称 string `json:"name"`
	U值  string `json:"value"`
}

// U快照 保留一次请求的原文与检查视图。U体 永远是原始字节。
type U快照 struct {
	U方法     string
	U原始路径   string
	U解码路径   string
	U查询     []U目标
	U头      []U目标
	U表单     []U目标
	U多部分    []U目标
	UJSON   []U目标
	UXML    []U目标
	UCookie []U目标
	U跳过     []string
	U体      []byte
	U检查体    []byte
	U内容类型   string
	U内容编码   string
}

// U体过大错误 让调用方把请求映射为 413，而不是通用的读取失败。
type U体过大错误 struct {
	U限制 int64
}

func (错误 *U体过大错误) Error() string {
	return fmt.Sprintf("request body exceeds %d bytes", 错误.U限制)
}

// U管线 保存需要检查的请求头名称。站点之间的差异只有请求体上限。
type U管线 struct {
	头名 []string
}

// U新建 构造管线。头名去重并保持配置顺序，便于日志重现。
func U新建(头名 []string) *U管线 {
	已见 := make(map[string]struct{}, len(头名))
	结果 := make([]string, 0, len(头名))
	for _, 名称 := range 头名 {
		标准 := http.CanonicalHeaderKey(strings.TrimSpace(名称))
		if 标准 == "" {
			continue
		}
		if _, 重复 := 已见[标准]; 重复 {
			continue
		}
		已见[标准] = struct{}{}
		结果 = append(结果, 标准)
	}
	return &U管线{头名: 结果}
}

// U读取 读取有界请求体并提取检查目标。读取前先看 Content-Length，
// 再用 MaxBytesReader 兜住 chunked 请求。
func (管线 *U管线) U读取(写 http.ResponseWriter, 请求 *http.Request, 最大 int64) (U快照, error) {
	媒体, 参数, 媒体错误 := mime.ParseMediaType(请求.Header.Get("Content-Type"))
	快照 := U快照{
		U方法:     请求.Method,
		U原始路径:   请求.URL.EscapedPath(),
		U内容类型:   strings.ToLower(媒体),
		U内容编码:   strings.ToLower(strings.TrimSpace(请求.Header.Get("Content-Encoding"))),
		U查询:     []U目标{},
		U头:      []U目标{},
		U表单:     []U目标{},
		U多部分:    []U目标{},
		UJSON:   []U目标{},
		UXML:    []U目标{},
		UCookie: []U目标{},
		U跳过:     []string{},
	}
	if 最大 <= 0 {
		return 快照, errors.New("inspection: body limit must be positive")
	}
	if 请求.ContentLength > 最大 {
		return 快照, &U体过大错误{U限制: 最大}
	}

	var 原体 []byte
	if 请求.Body != nil && 请求.Body != http.NoBody {
		有限 := http.MaxBytesReader(写, 请求.Body, 最大)
		体, 错误 := io.ReadAll(有限)
		if 错误 != nil {
			var 超限 *http.MaxBytesError
			if errors.As(错误, &超限) {
				return 快照, &U体过大错误{U限制: 最大}
			}
			return 快照, fmt.Errorf("inspection: read request body: %w", 错误)
		}
		原体 = 体
	}
	快照.U体 = 原体

	解码路径, 错误 := url.PathUnescape(快照.U原始路径)
	if 错误 != nil {
		解码路径 = 快照.U原始路径
		快照.U跳过 = append(快照.U跳过, "invalid path escape")
	}
	快照.U解码路径 = 解码路径

	快照.U查询 = 提取查询(请求.URL.RawQuery, &快照)
	快照.U头 = 提取头(请求.Header, 管线.头名)
	for _, 饼干 := range 请求.Cookies() {
		快照.UCookie = append(快照.UCookie, U目标{
			U类别: "cookie", U名称: 饼干.Name, U值: 饼干.Value,
		})
	}

	检查体 := 解压体(快照.U内容编码, 原体, &快照)
	快照.U检查体 = 检查体
	if len(检查体) > 0 && 媒体错误 == nil {
		提取体目标(快照.U内容类型, 参数["boundary"], 检查体, &快照)
	}
	return 快照, nil
}

// U目标列表 按稳定顺序拼接所有类别，保证首个命中规则可复现。
func (快照 U快照) U目标列表() []U目标 {
	结果 := make([]U目标, 0, 2+len(快照.U查询)+len(快照.U头)+len(快照.U表单)+
		len(快照.U多部分)+len(快照.UJSON)+len(快照.UXML)+len(快照.UCookie))
	结果 = append(结果,
		U目标{U类别: "path", U名称: "raw", U值: 快照.U原始路径},
		U目标{U类别: "path", U名称: "decoded", U值: 快照.U解码路径},
	)
	结果 = append(结果, 快照.U查询...)
	结果 = append(结果, 快照.U头...)
	结果 = append(结果, 快照.UCookie...)
	结果 = append(结果, 快照.U表单...)
	结果 = append(结果, 快照.U多部分...)
	结果 = append(结果, 快照.UJSON...)
	结果 = append(结果, 快照.UXML...)
	return 结果
}

func 提取查询(原始 string, 快照 *U快照) []U目标 {
	if 原始 == "" {
		return []U目标{}
	}
	值, 错误 := url.ParseQuery(原始)
	if 错误 != nil {
		快照.U跳过 = append(快照.U跳过, "invalid query encoding")
	}
	结果 := make([]U目标, 0, len(值))
	for 键, 列表 := range 值 {
		for _, 值项 := range 列表 {
			结果 = append(结果, U目标{U类别: "query", U名称: 键, U值: 值项})
		}
	}
	return 结果
}

func 提取头(头 http.Header, 名称列表 []string) []U目标 {
	结果 := make([]U目标, 0, len(名称列表))
	for _, 名称 := range 名称列表 {
		for _, 值 := range 头.Values(名称) {
			if 值 == "" {
				continue
			}
			结果 = append(结果, U目标{U类别: "header", U名称: 名称, U值: 值})
		}
	}
	return 结果
}

func 解压体(编码 string, 原体 []byte, 快照 *U快照) []byte {
	if len(原体) == 0 {
		return nil
	}
	switch 编码 {
	case "", "identity":
		return 原体
	case "gzip":
		return 读取解压(func(读者 io.Reader) (io.ReadCloser, error) { return gzip.NewReader(读者) }, 原体, "gzip", 快照)
	case "deflate":
		解压器, 错误 := zlib.NewReader(bytes.NewReader(原体))
		if 错误 == nil {
			return 读取解压器(解压器, "deflate", 快照)
		}
		return 读取解压(func(读者 io.Reader) (io.ReadCloser, error) { return flate.NewReader(读者), nil }, 原体, "deflate", 快照)
	default:
		快照.U跳过 = append(快照.U跳过, "unsupported content-encoding: "+编码)
		return nil
	}
}

func 读取解压(构造 func(io.Reader) (io.ReadCloser, error), 原体 []byte, 名称 string, 快照 *U快照) []byte {
	解压器, 错误 := 构造(bytes.NewReader(原体))
	if 错误 != nil {
		快照.U跳过 = append(快照.U跳过, "invalid "+名称+" body")
		return nil
	}
	return 读取解压器(解压器, 名称, 快照)
}

func 读取解压器(解压器 io.ReadCloser, 名称 string, 快照 *U快照) []byte {
	defer func() { _ = 解压器.Close() }()
	体, 错误 := io.ReadAll(io.LimitReader(解压器, 最大解压体积+1))
	if 错误 != nil {
		快照.U跳过 = append(快照.U跳过, "truncated "+名称+" body")
		return nil
	}
	if int64(len(体)) > 最大解压体积 {
		快照.U跳过 = append(快照.U跳过, "decompressed body exceeds 4 MiB")
		return nil
	}
	return 体
}

func 提取体目标(内容类型, 边界 string, 体 []byte, 快照 *U快照) {
	switch {
	case 内容类型 == "application/x-www-form-urlencoded":
		提取表单(体, 快照)
	case 内容类型 == "multipart/form-data":
		提取多部分(边界, 体, 快照)
	case 内容类型 == "application/json" || strings.HasSuffix(内容类型, "+json"):
		提取JSON(体, 快照)
	case 内容类型 == "application/xml", 内容类型 == "text/xml", strings.HasSuffix(内容类型, "+xml"):
		快照.UXML = append(快照.UXML, U目标{U类别: "xml", U名称: "body", U值: string(体)})
	}
}

func 提取表单(体 []byte, 快照 *U快照) {
	值, 错误 := url.ParseQuery(string(体))
	if 错误 != nil {
		快照.U跳过 = append(快照.U跳过, "invalid form encoding")
	}
	for 键, 列表 := range 值 {
		for _, 值项 := range 列表 {
			快照.U表单 = append(快照.U表单, U目标{U类别: "form", U名称: 键, U值: 值项})
		}
	}
}

func 提取多部分(边界 string, 体 []byte, 快照 *U快照) {
	if 边界 == "" {
		快照.U跳过 = append(快照.U跳过, "multipart boundary missing")
		return
	}
	读取器 := multipart.NewReader(bytes.NewReader(体), 边界)
	for 序号 := 0; 序号 < 最大分段数; 序号++ {
		部分, 错误 := 读取器.NextPart()
		if errors.Is(错误, io.EOF) {
			return
		}
		if 错误 != nil {
			快照.U跳过 = append(快照.U跳过, "invalid multipart body")
			return
		}
		处置, 处置参数, 处置错误 := mime.ParseMediaType(部分.Header.Get("Content-Disposition"))
		_ = 处置
		名称 := 处置参数["name"]
		文件名 := 处置参数["filename"]
		if 处置错误 != nil {
			名称 = 部分.FormName()
			文件名 = 部分.FileName()
		}
		if 名称 != "" {
			快照.U多部分 = append(快照.U多部分, U目标{U类别: "multipart", U名称: "field", U值: 名称})
		}
		if 文件名 != "" {
			快照.U多部分 = append(快照.U多部分, U目标{U类别: "multipart", U名称: "filename", U值: 文件名})
			_ = 部分.Close()
			continue
		}
		类型 := 媒体类型(部分.Header.Get("Content-Type"))
		if 类型 == "" {
			类型 = "text/plain"
		}
		if strings.HasPrefix(类型, "text/") || 类型 == "application/json" ||
			类型 == "application/xml" || strings.HasSuffix(类型, "+json") || strings.HasSuffix(类型, "+xml") {
			内容, _ := io.ReadAll(io.LimitReader(部分, 最大文本段))
			if 名称 == "" {
				名称 = "part"
			}
			快照.U多部分 = append(快照.U多部分, U目标{U类别: "multipart", U名称: 名称, U值: string(内容)})
		}
		_ = 部分.Close()
	}
	快照.U跳过 = append(快照.U跳过, "multipart parts truncated after 1000")
}

func 提取JSON(体 []byte, 快照 *U快照) {
	type 容器结构 struct {
		是对象 bool
		期待键 bool
		当前键 string
	}
	var 栈 []容器结构
	忽略深度 := 0
	解码器 := json.NewDecoder(bytes.NewReader(体))
	解码器.UseNumber()
	for {
		令牌, 错误 := 解码器.Token()
		if errors.Is(错误, io.EOF) {
			return
		}
		if 错误 != nil {
			快照.U跳过 = append(快照.U跳过, "invalid JSON body")
			return
		}
		if 定界符, 是定界 := 令牌.(json.Delim); 是定界 {
			switch 定界符 {
			case '{', '[':
				深度 := len(栈) + 1
				if 忽略深度 == 0 && 深度 > 最大JSON深度 {
					忽略深度 = 深度
					快照.U跳过 = append(快照.U跳过, "JSON deeper than 8 levels ignored")
				}
				栈 = append(栈, 容器结构{是对象: 定界符 == '{', 期待键: 定界符 == '{'})
			case '}', ']':
				if len(栈) == 0 {
					快照.U跳过 = append(快照.U跳过, "invalid JSON body")
					return
				}
				栈 = 栈[:len(栈)-1]
				if 忽略深度 > 0 && len(栈) < 忽略深度 {
					忽略深度 = 0
				}
				if len(栈) > 0 && 栈[len(栈)-1].是对象 {
					栈[len(栈)-1].期待键 = true
				}
			}
			continue
		}
		if 忽略深度 != 0 {
			continue
		}
		文本 := 标量文本(令牌)
		if len(栈) == 0 {
			快照.UJSON = append(快照.UJSON, U目标{U类别: "json", U名称: "value", U值: 文本})
			continue
		}
		当前 := &栈[len(栈)-1]
		if 当前.是对象 {
			if 当前.期待键 {
				键 := 文本
				快照.UJSON = append(快照.UJSON, U目标{U类别: "json", U名称: "key", U值: 键})
				当前.当前键 = 键
				当前.期待键 = false
				continue
			}
			名称 := 当前.当前键
			if 名称 == "" {
				名称 = "value"
			}
			快照.UJSON = append(快照.UJSON, U目标{U类别: "json", U名称: 名称, U值: 文本})
			当前.期待键 = true
			continue
		}
		名称 := "value"
		if len(栈) > 1 {
			if 父 := 栈[len(栈)-2]; 父.是对象 && 父.当前键 != "" {
				名称 = 父.当前键
			}
		}
		快照.UJSON = append(快照.UJSON, U目标{U类别: "json", U名称: 名称, U值: 文本})
	}
}

func 标量文本(值 any) string {
	switch 值 := 值.(type) {
	case string:
		return 值
	case json.Number:
		return 值.String()
	case bool:
		if 值 {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return fmt.Sprint(值)
	}
}

func 媒体类型(原文 string) string {
	媒体, _, 错误 := mime.ParseMediaType(原文)
	if 错误 != nil {
		return ""
	}
	return strings.ToLower(媒体)
}
