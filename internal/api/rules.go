package 管理接口

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	规则引擎 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/engine"
	检查管线 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/pipeline"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

const 规则请求体上限 = 64 << 10

type U规则输入 struct {
	UID    string `json:"id"`
	U名称    string `json:"name"`
	U类别    string `json:"category"`
	U正则    string `json:"pattern"`
	U启用    bool   `json:"enabled"`
	UScore int    `json:"score"`
}

type U规则列表响应 struct {
	U规则 []存储包.U规则行 `json:"rules"`
	U版本 int64      `json:"version"`
}

type U规则保存响应 struct {
	U规则 存储包.U规则行 `json:"rule"`
	U版本 int64    `json:"version"`
}

type U规则测试请求 struct {
	U规则 U规则输入  `json:"rule"`
	U载荷 string `json:"payload"`
}

type U规则测试响应 struct {
	U命中   bool   `json:"matched"`
	U规则ID string `json:"rule_id,omitempty"`
	U类别   string `json:"category,omitempty"`
}

func (管理器 *U管理器) 规则集合(写 http.ResponseWriter, 请求 *http.Request) {
	switch 请求.Method {
	case http.MethodGet:
		管理器.列出规则(写, 请求)
	case http.MethodPost:
		管理器.创建规则(写, 请求)
	default:
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "use GET or POST")
	}
}

func (管理器 *U管理器) 规则路由(写 http.ResponseWriter, 请求 *http.Request) {
	后缀 := strings.TrimPrefix(请求.URL.Path, "/api/rules/")
	switch {
	case 后缀 == "test":
		管理器.测试规则(写, 请求)
	case 后缀 == "versions":
		管理器.规则版本列表(写, 请求)
	case strings.HasPrefix(后缀, "versions/") && strings.HasSuffix(后缀, "/rollback"):
		管理器.回滚规则(写, 请求, strings.TrimSuffix(strings.TrimPrefix(后缀, "versions/"), "/rollback"))
	case strings.HasSuffix(后缀, "/toggle"):
		管理器.切换规则(写, 请求, strings.TrimSuffix(后缀, "/toggle"))
	case 后缀 != "":
		管理器.单条规则(写, 请求, 后缀)
	default:
		写错误(写, http.StatusNotFound, 错误未找到, "endpoint not found")
	}
}

func (管理器 *U管理器) 列出规则(写 http.ResponseWriter, 请求 *http.Request) {
	行列表, 错误 := 管理器.存储.U读取规则(请求.Context())
	if 错误 != nil {
		管理器.日志.Error("read rules failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rules")
		return
	}
	版本, 错误 := 管理器.存储.U最新规则版本(请求.Context())
	if 错误 != nil {
		管理器.日志.Error("read rule version failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rule version")
		return
	}
	写JSON(写, http.StatusOK, U规则列表响应{U规则: 行列表, U版本: 版本})
}

func (管理器 *U管理器) 创建规则(写 http.ResponseWriter, 请求 *http.Request) {
	var 输入 U规则输入
	if !读取JSON(写, 请求, &输入, 规则请求体上限) {
		return
	}
	行列表, 错误 := 管理器.存储.U读取规则(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rules")
		return
	}
	for _, 行 := range 行列表 {
		if 行.UID == 输入.UID {
			写错误(写, http.StatusConflict, 错误冲突, "rule id already exists")
			return
		}
	}
	新行 := 存储包.U规则行{
		UID: 输入.UID, U名称: 输入.U名称, U类别: 输入.U类别, U正则: 输入.U正则,
		U启用: 输入.U启用, UScore: 输入.UScore, U来源: "custom",
		U顺序: len(行列表), U创建时间: time.Now().UnixMilli(),
	}
	行列表 = append(行列表, 新行)
	if !管理器.编译通过(写, 行列表) {
		return
	}
	版本, 错误 := 管理器.保存规则(写, 请求, 行列表, "rule_create", "", 输入.UID, "create rule")
	if 错误 != nil {
		return
	}
	写JSON(写, http.StatusCreated, U规则保存响应{U规则: 新行, U版本: 版本})
}

func (管理器 *U管理器) 单条规则(写 http.ResponseWriter, 请求 *http.Request, 标识 string) {
	行列表, 错误 := 管理器.存储.U读取规则(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rules")
		return
	}
	序号 := -1
	for 下标, 行 := range 行列表 {
		if 行.UID == 标识 {
			序号 = 下标
			break
		}
	}
	if 序号 < 0 {
		写错误(写, http.StatusNotFound, 错误未找到, "rule not found")
		return
	}
	switch 请求.Method {
	case http.MethodGet:
		写JSON(写, http.StatusOK, 行列表[序号])
	case http.MethodPut:
		var 输入 U规则输入
		if !读取JSON(写, 请求, &输入, 规则请求体上限) {
			return
		}
		原行 := 行列表[序号]
		原JSON, _ := json.Marshal(原行)
		行列表[序号] = 存储包.U规则行{
			UID: 原行.UID, U名称: 输入.U名称, U类别: 输入.U类别, U正则: 输入.U正则,
			U启用: 输入.U启用, UScore: 输入.UScore, U来源: "custom",
			U默认版本: 原行.U默认版本, U顺序: 原行.U顺序,
			U创建时间: 原行.U创建时间,
		}
		if !管理器.编译通过(写, 行列表) {
			return
		}
		版本, 错误 := 管理器.保存规则(写, 请求, 行列表, "rule_update", string(原JSON), 标识, "update rule")
		if 错误 != nil {
			return
		}
		写JSON(写, http.StatusOK, U规则保存响应{U规则: 行列表[序号], U版本: 版本})
	case http.MethodDelete:
		原JSON, _ := json.Marshal(行列表[序号])
		行列表 = append(行列表[:序号], 行列表[序号+1:]...)
		if !管理器.编译通过(写, 行列表) {
			return
		}
		if _, 错误 := 管理器.保存规则(写, 请求, 行列表, "rule_delete", string(原JSON), 标识, "delete rule"); 错误 != nil {
			return
		}
		写.WriteHeader(http.StatusNoContent)
	default:
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "use GET, PUT or DELETE")
	}
}

