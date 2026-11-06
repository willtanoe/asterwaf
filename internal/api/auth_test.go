package 管理接口

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	配置 "git.asteriaxlabs.com/willtanoe/asterwaf/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func Test首次运行密码文件(t *testing.T) {
	目录 := t.TempDir()
	配置值 := 配置.U默认配置()
	配置值.U数据库 = filepath.Join(目录, "waf.db")
	配置值.U认证.U密码哈希 = ""

	凭据, 错误 := U准备凭据(配置值)
	if 错误 != nil {
		t.Fatalf("prepare: %v", 错误)
	}
	路径 := filepath.Join(目录, 首运行密码文件名)
	数据, 错误 := os.ReadFile(路径)
	if 错误 != nil {
		t.Fatalf("read password file: %v", 错误)
	}
	密码 := string(数据[:len(数据)-1])
	if len(密码) != 密码长度 {
		t.Fatalf("password length=%d", len(密码))
	}
	信息, 错误 := os.Stat(路径)
	if 错误 != nil {
		t.Fatalf("stat: %v", 错误)
	}
	if 权限 := 信息.Mode().Perm(); 权限 != 0o600 {
		t.Fatalf("password file mode=%o", 权限)
	}
	if 错误 := bcrypt.CompareHashAndPassword(凭据.U哈希, []byte(密码)); 错误 != nil {
		t.Fatalf("generated hash does not match file: %v", 错误)
	}

	再次, 错误 := U准备凭据(配置值)
	if 错误 != nil {
		t.Fatalf("second prepare: %v", 错误)
	}
	if 错误 := bcrypt.CompareHashAndPassword(再次.U哈希, []byte(密码)); 错误 != nil {
		t.Fatalf("existing password file was not reused: %v", 错误)
	}
}

func Test配置哈希优先(t *testing.T) {
	哈希, 错误 := bcrypt.GenerateFromPassword([]byte("configured-secret"), bcrypt.MinCost)
	if 错误 != nil {
		t.Fatalf("hash: %v", 错误)
	}
	配置值 := 配置.U默认配置()
	配置值.U数据库 = filepath.Join(t.TempDir(), "waf.db")
	配置值.U认证.U密码哈希 = string(哈希)
	凭据, 错误 := U准备凭据(配置值)
	if 错误 != nil {
		t.Fatalf("prepare: %v", 错误)
	}
	if 错误 := bcrypt.CompareHashAndPassword(凭据.U哈希, []byte("configured-secret")); 错误 != nil {
		t.Fatalf("configured hash rejected: %v", 错误)
	}
	if _, 错误 := os.Stat(filepath.Join(filepath.Dir(配置值.U数据库), 首运行密码文件名)); !os.IsNotExist(错误) {
		t.Fatal("password file was created despite configured hash")
	}
}

func Test非法哈希被拒绝(t *testing.T) {
	配置值 := 配置.U默认配置()
	配置值.U数据库 = filepath.Join(t.TempDir(), "waf.db")
	配置值.U认证.U密码哈希 = "not-a-bcrypt-hash"
	if _, 错误 := U准备凭据(配置值); 错误 == nil {
		t.Fatal("invalid bcrypt hash accepted")
	}
}

func Test来源校验(t *testing.T) {
	测试用例 := []struct {
		名称 string
		方法 string
		主机 string
		来源 string
		允许 bool
	}{
		{"GET ignores origin", "GET", "admin.test:9090", "http://evil.test", true},
		{"no origin allowed", "POST", "admin.test:9090", "", true},
		{"matching origin", "POST", "admin.test:9090", "http://admin.test:9090", true},
		{"matching https origin", "POST", "admin.test", "https://admin.test", true},
		{"mismatched host", "POST", "admin.test:9090", "http://evil.test", false},
		{"mismatched port", "POST", "admin.test:9090", "http://admin.test:8080", false},
		{"null origin", "POST", "admin.test:9090", "null", false},
		{"malformed origin", "POST", "admin.test:9090", "http://[::1", false},
	}
	for _, 用例 := range 测试用例 {
		t.Run(用例.名称, func(t *testing.T) {
			请求 := httptest.NewRequest(用例.方法, "http://"+用例.主机+"/api/login", nil)
			请求.Host = 用例.主机
			if 用例.来源 != "" {
				请求.Header.Set("Origin", 用例.来源)
			}
			if 实际 := U来源允许(请求); 实际 != 用例.允许 {
				t.Fatalf("allowed=%v, want %v", 实际, 用例.允许)
			}
		})
	}
}
