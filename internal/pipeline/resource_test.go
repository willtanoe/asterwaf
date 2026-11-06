package 检查管线

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test大体积请求在读取前拒绝(t *testing.T) {
	请求 := httptest.NewRequest("POST", "http://app.example.com/upload", strings.NewReader(strings.Repeat("x", 2<<20)))
	请求.Header.Set("Content-Type", "text/plain")
	管线 := U新建(nil)
	_, 错误 := 管线.U读取(httptest.NewRecorder(), 请求, 1024)
	var 超限 *U体过大错误
	if !errors.As(错误, &超限) {
		t.Fatalf("expected body-too-large, got %v", 错误)
	}
}

func Test深层JSON被截断(t *testing.T) {
	深度 := 200
	构建 := strings.Builder{}
	for 序号 := 0; 序号 < 深度; 序号++ {
		构建.WriteString(`{"a":`)
	}
	构建.WriteString(`"deep"`)
	for 序号 := 0; 序号 < 深度; 序号++ {
		构建.WriteString("}")
	}
	请求 := httptest.NewRequest("POST", "http://app.example.com/api", strings.NewReader(构建.String()))
	请求.Header.Set("Content-Type", "application/json")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if 有值(快照.UJSON, "deep") {
		t.Fatal("leaf beyond depth limit was collected")
	}
	if !包含跳过(快照.U跳过, "deeper than") {
		t.Fatalf("skip reason missing: %+v", 快照.U跳过)
	}
}

func Test高压缩比炸弹被丢弃(t *testing.T) {
	var 压缩 bytes.Buffer
	写器 := gzip.NewWriter(&压缩)
	块 := make([]byte, 1<<20)
	for 序号 := 0; 序号 < 8; 序号++ {
		if _, 错误 := 写器.Write(块); 错误 != nil {
			t.Fatalf("write: %v", 错误)
		}
	}
	_ = 写器.Close()
	if 压缩.Len() > 1<<20 {
		t.Fatalf("compressed payload too large for the test: %d", 压缩.Len())
	}
	请求 := httptest.NewRequest("POST", "http://app.example.com/", bytes.NewReader(压缩.Bytes()))
	请求.Header.Set("Content-Encoding", "gzip")
	快照, 错误 := U新建(nil).U读取(httptest.NewRecorder(), 请求, 1<<20)
	if 错误 != nil {
		t.Fatalf("read: %v", 错误)
	}
	if 快照.U检查体 != nil {
		t.Fatal("decompressed bomb was kept")
	}
	if !包含跳过(快照.U跳过, "exceeds 4 MiB") {
		t.Fatalf("bomb not marked: %+v", 快照.U跳过)
	}
}
