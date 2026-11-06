package 合成流量

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	检查管线 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/pipeline"
	存储包 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/store"
)

// 每小时权重刻画工作日的日内曲线，夜间低、午后高。
var 小时权重 = []int{2, 1, 1, 1, 2, 3, 5, 8, 12, 16, 20, 22, 20, 22, 24, 24, 22, 20, 18, 14, 10, 7, 5, 3}

type U参数 struct {
	U结束时间  time.Time
	U天数    int
	U站点    []string
	U每小时系数 int
}

type 正常请求 struct {
	路径   string
	查询   string
	方法   string
	状态   int
	权重   int
	内容类型 string
}

type 攻击请求 struct {
	名称   string
	路径   string
	查询   string
	方法   string
	内容类型 string
	载荷   string
	类别   string
	规则ID string
	用户代理 string
}

var 正常请求池 = []正常请求{
	{路径: "/", 方法: "GET", 状态: 200, 权重: 18},
	{路径: "/rest/products/search", 查询: "q=apple", 方法: "GET", 状态: 200, 权重: 14},
	{路径: "/rest/products/search", 查询: "q=orange%20juice", 方法: "GET", 状态: 200, 权重: 10},
	{路径: "/api/Products", 方法: "GET", 状态: 200, 权重: 10},
	{路径: "/api/Products/1", 方法: "GET", 状态: 200, 权重: 7},
	{路径: "/assets/i18n/en.json", 方法: "GET", 状态: 200, 权重: 8},
	{路径: "/assets/public/images/products/apple_juice.jpg", 方法: "GET", 状态: 200, 权重: 6},
	{路径: "/rest/user/login", 方法: "POST", 状态: 200, 权重: 5, 内容类型: "application/json"},
	{路径: "/api/Feedbacks", 方法: "GET", 状态: 200, 权重: 4},
	{路径: "/rest/basket/1", 方法: "GET", 状态: 200, 权重: 5},
	{路径: "/api/Challenges", 方法: "GET", 状态: 200, 权重: 3},
	{路径: "/rest/products/1/reviews", 方法: "GET", 状态: 200, 权重: 3},
	{路径: "/favicon.ico", 方法: "GET", 状态: 200, 权重: 2},
	{路径: "/api/Quantitys", 方法: "GET", 状态: 200, 权重: 2},
	{路径: "/rest/user/whoami", 方法: "GET", 状态: 401, 权重: 3},
}

var 攻击请求池 = []攻击请求{
	{名称: "union select", 路径: "/rest/products/search", 查询: "q=apple%27%20UNION%20SELECT%20email,password%20FROM%20Users--", 方法: "GET", 类别: "sqli", 规则ID: "sqli.union-select", 用户代理: "sqlmap/1.8.7"},
	{名称: "boolean sqli", 路径: "/rest/products/search", 查询: "q=%27%20OR%201%3D1--", 方法: "GET", 类别: "sqli", 规则ID: "sqli.boolean", 用户代理: "Mozilla/5.0"},
	{名称: "script xss", 路径: "/search", 查询: "q=%3Cscript%3Ealert(1)%3C%2Fscript%3E", 方法: "GET", 类别: "xss", 规则ID: "xss.script", 用户代理: "Mozilla/5.0"},
	{名称: "event xss", 路径: "/search", 查询: "q=%3Cimg%20src%3Dx%20onerror%3Dalert(1)%3E", 方法: "GET", 类别: "xss", 规则ID: "xss.event", 用户代理: "Mozilla/5.0"},
	{名称: "traversal", 路径: "/download", 查询: "file=../../../../etc/passwd", 方法: "GET", 类别: "traversal", 规则ID: "traversal.dotdot", 用户代理: "Mozilla/5.0"},
	{名称: "shell", 路径: "/search", 查询: "q=%3Bcat%20%2Fetc%2Fpasswd", 方法: "GET", 类别: "rce", 规则ID: "rce.shell-meta", 用户代理: "Mozilla/5.0"},
	{名称: "php payload", 路径: "/file-upload", 方法: "POST", 内容类型: "application/x-www-form-urlencoded", 载荷: "code=%3C%3Fphp%20system(%24_GET%5B%27c%27%5D)%3B%20%3F%3E", 类别: "rce", 规则ID: "rce.php", 用户代理: "Mozilla/5.0"},
	{名称: "scanner", 路径: "/.git/config", 方法: "GET", 类别: "scanner", 规则ID: "scanner.path", 用户代理: "Nikto/2.5.0"},
}

var 浏览器代理 = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/131.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 Version/18.1 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0",
}

// U生成 产生确定性合成记录；同一参数与同一机器时钟下结果一致。
func U生成(参数 U参数) []存储包.U记录 {
	if 参数.U天数 <= 0 {
		参数.U天数 = 14
	}
	if 参数.U每小时系数 <= 0 {
		参数.U每小时系数 = 7
	}
	if len(参数.U站点) == 0 {
		参数.U站点 = []string{"app.example.com"}
	}
	随机源 := rand.New(rand.NewSource(20261106))
	地址池 := 构建地址池(随机源)
	结束 := 参数.U结束时间.UTC()
	开始 := 结束.Truncate(24 * time.Hour).Add(-time.Duration(参数.U天数) * 24 * time.Hour)
	记录列表 := make([]存储包.U记录, 0, 参数.U天数*2200)
	序号 := 0
	for 日 := 0; ; 日++ {
		日初 := 开始.Add(time.Duration(日) * 24 * time.Hour)
		if 日初.After(结束) {
			break
		}
		for 小时, 权重 := range 小时权重 {
			小时初 := 日初.Add(time.Duration(小时) * time.Hour)
			if 小时初.After(结束) {
				continue
			}
			数量 := 权重 * 参数.U每小时系数
			for 次 := 0; 次 < 数量; 次++ {
				时刻 := 小时初.Add(time.Duration(随机源.Intn(3600)) * time.Second)
				if 时刻.After(结束) {
					时刻 = 结束
				}
				站点 := 选择站点(随机源, 参数.U站点)
				地址 := 选择地址(随机源, 地址池)
				记录 := 生成一条(随机源, 序号, 时刻, 站点, 地址)
				记录列表 = append(记录列表, 记录)
				序号++
			}
		}
	}
	return 记录列表
}

