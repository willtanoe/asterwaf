# asterwaf 项目约定

本文件是仓库的长期约束。上下文被压缩或新会话开始时，先读本文件、`docs/decisions.md`、`docs/progress.md`。

## 语言与命名

- 与用户对话、进度总结：中文。
- 注释：中文，只解释“为什么”，不复述代码在做什么。
- 以下内容一律英文：README 与 `docs/*.md`、commit message、错误信息、日志消息文本、UI 文案、文件名、目录名、import 路径、YAML/JSON 键、API 路径、数据库表名与列名、slog 字段名、枚举值（`block`、`shadow`、`ratelimited`、`allow`）、错误码、指标名、测试数据中的攻击载荷名称。
- 导出标识符：必须以大写拉丁字母 `U` 开头，其后使用中文。例如 `U检查器`、`U解析配置`。只有首字符必须是 `U`。
- 未导出标识符：纯中文，不加前缀。例如 `规则集`、`解析目标`。
- 局部变量、参数、测试函数名使用中文。测试函数形如 `Test规则匹配`。
- package 子句使用中文包名，目录名与 import 路径保持英文。例如目录 `internal/engine` 中写 `package 引擎`。
- 实现标准库 interface 的方法名保持英文：`ServeHTTP`、`Close`、`Read`、`Write`、`Error`、`String`、`Unwrap`、`Rewrite`、`RoundTrip` 等。`main`、`init`、`TestMain`、`go:embed` 变量名保持英文。
- 前端：组件内部局部变量与函数可用中文；文件名、props、emits、路由名、API 字段保持英文。
- 每个参与 YAML、JSON 或 SQL 序列化的 struct 字段必须有显式 tag，键名英文，禁止依赖字段名推导。每个这类 struct 必须有往返测试。
- 不关闭或放宽 golangci-lint 规则。若规则与中文命名本质冲突，停止并询问用户。

## 代码风格

- 注释只写原因与约束，不写显而易见的过程。
- 不预先抽象；一个包装函数必须有两个以上调用方。
- 不写占位数据、lorem、营销文案、emoji。
- 新包必须有第二个使用方，并在同一 commit 更新本文件的包清单。

## 架构

- Go 1.27.3，`net/http` + `httputil.ReverseProxy`。
- 存储 SQLite（`modernc.org/sqlite`），WAL、`busy_timeout=5000`、`synchronous=NORMAL`、内嵌迁移。
- 配置 YAML 仅作为数据库空表时的种子。
- 前端 Vue 3 + Vite + TypeScript，构建产物通过 `go:embed` 嵌入，源码在 `web/`。
- module：`git.asteriaxlabs.com/willtanoe/asterwaf`。

### 目录与包清单

| 目录 | package | 职责 | 使用方 |
| --- | --- | --- | --- |
| `cmd/asterwaf` | `main` | 进程入口、子命令 | 无 |
| `internal/config` | `配置` | YAML 解析、默认值、校验 | cmd、proxy、api |
| `internal/clientip` | `客户端地址` | 受信代理与真实客户端地址 | proxy、api |
| `internal/proxy` | `反向代理` | 站点路由、反向代理、请求编排 | cmd |
| `internal/pipeline` | `检查管线` | 请求目标提取、解压、脱敏 | proxy |
| `internal/engine` | `规则引擎` | 规则编译、匹配、模式决策 | proxy、api |
| `internal/accesslist` | `访问列表` | ACL 匹配与 allow-only | proxy、api |
| `internal/ratelimit` | `限流` | 每 IP 令牌桶 | proxy、api |
| `internal/store` | `存储` | SQLite、迁移、异步日志、统计 | cmd、api、proxy、seed |
| `internal/api` | `管理接口` | 管理 HTTP API、会话、静态资源 | cmd |
| `internal/seed` | `合成流量` | 可复现合成流量 | cmd |
| `internal/calibration` | `校准` | 误报/漏报语料回归 | 测试 |
| `web` | `前端` | 嵌入式仪表盘资源（`go:embed`） | cmd |

## 工具链

- 系统 apt 的 Go 只用于引导；仓库内固定 Go 与 Node 在 `.toolchain/`，由 `toolchain.env` 的 URL 与 SHA256 决定。
- 所有 Makefile 配方与脚本设置 `GOTOOLCHAIN=local`，并把固定工具链放在 `PATH` 最前。
- 不从 github.com clone 或下载。依赖只经 Go module proxy、npm registry 获取，版本固定。
- 预批准依赖：`gopkg.in/yaml.v3`、`modernc.org/sqlite`、`golang.org/x/crypto`；前端 Vue 3、vue-router、Chart.js、Vite、TypeScript、vue-tsc、`@vitejs/plugin-vue`。额外依赖是停止条件。

## 安全底线

- 转发给上游的请求体必须是原始字节。
- 不对无界输入使用 `io.ReadAll`。
- 日志与审计不记录密码、token、cookie 明文。
- 管理面默认只监听 `127.0.0.1`。
- 日志页内容由攻击者控制，只以纯文本渲染。

## 常用命令

```sh
make bootstrap   # 下载并校验固定 Go/Node
make tools       # 安装固定 golangci-lint、govulncheck
make check       # gofmt + vet + lint + govulncheck + race 测试
make build       # 构建前端并生成 bin/asterwaf
make run         # 使用 config.yaml 启动
make seed        # 生成合成流量数据库
make stage-a     # Stage A 冒烟测试
```
