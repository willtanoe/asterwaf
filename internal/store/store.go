package 存储

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	日志队列容量 = 4096
	日志批量大小 = 256
	日志批量间隔 = time.Second
	关闭排空时间 = 5 * time.Second
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// U记录 是请求日志的内存形态，由异步写协程批量落库。
type U记录 struct {
	U请求ID  string
	U时间    int64
	USite  string
	U方法    string
	U路径    string
	U查询    string
	U状态    int
	U动作    string
	U类别    string
	U规则ID  string
	U影子    bool
	U客户端IP string
	U用户代理  string
	U耗时毫秒  int64
	U请求头   string
	U请求体   string
	U匹配    string
	U种子    bool
}

// U存储 负责 SQLite 生命周期与日志队列。请求路径只做非阻塞入队。
type U存储 struct {
	数据库  *sql.DB
	日志   *slog.Logger
	队列   chan U记录
	丢弃   atomic.Int64
	关闭中  atomic.Bool
	停止   chan struct{}
	完成   chan struct{}
	保留停止 chan struct{}
	保留完成 chan struct{}
	一次   sync.Once
}

// U打开 建库、迁移并启动异步写协程。
func U打开(路径 string, 日志 *slog.Logger) (*U存储, error) {
	return 打开(路径, 日志队列容量, 日志)
}

func 打开(路径 string, 队列容量 int, 日志 *slog.Logger) (*U存储, error) {
	if 日志 == nil {
		日志 = slog.Default()
	}
	if 队列容量 <= 0 {
		return nil, errors.New("store: queue capacity must be positive")
	}
	if 目录 := filepath.Dir(路径); 目录 != "" && 目录 != "." {
		if 错误 := os.MkdirAll(目录, 0o700); 错误 != nil {
			return nil, fmt.Errorf("store: create data directory: %w", 错误)
		}
	}
	源, 错误 := 数据源(路径)
	if 错误 != nil {
		return nil, 错误
	}
	数据库, 错误 := sql.Open("sqlite", 源)
	if 错误 != nil {
		return nil, fmt.Errorf("store: open database: %w", 错误)
	}
	数据库.SetMaxOpenConns(4)
	数据库.SetMaxIdleConns(4)
	if 错误 := 数据库.PingContext(context.Background()); 错误 != nil {
		_ = 数据库.Close()
		return nil, fmt.Errorf("store: connect database: %w", 错误)
	}
	if 错误 := 迁移(数据库); 错误 != nil {
		_ = 数据库.Close()
		return nil, 错误
	}
	存储 := &U存储{
		数据库:  数据库,
		日志:   日志,
		队列:   make(chan U记录, 队列容量),
		停止:   make(chan struct{}),
		完成:   make(chan struct{}),
		保留停止: make(chan struct{}),
		保留完成: make(chan struct{}),
	}
	go 存储.写循环()
	go 存储.保留循环()
	return 存储, nil
}

// U加入 永不阻塞请求路径；队列满时丢弃并计数。
func (存储 *U存储) U加入(记录 U记录) {
	if 存储.关闭中.Load() {
		存储.丢弃.Add(1)
		return
	}
	select {
	case 存储.队列 <- 记录:
	default:
		存储.丢弃.Add(1)
	}
}

// U丢弃数 返回进程内被丢弃的日志行数。
func (存储 *U存储) U丢弃数() int64 {
	return 存储.丢弃.Load()
}

// Close 停止接收并排空队列；超时返回错误但不会阻塞进程退出。
func (存储 *U存储) Close(上下文 context.Context) error {
	存储.关闭中.Store(true)
	存储.一次.Do(func() {
		close(存储.停止)
		close(存储.保留停止)
	})
	for {
		select {
		case <-存储.完成:
			select {
			case <-存储.保留完成:
				return 存储.数据库.Close()
			case <-上下文.Done():
				return fmt.Errorf("store: retention shutdown timed out: %w", 上下文.Err())
			}
		case <-上下文.Done():
			return fmt.Errorf("store: log drain timed out: %w", 上下文.Err())
		}
	}
}

func (存储 *U存储) 写循环() {
	defer close(存储.完成)
	批次 := make([]U记录, 0, 日志批量大小)
	计时器 := time.NewTicker(日志批量间隔)
	defer 计时器.Stop()

	刷新 := func() {
		if len(批次) == 0 {
			return
		}
		存储.写批次(批次)
		批次 = 批次[:0]
	}

	for {
		select {
		case 记录 := <-存储.队列:
			批次 = append(批次, 记录)
			if len(批次) >= 日志批量大小 {
				刷新()
			}
		case <-计时器.C:
			刷新()
		case <-存储.停止:
			计时器.Stop()
			for {
				select {
				case 记录 := <-存储.队列:
					批次 = append(批次, 记录)
					if len(批次) >= 日志批量大小 {
						刷新()
					}
				default:
					刷新()
					return
				}
			}
		}
	}
}

func (存储 *U存储) 写批次(批次 []U记录) {
	if 错误 := 存储.写入事务(批次); 错误 != nil {
		存储.日志.Error("log batch write failed", "error", 错误, "rows", len(批次))
		存储.丢弃.Add(int64(len(批次)))
	}
}

