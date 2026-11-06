package 前端

import "embed"

// DistFS 由 make web-build 生成，名字保持英文以满足 go:embed 约束。
//go:embed dist
var DistFS embed.FS