func 生成一条(随机源 *rand.Rand, 序号 int, 时刻 time.Time, 站点, 地址 string) 存储包.U记录 {
	记录 := 存储包.U记录{
		U请求ID:  fmt.Sprintf("seed-%08d", 序号),
		U时间:    时刻.UnixMilli(),
		USite:  站点,
		U客户端IP: 地址,
		U用户代理:  浏览器代理[随机源.Intn(len(浏览器代理))],
		U耗时毫秒:  延迟(随机源),
		U种子:    true,
		U动作:    "allow",
		U状态:    200,
		U请求头:   "",
	}
	if 随机源.Intn(100) < 8 {
		return 填充攻击(随机源, 记录, 序号)
	}
	正常 := 加权正常(随机源)
	记录.U方法 = 正常.方法
	记录.U路径 = 正常.路径
	记录.U查询 = 检查管线.U脱敏查询(正常.查询)
	记录.U状态 = 正常.状态
	return 记录
}

func 填充攻击(随机源 *rand.Rand, 记录 存储包.U记录, 序号 int) 存储包.U记录 {
	攻击 := 攻击请求池[随机源.Intn(len(攻击请求池))]
	记录.U方法 = 攻击.方法
	记录.U路径 = 攻击.路径
	记录.U查询 = 检查管线.U脱敏查询(攻击.查询)
	记录.U用户代理 = 攻击.用户代理
	记录.U类别 = 攻击.类别
	记录.U规则ID = 攻击.规则ID
	记录.U动作 = "block"
	记录.U状态 = 403
	掷点 := 随机源.Intn(100)
	switch {
	case 掷点 < 55:
		// 默认拦截
	case 掷点 < 75:
		记录.U动作 = "shadow"
		记录.U影子 = true
		记录.U状态 = 200
	case 掷点 < 90:
		记录.U动作 = "ratelimited"
		记录.U类别 = "ratelimit"
		记录.U规则ID = "ratelimit"
		记录.U状态 = 429
	default:
		记录.U动作 = "allow"
		记录.U类别 = ""
		记录.U规则ID = ""
		记录.U状态 = 200
	}
	头, _ := json.Marshal(map[string][]string{
		"Host":       {记录.USite},
		"User-Agent": {记录.U用户代理},
	})
	记录.U请求头 = string(头)
	if 攻击.方法 == "POST" && 攻击.载荷 != "" {
		记录.U请求体 = string(检查管线.U脱敏体(攻击.内容类型, []byte(攻击.载荷)))
	}
	记录.U匹配 = 匹配摘要(攻击.类别, "query", 攻击.查询)
	return 记录
}

func 延迟(随机源 *rand.Rand) int64 {
	掷点 := 随机源.Intn(100)
	switch {
	case 掷点 < 70:
		return int64(1 + 随机源.Intn(6))
	case 掷点 < 95:
		return int64(8 + 随机源.Intn(40))
	default:
		return int64(50 + 随机源.Intn(350))
	}
}

func 加权正常(随机源 *rand.Rand) 正常请求 {
	总权重 := 0
	for _, 项 := range 正常请求池 {
		总权重 += 项.权重
	}
	目标 := 随机源.Intn(总权重)
	for _, 项 := range 正常请求池 {
		if 目标 < 项.权重 {
			return 项
		}
		目标 -= 项.权重
	}
	return 正常请求池[0]
}

func 选择站点(随机源 *rand.Rand, 站点 []string) string {
	if len(站点) == 1 {
		return 站点[0]
	}
	掷点 := 随机源.Intn(100)
	switch {
	case 掷点 < 60:
		return 站点[0]
	case 掷点 < 85 && len(站点) > 1:
		return 站点[1]
	default:
		return 站点[len(站点)-1]
	}
}

func 构建地址池(随机源 *rand.Rand) []string {
	地址 := make([]string, 0, 500)
	for 序号 := 0; 序号 < 400; 序号++ {
		第三段 := 1 + 随机源.Intn(3)
		地址 = append(地址, fmt.Sprintf("203.0.113.%d", 第三段*60+随机源.Intn(60)))
	}
	for 序号 := 0; 序号 < 100; 序号++ {
		地址 = append(地址, fmt.Sprintf("2001:db8:%x::%x", 1+随机源.Intn(4), 1+随机源.Intn(65535)))
	}
	return 地址
}

func 选择地址(随机源 *rand.Rand, 地址池 []string) string {
	if 随机源.Intn(100) < 78 {
		return 地址池[随机源.Intn(400)]
	}
	return 地址池[400+随机源.Intn(100)]
}

func 匹配摘要(类别, 名称, 值 string) string {
	目标 := 检查管线.U目标{U类别: 类别, U名称: 名称, U值: 值}
	if 数据, 错误 := json.Marshal(目标); 错误 == nil && len(数据) <= 检查管线.U匹配列上限 {
		return string(数据)
	}
	return 检查管线.U截断文本(类别+"."+名称, 检查管线.U匹配列上限)
}
