package 管理接口

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

type U日志列表响应 struct {
	U条目   []存储包.U日志条目 `json:"items"`
	U下一游标 string      `json:"next_cursor,omitempty"`
}

func (管理器 *U管理器) 日志路由(写 http.ResponseWriter, 请求 *http.Request) {
	后缀 := strings.TrimPrefix(请求.URL.Path, "/api/logs")
	后缀 = strings.TrimPrefix(后缀, "/")
	if 后缀 == "" {
		管理器.日志列表(写, 请求)
		return
	}
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	标识, 错误 := strconv.ParseInt(后缀, 10, 64)
	if 错误 != nil || 标识 <= 0 {
		写错误(写, http.StatusNotFound, 错误未找到, "log not found")
		return
	}
	条目, 错误 := 管理器.存储.U日志详情(请求.Context(), 标识)
	if 是未找到(错误) {
		写错误(写, http.StatusNotFound, 错误未找到, "log not found")
		return
	}
	if 错误 != nil {
		管理器.日志.Error("read log detail failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read log")
		return
	}
	写JSON(写, http.StatusOK, 条目)
}

func (管理器 *U管理器) 日志列表(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	查询 := 存储包.U日志查询{
		USite: 请求.URL.Query().Get("site"),
		U动作:   请求.URL.Query().Get("action"),
		U类别:   请求.URL.Query().Get("category"),
		U规则ID: 请求.URL.Query().Get("rule_id"),
		U地址:   请求.URL.Query().Get("ip"),
		U路径:   请求.URL.Query().Get("path"),
	}
	if 值 := 请求.URL.Query().Get("from"); 值 != "" {
		时刻, 错误 := time.Parse(time.RFC3339, 值)
		if 错误 != nil {
			写错误(写, http.StatusBadRequest, 错误请求, "from must be RFC3339")
			return
		}
		查询.U从 = 时刻.UTC().UnixMilli()
	}
	if 值 := 请求.URL.Query().Get("to"); 值 != "" {
		时刻, 错误 := time.Parse(time.RFC3339, 值)
		if 错误 != nil {
			写错误(写, http.StatusBadRequest, 错误请求, "to must be RFC3339")
			return
		}
		查询.U到 = 时刻.UTC().UnixMilli()
	}
	if 值 := 请求.URL.Query().Get("status"); 值 != "" {
		状态, 错误 := strconv.Atoi(值)
		if 错误 != nil || 状态 < 100 || 状态 > 599 {
			写错误(写, http.StatusBadRequest, 错误请求, "status must be an HTTP status code")
			return
		}
		查询.U状态 = 状态
	}
	限制 := 50
	if 值 := 请求.URL.Query().Get("limit"); 值 != "" {
		数量, 错误 := strconv.Atoi(值)
		if 错误 != nil || 数量 < 1 {
			写错误(写, http.StatusBadRequest, 错误请求, "limit must be a positive integer")
			return
		}
		if 数量 > 200 {
			数量 = 200
		}
		限制 = 数量
	}
	查询.U限制 = 限制
	if 游标 := 请求.URL.Query().Get("cursor"); 游标 != "" {
		时间, 标识, 错误 := 解码游标(游标)
		if 错误 != nil {
			写错误(写, http.StatusBadRequest, 错误请求, "cursor is invalid")
			return
		}
		查询.U游标时间 = 时间
		查询.U游标ID = 标识
	}

	条目列表, 错误 := 管理器.存储.U查询日志(请求.Context(), 查询)
	if 错误 != nil {
		管理器.日志.Error("query logs failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read logs")
		return
	}
	响应 := U日志列表响应{U条目: 条目列表}
	if len(条目列表) > 限制 {
		响应.U条目 = 条目列表[:限制]
		末尾 := 响应.U条目[len(响应.U条目)-1]
		响应.U下一游标 = 编码游标(末尾.U时间, 末尾.UID)
	}
	if 响应.U条目 == nil {
		响应.U条目 = []存储包.U日志条目{}
	}
	写JSON(写, http.StatusOK, 响应)
}

func 编码游标(时间, 标识 int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%d", 时间, 标识)))
}

func 解码游标(值 string) (int64, int64, error) {
	数据, 错误 := base64.RawURLEncoding.DecodeString(值)
	if 错误 != nil {
		return 0, 0, 错误
	}
	部分 := strings.SplitN(string(数据), ":", 2)
	if len(部分) != 2 {
		return 0, 0, errors.New("cursor needs ts:id")
	}
	时间, 错误 := strconv.ParseInt(部分[0], 10, 64)
	if 错误 != nil {
		return 0, 0, 错误
	}
	标识, 错误 := strconv.ParseInt(部分[1], 10, 64)
	if 错误 != nil {
		return 0, 0, 错误
	}
	return 时间, 标识, nil
}