// U直接写入 供 seed 使用：同一事务写日志与 rollup，不经过异步队列。
func (存储 *U存储) U直接写入(批次 []U记录) error {
	return 存储.写入事务(批次)
}

func (存储 *U存储) 写入事务(批次 []U记录) error {
	事务, 错误 := 存储.数据库.Begin()
	if 错误 != nil {
		return fmt.Errorf("store: begin log batch: %w", 错误)
	}
	defer func() { _ = 事务.Rollback() }()
	语句, 错误 := 事务.Prepare(`INSERT INTO logs (
		request_id, ts, site, method, path, query, status, action, category, rule_id,
		shadow, client_ip, user_agent, duration_ms, request_headers, request_body, matched, seed
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if 错误 != nil {
		return fmt.Errorf("store: prepare log insert: %w", 错误)
	}
	defer func() { _ = 语句.Close() }()
	for _, 记录 := range 批次 {
		影子 := 0
		if 记录.U影子 {
			影子 = 1
		}
		种子 := 0
		if 记录.U种子 {
			种子 = 1
		}
		if _, 错误 := 语句.Exec(
			记录.U请求ID, 记录.U时间, 记录.USite, 记录.U方法, 记录.U路径, 记录.U查询,
			记录.U状态, 记录.U动作, 记录.U类别, 记录.U规则ID, 影子, 记录.U客户端IP,
			记录.U用户代理, 记录.U耗时毫秒, 记录.U请求头, 记录.U请求体, 记录.U匹配, 种子,
		); 错误 != nil {
			return fmt.Errorf("store: insert log row: %w", 错误)
		}
		if 错误 := 写入聚合(事务, 记录); 错误 != nil {
			return 错误
		}
	}
	if 错误 := 事务.Commit(); 错误 != nil {
		return fmt.Errorf("store: commit log batch: %w", 错误)
	}
	return nil
}

// U包含种子数据 让服务启动时能大声提示正在使用合成数据库。
func (存储 *U存储) U包含种子数据(上下文 context.Context) bool {
	var 存在 int
	错误 := 存储.数据库.QueryRowContext(上下文, `SELECT EXISTS(SELECT 1 FROM logs WHERE seed = 1 LIMIT 1)`).Scan(&存在)
	return 错误 == nil && 存在 == 1
}

// 数据源 组装 modernc.org/sqlite 的连接串。路径必须先归一化为绝对路径：
// url.URL.String() 在 Path 非空时固定输出 "//"，相对路径的首段会被 SQLite
// 当作 URI authority（file://data/x.db 中的 "data"）而解析失败。
func 数据源(路径 string) (string, error) {
	绝对, 错误 := filepath.Abs(路径)
	if 错误 != nil {
		return "", fmt.Errorf("store: resolve database path: %w", 错误)
	}
	值 := url.URL{Scheme: "file", Path: 绝对}
	return 值.String() + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)", nil
}

func 迁移(数据库 *sql.DB) error {
	if _, 错误 := 数据库.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); 错误 != nil {
		return fmt.Errorf("store: create migrations table: %w", 错误)
	}
	var 当前 int
	if 错误 := 数据库.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&当前); 错误 != nil {
		return fmt.Errorf("store: read schema version: %w", 错误)
	}
	条目, 错误 := migrationsFS.ReadDir("migrations")
	if 错误 != nil {
		return fmt.Errorf("store: read migrations: %w", 错误)
	}
	文件列表 := make([]string, 0, len(条目))
	for _, 条目 := range 条目 {
		if !条目.IsDir() && strings.HasSuffix(条目.Name(), ".sql") {
			文件列表 = append(文件列表, 条目.Name())
		}
	}
	sort.Strings(文件列表)
	for _, 文件名 := range 文件列表 {
		版本, 错误 := 版本号(文件名)
		if 错误 != nil {
			return 错误
		}
		if 版本 <= 当前 {
			continue
		}
		内容, 错误 := migrationsFS.ReadFile("migrations/" + 文件名)
		if 错误 != nil {
			return fmt.Errorf("store: read migration %s: %w", 文件名, 错误)
		}
		事务, 错误 := 数据库.Begin()
		if 错误 != nil {
			return fmt.Errorf("store: begin migration %s: %w", 文件名, 错误)
		}
		if _, 错误 := 事务.Exec(string(内容)); 错误 != nil {
			_ = 事务.Rollback()
			return fmt.Errorf("store: apply migration %s: %w", 文件名, 错误)
		}
		if _, 错误 := 事务.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`,
			版本, time.Now().UnixMilli()); 错误 != nil {
			_ = 事务.Rollback()
			return fmt.Errorf("store: record migration %s: %w", 文件名, 错误)
		}
		if 错误 := 事务.Commit(); 错误 != nil {
			return fmt.Errorf("store: commit migration %s: %w", 文件名, 错误)
		}
		当前 = 版本
	}
	return nil
}

func 版本号(文件名 string) (int, error) {
	下划线 := strings.IndexByte(文件名, '_')
	if 下划线 <= 0 {
		return 0, fmt.Errorf("store: migration %s has no numeric prefix", 文件名)
	}
	版本, 错误 := strconv.Atoi(文件名[:下划线])
	if 错误 != nil {
		return 0, fmt.Errorf("store: migration %s has invalid version: %w", 文件名, 错误)
	}
	return 版本, nil
}
