package 检查管线

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"unicode/utf8"
)

func Test提取路径查询与头(t *testing.T) {
	请求 := httptest.NewRequest("GET", "http://app.example.com/a%2Fb/../c?name=%3Cscript%3E&q=1&q=2", nil)
	请求.Header.Set("User-Agent", "sqlmap/1.8")
	请求.Header.Set("Referer", "http://app.example.com/")
	请求.Header.Set("X-Ignored", "not inspected")
	管线 := U新建([]string{"User-Agent", "Referer"})

	快照, 错误 := 管线.U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if 快照.U原始路径 != "/a%2Fb/../c" {
		t.Fatalf("raw path=%q", 快照.U原始路径)
	}
	if 快照.U解码路径 != "/a/b/../c" {
		t.Fatalf("decoded path=%q", 快照.U解码路径)
	}
	if !有目标(快照.U查询, "query", "name", "<script>") {
		t.Fatalf("decoded query target missing: %+v", 快照.U查询)
	}
	if !有目标(快照.U头, "header", "User-Agent", "sqlmap/1.8") {
		t.Fatalf("user-agent target missing: %+v", 快照.U头)
	}
	if 有名称(快照.U头, "X-Ignored") {
		t.Fatal("non-configured header was inspected")
	}
	目标列表 := 快照.U目标列表()
	if len(目标列表) < 4 || 目标列表[0].U类别 != "path" || 目标列表[1].U名称 != "decoded" {
		t.Fatalf("target order is not stable: %+v", 目标列表)
	}
}

func Test提取表单(t *testing.T) {
	请求 := httptest.NewRequest("POST", "http://app.example.com/login", strings.NewReader("user=admin&password=hunter2&q=a+b"))
	请求.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !有目标(快照.U表单, "form", "password", "hunter2") {
		t.Fatalf("form target missing: %+v", 快照.U表单)
	}
	if !有目标(快照.U表单, "form", "q", "a b") {
		t.Fatalf("form decoding failed: %+v", 快照.U表单)
	}
}

func Test提取JSON键与叶子(t *testing.T) {
	体 := `{"user":"bob","password":"s3cret","n":12,"ok":true,"none":null,"nested":{"q":"1 union select"},"arr":["x",{"inner":"y"}]}`
	请求 := httptest.NewRequest("POST", "http://app.example.com/api", strings.NewReader(体))
	请求.Header.Set("Content-Type", "application/json; charset=utf-8")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !有目标(快照.UJSON, "json", "key", "password") {
		t.Fatalf("json key target missing: %+v", 快照.UJSON)
	}
	if !有目标(快照.UJSON, "json", "password", "s3cret") {
		t.Fatalf("json leaf missing: %+v", 快照.UJSON)
	}
	if !有目标(快照.UJSON, "json", "n", "12") {
		t.Fatalf("json number missing: %+v", 快照.UJSON)
	}
	if !有目标(快照.UJSON, "json", "q", "1 union select") {
		t.Fatalf("nested json leaf missing: %+v", 快照.UJSON)
	}
}

func TestJSON深度上限(t *testing.T) {
	var 构建 strings.Builder
	构建.WriteString(`{"root":`)
	深度 := 9
	for 序号 := 0; 序号 < 深度; 序号++ {
		构建.WriteString(`{"d` + string(rune('0'+序号)) + `":`)
	}
	构建.WriteString(`"deep-value"`)
	for 序号 := 0; 序号 < 深度; 序号++ {
		构建.WriteString("}")
	}
	构建.WriteString("}")

	请求 := httptest.NewRequest("POST", "http://app.example.com/api", strings.NewReader(构建.String()))
	请求.Header.Set("Content-Type", "application/json")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !包含跳过(快照.U跳过, "deeper than 8") {
		t.Fatalf("depth skip not recorded: %+v", 快照.U跳过)
	}
	if 有值(快照.UJSON, "deep-value") {
		t.Fatal("leaf beyond depth 8 was collected")
	}
}

