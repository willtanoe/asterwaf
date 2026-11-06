package 管理接口

import (
	"encoding/json"
	"net/http"
)

const (
	错误请求  = "invalid_request"
	错误未认证 = "unauthorized"
	错误禁止  = "forbidden"
	错误未找到 = "not_found"
	错误冲突  = "conflict"
	错误限流  = "rate_limited"
	错误内部  = "internal"
)

type U错误详情 struct {
	U代码 string `json:"code"`
	U消息 string `json:"message"`
}

type U错误响应 struct {
	U错误 U错误详情 `json:"error"`
}

func 写JSON(写 http.ResponseWriter, 状态 int, 值 any) {
	写.Header().Set("Content-Type", "application/json")
	写.WriteHeader(状态)
	_ = json.NewEncoder(写).Encode(值)
}

func 写错误(写 http.ResponseWriter, 状态 int, 代码, 消息 string) {
	写JSON(写, 状态, U错误响应{U错误: U错误详情{U代码: 代码, U消息: 消息}})
}
