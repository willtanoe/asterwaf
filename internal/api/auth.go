package 管理接口

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	"golang.org/x/crypto/bcrypt"
)

const (
	首运行密码文件名 = "first-run-admin-password"
	密码长度     = 20
)

var 密码字母 = []byte("abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789")

type U凭据 struct {
	U哈希   []byte
	U文件路径 string
}

// U准备凭据 优先使用配置哈希；否则读取或生成首次运行密码文件。
// 密码本体绝不进入日志，文件路径只写 stderr。
func U准备凭据(配置值 配置.U配置) (U凭据, error) {
	if 配置值.U认证.U密码哈希 != "" {
		if _, 错误 := bcrypt.Cost([]byte(配置值.U认证.U密码哈希)); 错误 != nil {
			return U凭据{}, fmt.Errorf("config: auth.password_hash is not a valid bcrypt hash: %w", 错误)
		}
		return U凭据{U哈希: []byte(配置值.U认证.U密码哈希)}, nil
	}

	目录 := filepath.Dir(配置值.U数据库)
	路径 := filepath.Join(目录, 首运行密码文件名)
	if 错误 := os.MkdirAll(目录, 0o700); 错误 != nil {
		return U凭据{}, fmt.Errorf("create data directory: %w", 错误)
	}
	密码, 错误 := 读取密码文件(路径)
	if errors.Is(错误, os.ErrNotExist) {
		密码, 错误 = 创建密码文件(路径)
	}
	if 错误 != nil {
		return U凭据{}, 错误
	}
	if 错误 := os.Chmod(路径, 0o600); 错误 != nil {
		return U凭据{}, fmt.Errorf("chmod password file: %w", 错误)
	}
	哈希, 错误 := bcrypt.GenerateFromPassword([]byte(密码), bcrypt.DefaultCost)
	if 错误 != nil {
		return U凭据{}, fmt.Errorf("hash first-run password: %w", 错误)
	}
	fmt.Fprintf(os.Stderr, "asterwaf: admin password file: %s\n", 路径)
	return U凭据{U哈希: 哈希, U文件路径: 路径}, nil
}

func 读取密码文件(路径 string) (string, error) {
	数据, 错误 := os.ReadFile(路径)
	if 错误 != nil {
		return "", 错误
	}
	密码 := strings.TrimSpace(string(数据))
	if 密码 == "" {
		return "", fmt.Errorf("first-run password file %s is empty", 路径)
	}
	return 密码, nil
}

func 创建密码文件(路径 string) (string, error) {
	密码, 错误 := 生成密码()
	if 错误 != nil {
		return "", 错误
	}
	文件, 错误 := os.OpenFile(路径, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(错误, os.ErrExist) {
		return 读取密码文件(路径)
	}
	if 错误 != nil {
		return "", fmt.Errorf("create first-run password file: %w", 错误)
	}
	if _, 错误 := 文件.WriteString(密码 + "\n"); 错误 != nil {
		_ = 文件.Close()
		_ = os.Remove(路径)
		return "", fmt.Errorf("write first-run password file: %w", 错误)
	}
	if 错误 := 文件.Close(); 错误 != nil {
		return "", fmt.Errorf("close first-run password file: %w", 错误)
	}
	return 密码, nil
}

func 生成密码() (string, error) {
	结果 := make([]byte, 密码长度)
	上限 := big.NewInt(int64(len(密码字母)))
	for 序号 := range 结果 {
		下标, 错误 := rand.Int(rand.Reader, 上限)
		if 错误 != nil {
			return "", fmt.Errorf("generate admin password: %w", 错误)
		}
		结果[序号] = 密码字母[下标.Int64()]
	}
	return string(结果), nil
}

// U来源允许 对浏览器发起的写请求做同源校验；无 Origin 的脚本客户端按规范放行。
func U来源允许(请求 *http.Request) bool {
	switch 请求.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	来源 := 请求.Header.Get("Origin")
	if 来源 == "" {
		return true
	}
	解析, 错误 := url.Parse(来源)
	if 错误 != nil {
		return false
	}
	if 解析.Scheme != "http" && 解析.Scheme != "https" {
		return false
	}
	return strings.EqualFold(解析.Host, 请求.Host)
}