func Test提取多部分(t *testing.T) {
	var 缓冲 bytes.Buffer
	写器 := multipart.NewWriter(&缓冲)
	if 错误 := 写器.WriteField("username", "alice"); 错误 != nil {
		t.Fatalf("field: %v", 错误)
	}
	文件, 错误 := 写器.CreateFormFile("upload", "../../shell.php")
	if 错误 != nil {
		t.Fatalf("file: %v", 错误)
	}
	_, _ = 文件.Write([]byte("<?php system($_GET['c']); ?>"))
	文本头 := textproto.MIMEHeader{}
	文本头.Set("Content-Disposition", `form-data; name="comment"`)
	文本头.Set("Content-Type", "text/plain")
	文本, 错误 := 写器.CreatePart(文本头)
	if 错误 != nil {
		t.Fatalf("part: %v", 错误)
	}
	_, _ = 文本.Write([]byte("hello world"))
	if 错误 := 写器.Close(); 错误 != nil {
		t.Fatalf("close writer: %v", 错误)
	}

	请求 := httptest.NewRequest("POST", "http://app.example.com/upload", bytes.NewReader(缓冲.Bytes()))
	请求.Header.Set("Content-Type", 写器.FormDataContentType())
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !有目标(快照.U多部分, "multipart", "field", "username") {
		t.Fatalf("field name missing: %+v", 快照.U多部分)
	}
	if !有目标(快照.U多部分, "multipart", "filename", "../../shell.php") {
		t.Fatalf("filename missing: %+v", 快照.U多部分)
	}
	if !有目标(快照.U多部分, "multipart", "comment", "hello world") {
		t.Fatalf("text part missing: %+v", 快照.U多部分)
	}
}

func Test提取XML原文(t *testing.T) {
	原文 := `<?xml version="1.0"?><root><name>&lt;script&gt;</name></root>`
	请求 := httptest.NewRequest("POST", "http://app.example.com/xml", strings.NewReader(原文))
	请求.Header.Set("Content-Type", "text/xml")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if len(快照.UXML) != 1 || 快照.UXML[0].U值 != 原文 {
		t.Fatalf("xml raw text not preserved: %+v", 快照.UXML)
	}
}

func Test解压只用于检查(t *testing.T) {
	原体 := []byte("user=admin&password=abc")
	var 压缩 bytes.Buffer
	写器 := gzip.NewWriter(&压缩)
	if _, 错误 := 写器.Write(原体); 错误 != nil {
		t.Fatalf("gzip write: %v", 错误)
	}
	if 错误 := 写器.Close(); 错误 != nil {
		t.Fatalf("gzip close: %v", 错误)
	}

	请求 := httptest.NewRequest("POST", "http://app.example.com/login", bytes.NewReader(压缩.Bytes()))
	请求.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	请求.Header.Set("Content-Encoding", "gzip")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !有目标(快照.U表单, "form", "password", "abc") {
		t.Fatalf("decompressed form not inspected: %+v", 快照.U表单)
	}
	if !bytes.Equal(快照.U体, 压缩.Bytes()) {
		t.Fatal("original compressed bytes were replaced")
	}
}

func TestDeflate解压(t *testing.T) {
	原体 := []byte("user=admin")
	var 压缩 bytes.Buffer
	写器 := zlib.NewWriter(&压缩)
	_, _ = 写器.Write(原体)
	_ = 写器.Close()

	请求 := httptest.NewRequest("POST", "http://app.example.com/login", bytes.NewReader(压缩.Bytes()))
	请求.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	请求.Header.Set("Content-Encoding", "deflate")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !有目标(快照.U表单, "form", "user", "admin") {
		t.Fatalf("deflate body not inspected: %+v", 快照.U表单)
	}
}

func Test未知编码标记跳过(t *testing.T) {
	请求 := httptest.NewRequest("POST", "http://app.example.com/", strings.NewReader("abc"))
	请求.Header.Set("Content-Encoding", "br")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !包含跳过(快照.U跳过, "unsupported content-encoding") {
		t.Fatalf("skipped=%+v", 快照.U跳过)
	}
	if len(快照.U检查体) != 0 {
		t.Fatal("unsupported encoding should not be inspected")
	}
}

func Test解压炸弹被限制(t *testing.T) {
	var 压缩 bytes.Buffer
	写器 := gzip.NewWriter(&压缩)
	块 := bytes.Repeat([]byte{0}, 1<<20)
	for 序号 := 0; 序号 < 5; 序号++ {
		_, _ = 写器.Write(块)
	}
	_ = 写器.Close()

	请求 := httptest.NewRequest("POST", "http://app.example.com/", bytes.NewReader(压缩.Bytes()))
	请求.Header.Set("Content-Encoding", "gzip")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if !包含跳过(快照.U跳过, "decompressed body exceeds") {
		t.Fatalf("bomb not marked: %+v", 快照.U跳过)
	}
	if 快照.U检查体 != nil {
		t.Fatal("oversized decompressed body should be discarded")
	}
}

func Test请求体上限(t *testing.T) {
	请求 := httptest.NewRequest("POST", "http://app.example.com/", strings.NewReader("0123456789"))
	请求.Header.Set("Content-Type", "text/plain")
	_, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 5)
	var 超限 *U体过大错误
	if !errors.As(错误, &超限) {
		t.Fatalf("expected body-too-large, got %v", 错误)
	}

	分块请求 := httptest.NewRequest("POST", "http://app.example.com/", strings.NewReader(strings.Repeat("x", 64)))
	分块请求.ContentLength = -1
	_, 错误 = U新建(nil).U读取(httptest.NewRecorder(), 分块请求, 8)
	if !errors.As(错误, &超限) {
		t.Fatalf("expected chunked body-too-large, got %v", 错误)
	}
}