func (管理器 *U管理器) 切换规则(写 http.ResponseWriter, 请求 *http.Request, 标识 string) {
	if 请求.Method != http.MethodPost {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only POST is supported")
		return
	}
	行列表, 错误 := 管理器.存储.U读取规则(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rules")
		return
	}
	找到 := false
	for 下标 := range 行列表 {
		if 行列表[下标].UID != 标识 {
			continue
		}
		找到 = true
		行列表[下标].U启用 = !行列表[下标].U启用
	}
	if !找到 {
		写错误(写, http.StatusNotFound, 错误未找到, "rule not found")
		return
	}
	if !管理器.编译通过(写, 行列表) {
		return
	}
	版本, 错误 := 管理器.保存规则(写, 请求, 行列表, "rule_toggle", "", 标识, "toggle rule")
	if 错误 != nil {
		return
	}
	for _, 行 := range 行列表 {
		if 行.UID == 标识 {
			写JSON(写, http.StatusOK, U规则保存响应{U规则: 行, U版本: 版本})
			return
		}
	}
}

func (管理器 *U管理器) 测试规则(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodPost {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only POST is supported")
		return
	}
	var 输入 U规则测试请求
	if !读取JSON(写, 请求, &输入, 规则请求体上限) {
		return
	}
	引擎, 错误 := 规则引擎.U新建([]规则引擎.U规则{{
		UID: 输入.U规则.UID, U名称: 输入.U规则.U名称, U类别: 输入.U规则.U类别,
		U正则: 输入.U规则.U正则, U启用: true, UScore: 输入.U规则.UScore,
	}})
	if 错误 != nil {
		写错误(写, http.StatusBadRequest, 错误请求, 错误.Error())
		return
	}
	决策 := 引擎.U决策("enforce", []检查管线.U目标{{U类别: "test", U名称: "payload", U值: 输入.U载荷}})
	写JSON(写, http.StatusOK, U规则测试响应{
		U命中:   决策.U动作 == 规则引擎.U动作拦截,
		U规则ID: 决策.U规则ID, U类别: 决策.U类别,
	})
}

func (管理器 *U管理器) 规则版本列表(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	列表, 错误 := 管理器.存储.U版本列表(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rule versions")
		return
	}
	写JSON(写, http.StatusOK, map[string]any{"versions": 列表})
}

func (管理器 *U管理器) 回滚规则(写 http.ResponseWriter, 请求 *http.Request, 标识 string) {
	if 请求.Method != http.MethodPost {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only POST is supported")
		return
	}
	版本, 错误 := strconv.ParseInt(标识, 10, 64)
	if 错误 != nil || 版本 <= 0 {
		写错误(写, http.StatusBadRequest, 错误请求, "version must be a positive integer")
		return
	}
	行列表, 版本信息, 错误 := 管理器.存储.U版本详情(请求.Context(), 版本)
	if 是未找到(错误) {
		写错误(写, http.StatusNotFound, 错误未找到, "rule version not found")
		return
	}
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rule version")
		return
	}
	if !管理器.编译通过(写, 行列表) {
		return
	}
	新版本, 错误 := 管理器.保存规则(写, 请求, 行列表, "rule_rollback",
		fmt.Sprintf(`{"version":%d}`, 版本), fmt.Sprint(版本),
		fmt.Sprintf("rollback to version %d (%s)", 版本, 版本信息.U备注))
	if 错误 != nil {
		return
	}
	新行, 错误 := 管理器.存储.U读取规则(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not reload rules")
		return
	}
	写JSON(写, http.StatusOK, map[string]any{"version": 新版本, "rules": 新行})
}

func (管理器 *U管理器) 编译通过(写 http.ResponseWriter, 行列表 []存储包.U规则行) bool {
	规则列表 := make([]规则引擎.U规则, 0, len(行列表))
	for _, 行 := range 行列表 {
		规则列表 = append(规则列表, 行.U转为规则())
	}
	if _, 错误 := 规则引擎.U新建(规则列表); 错误 != nil {
		写错误(写, http.StatusBadRequest, 错误请求, 错误.Error())
		return false
	}
	return true
}

func (管理器 *U管理器) 保存规则(写 http.ResponseWriter, 请求 *http.Request, 行列表 []存储包.U规则行, 动作, 前, 标识, 说明 string) (int64, error) {
	会话, _ := 请求.Context().Value(会话上下文键{}).(会话值)
	客户端 := 管理器.解析器.U解析(请求)
	版本, 错误 := 管理器.存储.U保存规则集(请求.Context(), 行列表, 存储包.U审计记录{
		U操作者: 会话.U用户名, U动作: 动作, U前: 前, U客户端IP: 客户端.U地址.String(),
	}, 说明)
	if 错误 != nil {
		管理器.日志.Error("save rules failed", "action", 动作, "rule", 标识, "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not save rules")
		return 0, 错误
	}
	新行, 错误 := 管理器.存储.U读取规则(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not reload rules")
		return 0, 错误
	}
	规则列表 := make([]规则引擎.U规则, 0, len(新行))
	for _, 行 := range 新行 {
		规则列表 = append(规则列表, 行.U转为规则())
	}
	if 错误 := 管理器.引擎.U安装(规则列表, 版本); 错误 != nil {
		管理器.日志.Error("install ruleset failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not activate rules")
		return 0, 错误
	}
	return 版本, nil
}
