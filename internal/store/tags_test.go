package 存储

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 序列化键必须是显式英文 tag；这里防止后续新增字段漏写 tag 或写成中文。
func Test序列化字段都有英文tag(t *testing.T) {
	根 := filepath.Join("..", "..")
	包目录 := []string{"internal/accesslist", "internal/api", "internal/config", "internal/engine", "internal/pipeline", "internal/ratelimit", "internal/store", "internal/seed"}
	需要tag := map[string]bool{"yaml": true, "json": true}
	检查 := 0
	for _, 目录 := range 包目录 {
		文件集 := token.NewFileSet()
		条目, 错误 := os.ReadDir(filepath.Join(根, 目录))
		if 错误 != nil {
			t.Fatalf("read %s: %v", 目录, 错误)
		}
		for _, 条目 := range 条目 {
			if 条目.IsDir() || !strings.HasSuffix(条目.Name(), ".go") || strings.HasSuffix(条目.Name(), "_test.go") {
				continue
			}
			文件, 错误 := parser.ParseFile(文件集, filepath.Join(根, 目录, 条目.Name()), nil, 0)
			if 错误 != nil {
				t.Fatalf("parse %s: %v", 条目.Name(), 错误)
			}
			ast.Inspect(文件, func(节点 ast.Node) bool {
				结构, 是 := 节点.(*ast.TypeSpec)
				if !是 {
					return true
				}
				结构体, 是 := 结构.Type.(*ast.StructType)
				if !是 {
					return true
				}
				for _, 字段 := range 结构体.Fields.List {
					if len(字段.Names) == 0 || 字段.Tag == nil {
						continue
					}
					tag := 字段.Tag.Value
					for 键 := range 需要tag {
						if !strings.Contains(tag, 键+`:"`) {
							continue
						}
						起始 := strings.Index(tag, 键+`:"`) + len(键) + 3
						结束 := strings.Index(tag[起始:], `"`)
						值 := tag[起始 : 起始+结束]
						值 = strings.Split(值, ",")[0]
						if 值 == "" || 值 == "-" {
							continue
						}
						检查++
						for _, 字符 := range 值 {
							if 字符 > 127 {
								t.Fatalf("%s.%s has non-ASCII %s key %q", 结构.Name.Name, 字段.Names[0].Name, 键, 值)
							}
						}
					}
				}
				return true
			})
		}
	}
	if 检查 < 50 {
		t.Fatalf("only %d tagged keys inspected; parser likely missed files", 检查)
	}
	t.Logf("inspected %d serialized keys", 检查)
}