func Test脱敏头与查询(t *testing.T) {
	头 := jsonHeaders()
	清理 := U脱敏头(头)
	if 清理.Get("Authorization") != "<redacted>" || 清理.Get("X-Api-Key") != "<redacted>" {
		t.Fatalf("sensitive headers not redacted: %+v", 清理)
	}
	if 头.Get("Authorization") != "Bearer secret-token" {
		t.Fatal("redaction mutated the original header")
	}

	查询 := U脱敏查询("password=hunter2&q=ok&access_token=abc&card_number=4111")
	if strings.Contains(查询, "hunter2") || strings.Contains(查询, "abc") || strings.Contains(查询, "4111") {
		t.Fatalf("query not redacted: %s", 查询)
	}
	if !strings.Contains(查询, "q=ok") {
		t.Fatalf("benign query value lost: %s", 查询)
	}
}

func Test脱敏体(t *testing.T) {
	JSON体 := []byte(`{"user":"bob","password":"hunter2","nested":{"token":"abc"}}`)
	清理 := U脱敏体("application/json", JSON体)
	var 解析 map[string]any
	if 错误 := json.Unmarshal(清理, &解析); 错误 != nil {
		t.Fatalf("redacted JSON invalid: %v (%s)", 错误, 清理)
	}
	if 解析["password"] != "<redacted>" || 解析["user"] != "bob" {
		t.Fatalf("json redaction failed: %s", 清理)
	}
	if 嵌套, 可以 := 解析["nested"].(map[string]any); !可以 || 嵌套["token"] != "<redacted>" {
		t.Fatalf("nested redaction failed: %s", 清理)
	}

	表单 := U脱敏体("application/x-www-form-urlencoded", []byte("user=bob&password=hunter2"))
	if strings.Contains(string(表单), "hunter2") || !strings.Contains(string(表单), "user=bob") {
		t.Fatalf("form redaction failed: %s", 表单)
	}

	非法 := U脱敏体("application/json", []byte(`{"password":"hunter2"`))
	if strings.Contains(string(非法), "hunter2") {
		t.Fatalf("invalid JSON leaked: %s", 非法)
	}

	超长 := U脱敏体("text/plain", bytes.Repeat([]byte("a"), 4096))
	if len(超长) > U存储体上限 {
		t.Fatalf("stored body exceeded limit: %d", len(超长))
	}
}

func Test截断保持UTF8(t *testing.T) {
	值 := strings.Repeat("中", 10)
	截断 := U截断文本(值, 4)
	if len(截断) > 4 {
		t.Fatalf("length=%d", len(截断))
	}
	if !utf8.ValidString(截断) {
		t.Fatal("truncation produced invalid UTF-8")
	}
}

func Test目标JSON往返(t *testing.T) {
	原始 := U目标{U类别: "query", U名称: "q", U值: "1 union select"}
	数据, 错误 := json.Marshal(原始)
	if 错误 != nil {
		t.Fatalf("marshal: %v", 错误)
	}
	var 还原 U目标
	if 错误 := json.Unmarshal(数据, &还原); 错误 != nil {
		t.Fatalf("unmarshal: %v", 错误)
	}
	if 还原 != 原始 {
		t.Fatalf("round trip mismatch: %+v", 还原)
	}
	if !strings.Contains(string(数据), `"category"`) || !strings.Contains(string(数据), `"name"`) || !strings.Contains(string(数据), `"value"`) {
		t.Fatalf("unexpected JSON keys: %s", 数据)
	}
}

func 有目标(列表 []U目标, 类别, 名称, 值 string) bool {
	for _, 目标 := range 列表 {
		if 目标.U类别 == 类别 && 目标.U名称 == 名称 && 目标.U值 == 值 {
			return true
		}
	}
	return false
}

func 有名称(列表 []U目标, 名称 string) bool {
	for _, 目标 := range 列表 {
		if 目标.U名称 == 名称 {
			return true
		}
	}
	return false
}

func 有值(列表 []U目标, 值 string) bool {
	for _, 目标 := range 列表 {
		if 目标.U值 == 值 {
			return true
		}
	}
	return false
}

func 包含跳过(列表 []string, 片段 string) bool {
	for _, 项 := range 列表 {
		if strings.Contains(项, 片段) {
			return true
		}
	}
	return false
}

func jsonHeaders() http.Header {
	头 := http.Header{}
	头.Set("Authorization", "Bearer secret-token")
	头.Set("Cookie", "session=abc")
	头.Set("Set-Cookie", "session=abc")
	头.Set("X-Api-Key", "key-123")
	头.Set("User-Agent", "curl/8")
	return 头
}
