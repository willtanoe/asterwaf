package 管理接口

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/accesslist"
	"git.asteriaxlabs.com/willtanoe/asterwaf/internal/ratelimit"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

type U站点视图 struct {
	U主机     string `json:"host"`
	U上游     string `json:"upstream"`
	U模式     string `json:"mode"`
	U健康检查路径 string `json:"health_path"`
	U最大请求体  int64  `json:"max_body_bytes"`
	U健康状态   string `json:"status"`
	U延迟毫秒   int64  `json:"latency_ms"`
	U失败次数   int    `json:"failures"`
	U最后检查   int64  `json:"last_check"`
}

func (管理器 *U管理器) 访问列表集合(写 http.ResponseWriter, 请求 *http.Request) {
	switch 请求.Method {
	case http.MethodGet:
		列表, 错误 := 管理器.存储.U读取访问列表(请求.Context())
		if 错误 != nil {
			写错误(写, http.StatusInternalServerError, 错误内部, "could not read access lists")
			return
		}
		写JSON(写, http.StatusOK, 列表)
	case http.MethodPost:
		管理器.创建访问条目(写, 请求)
	default:
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "use GET or POST")
	}
}

func (管理器 *U管理器) 访问列表路由(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodDelete {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only DELETE is supported")
		return
	}
	标识, 错误 := url.PathUnescape(strings.TrimPrefix(请求.URL.Path, "/api/access-lists/"))
	if 错误 != nil || 标识 == "" {
		写错误(写, http.StatusBadRequest, 错误请求, "entry id is invalid")
		return
	}
	列表, 错误 := 管理器.存储.U读取访问列表(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access lists")
		return
	}
	var 命中 *访问列表.U条目
	for 下标 := range 列表.U条目 {
		if 列表.U条目[下标].UID == 标识 {
			命中 = &列表.U条目[下标]
			break
		}
	}
	if 命中 == nil {
		写错误(写, http.StatusNotFound, 错误未找到, "access list entry not found")
		return
	}
	前, _ := json.Marshal(命中)
	审计 := 管理器.构造审计(请求, "access_list_delete", string(前), "")
	if 错误 := 管理器.存储.U删除访问条目(请求.Context(), 标识, 审计); 是未找到(错误) {
		写错误(写, http.StatusNotFound, 错误未找到, "access list entry not found")
		return
	} else if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not delete access list entry")
		return
	}
	if 错误 := 管理器.刷新访问列表(请求); 错误 != nil {
		管理器.日志.Error("refresh access lists failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not apply access lists")
		return
	}
	写.WriteHeader(http.StatusNoContent)
}

func (管理器 *U管理器) 创建访问条目(写 http.ResponseWriter, 请求 *http.Request) {
	var 条目 访问列表.U条目
	if !读取JSON(写, 请求, &条目, 64<<10) {
		return
	}
	列表, 错误 := 管理器.存储.U读取访问列表(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read access lists")
		return
	}
	if 条目.UID == "" {
		条目.UID = fmt.Sprintf("acl-%d", time.Now().UnixNano())
	}
	for _, 已有 := range 列表.U条目 {
		if 已有.UID == 条目.UID {
			写错误(写, http.StatusConflict, 错误冲突, "access list entry id already exists")
			return
		}
	}
	候选 := 访问列表.U列表{U仅允许: 列表.U仅允许, U条目: append(append([]访问列表.U条目{}, 列表.U条目...), 条目)}
	if _, 错误 := 访问列表.U新建(候选); 错误 != nil {
		写错误(写, http.StatusBadRequest, 错误请求, 错误.Error())
		return
	}
	后, _ := json.Marshal(条目)
	审计 := 管理器.构造审计(请求, "access_list_create", "", string(后))
	if 错误 := 管理器.存储.U新增访问条目(请求.Context(), 条目, 审计); 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not create access list entry")
		return
	}
	if 错误 := 管理器.刷新访问列表(请求); 错误 != nil {
		管理器.日志.Error("refresh access lists failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not apply access lists")
		return
	}
	写JSON(写, http.StatusCreated, 条目)
}

func (管理器 *U管理器) 刷新访问列表(请求 *http.Request) error {
	列表, 错误 := 管理器.存储.U读取访问列表(请求.Context())
	if 错误 != nil {
		return 错误
	}
	return 管理器.代理.U更新访问列表(列表)
}

func (管理器 *U管理器) 限流设置(写 http.ResponseWriter, 请求 *http.Request) {
	switch 请求.Method {
	case http.MethodGet:
		配置值, 错误 := 管理器.存储.U读取限流(请求.Context())
		if 错误 != nil {
			写错误(写, http.StatusInternalServerError, 错误内部, "could not read rate limit")
			return
		}
		写JSON(写, http.StatusOK, 配置值)
	case http.MethodPut:
		管理器.保存限流(写, 请求)
	default:
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "use GET or PUT")
	}
}

