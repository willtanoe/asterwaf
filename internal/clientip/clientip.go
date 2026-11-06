package 客户端地址

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// U结果 是一次请求的客户端定位结果。U受信对端 表示直接对端是否在受信代理列表内。
type U结果 struct {
	U地址   netip.Addr
	U受信对端 bool
	U协议   string
}

// U解析器 保存受信代理 CIDR。空列表表示只信任 RemoteAddr。
type U解析器 struct {
	受信 []netip.Prefix
}

// U新建 解析受信代理列表。无效条目会让启动失败，避免静默降级。
func U新建(列表 []string) (*U解析器, error) {
	解析器 := &U解析器{受信: make([]netip.Prefix, 0, len(列表))}
	for 序号, 原文 := range 列表 {
		前缀, 错误 := netip.ParsePrefix(strings.TrimSpace(原文))
		if 错误 != nil {
			return nil, fmt.Errorf("trusted proxy %d: %w", 序号, 错误)
		}
		解析器.受信 = append(解析器.受信, 前缀.Masked())
	}
	return 解析器, nil
}

// U解析 按 RFC 7239 之前的 X-Forwarded-For 约定从右向左回溯。
// 无效条目会让解析停在最后一个有效跳，而不是采用更左侧的不可信值。
func (解析器 *U解析器) U解析(请求 *http.Request) U结果 {
	远程, 可解析 := 解析地址(请求.RemoteAddr)
	if !可解析 {
		return U结果{U协议: 入站协议(请求)}
	}

	对端受信 := 解析器.U受信地址(远程)
	结果 := U结果{U地址: 远程, U受信对端: 对端受信, U协议: 入站协议(请求)}
	if !对端受信 {
		return 结果
	}
	if 值 := 请求.Header.Get("X-Forwarded-Proto"); 值 == "http" || 值 == "https" {
		结果.U协议 = 值
	}

	链 := 请求.Header.Values("X-Forwarded-For")
	if len(链) == 0 {
		return 结果
	}

	最后有效 := 远程
	段列表 := 拆分链(链)
	for 索引 := len(段列表) - 1; 索引 >= 0; 索引-- {
		段 := 段列表[索引]
		if 段 == "" {
			continue
		}
		候选, 解析错误 := netip.ParseAddr(段)
		if 解析错误 != nil {
			结果.U地址 = 最后有效
			return 结果
		}
		最后有效 = 候选
		if 解析器.U受信地址(候选) {
			continue
		}
		结果.U地址 = 候选
		return 结果
	}
	结果.U地址 = 最后有效
	return 结果
}

// U受信地址 报告某个地址是否位于受信代理网段。
func (解析器 *U解析器) U受信地址(地址 netip.Addr) bool {
	地址 = 地址.Unmap()
	for _, 前缀 := range 解析器.受信 {
		if 前缀.Contains(地址) {
			return true
		}
	}
	return false
}

func 拆分链(值 []string) []string {
	var 结果 []string
	for _, 项 := range 值 {
		for _, 段 := range strings.Split(项, ",") {
			结果 = append(结果, strings.TrimSpace(段))
		}
	}
	return 结果
}

func 解析地址(原文 string) (netip.Addr, bool) {
	主机, _, 错误 := net.SplitHostPort(原文)
	if 错误 != nil {
		主机 = 原文
	}
	地址, 错误 := netip.ParseAddr(strings.Trim(主机, "[]"))
	if 错误 != nil {
		return netip.Addr{}, false
	}
	return 地址.Unmap(), true
}

func 入站协议(请求 *http.Request) string {
	if 请求.TLS != nil {
		return "https"
	}
	return "http"
}