func (管理器 *U管理器) 保存限流(写 http.ResponseWriter, 请求 *http.Request) {
	var 配置值 限流.U配置
	if !读取JSON(写, 请求, &配置值, 64<<10) {
		return
	}
	if 配置值.U每分钟 <= 0 || 配置值.U突发 <= 0 {
		写错误(写, http.StatusBadRequest, 错误请求, "requests_per_minute and burst must be positive")
		return
	}
	旧值, 错误 := 管理器.存储.U读取限流(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read rate limit")
		return
	}
	前, _ := json.Marshal(旧值)
	后, _ := json.Marshal(配置值)
	审计 := 管理器.构造审计(请求, "rate_limit_update", string(前), string(后))
	if 错误 := 管理器.存储.U保存限流(请求.Context(), 配置值, 审计); 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not save rate limit")
		return
	}
	if 错误 := 管理器.代理.U更新限流(配置值); 错误 != nil {
		管理器.日志.Error("apply rate limit failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not apply rate limit")
		return
	}
	写JSON(写, http.StatusOK, 配置值)
}

func (管理器 *U管理器) 站点集合(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodGet {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only GET is supported")
		return
	}
	模式表, 错误 := 管理器.存储.U读取站点模式(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read site modes")
		return
	}
	健康表 := make(map[string]U站点健康)
	for _, 项 := range 管理器.探测器.U状态() {
		健康表[项.U主机] = 项
	}
	结果 := make([]U站点视图, 0, len(管理器.配置.U站点))
	for _, 站点值 := range 管理器.配置.U站点 {
		模式 := 站点值.U模式
		if 值, 存在 := 模式表[站点值.U主机]; 存在 {
			模式 = 值
		}
		健康 := 健康表[站点值.U主机]
		结果 = append(结果, U站点视图{
			U主机: 站点值.U主机, U上游: 站点值.U上游, U模式: 模式,
			U健康检查路径: 站点值.U健康检查路径, U最大请求体: 站点值.U最大请求体,
			U健康状态: 健康.U状态, U延迟毫秒: 健康.U延迟毫秒, U失败次数: 健康.U失败次数,
			U最后检查: 健康.U最后检查,
		})
	}
	写JSON(写, http.StatusOK, map[string]any{"sites": 结果})
}

func (管理器 *U管理器) 站点路由(写 http.ResponseWriter, 请求 *http.Request) {
	if 请求.Method != http.MethodPut {
		写错误(写, http.StatusMethodNotAllowed, 错误请求, "only PUT is supported")
		return
	}
	后缀 := strings.TrimPrefix(请求.URL.Path, "/api/sites/")
	if !strings.HasSuffix(后缀, "/mode") {
		写错误(写, http.StatusNotFound, 错误未找到, "endpoint not found")
		return
	}
	主机, 错误 := url.PathUnescape(strings.TrimSuffix(后缀, "/mode"))
	if 错误 != nil || 主机 == "" {
		写错误(写, http.StatusBadRequest, 错误请求, "site host is invalid")
		return
	}
	var 负载 struct {
		U模式 string `json:"mode"`
	}
	if !读取JSON(写, 请求, &负载, 16<<10) {
		return
	}
	if 负载.U模式 != "off" && 负载.U模式 != "shadow" && 负载.U模式 != "enforce" {
		写错误(写, http.StatusBadRequest, 错误请求, "mode must be off, shadow or enforce")
		return
	}
	站点值, 存在 := 管理器.配置.U按主机取站点(主机)
	if !存在 || 站点值.U主机 != 主机 {
		写错误(写, http.StatusNotFound, 错误未找到, "site not found")
		return
	}
	模式表, 错误 := 管理器.存储.U读取站点模式(请求.Context())
	if 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not read site modes")
		return
	}
	旧模式 := 站点值.U模式
	if 值, 存在 := 模式表[主机]; 存在 {
		旧模式 = 值
	}
	前, _ := json.Marshal(map[string]string{"mode": 旧模式})
	后, _ := json.Marshal(map[string]string{"mode": 负载.U模式})
	审计 := 管理器.构造审计(请求, "site_mode_update", string(前), string(后))
	if 错误 := 管理器.存储.U保存站点模式(请求.Context(), 主机, 负载.U模式, 审计); 错误 != nil {
		写错误(写, http.StatusInternalServerError, 错误内部, "could not save site mode")
		return
	}
	if 错误 := 管理器.代理.U更新站点模式(主机, 负载.U模式); 错误 != nil {
		管理器.日志.Error("apply site mode failed", "error", 错误)
		写错误(写, http.StatusInternalServerError, 错误内部, "could not apply site mode")
		return
	}
	写JSON(写, http.StatusOK, map[string]string{"host": 主机, "mode": 负载.U模式})
}

func (管理器 *U管理器) 构造审计(请求 *http.Request, 动作, 前, 后 string) 存储包.U审计记录 {
	会话, _ := 请求.Context().Value(会话上下文键{}).(会话值)
	客户端 := 管理器.解析器.U解析(请求)
	return 存储包.U审计记录{
		U操作者: 会话.U用户名, U动作: 动作, U前: 前, U后: 后, U客户端IP: 客户端.U地址.String(),
	}
}
