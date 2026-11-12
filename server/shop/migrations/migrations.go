// Package migrations 以 embed 方式内置建表与种子脚本，避免依赖运行时工作目录。
package migrations

import "embed"

// FS 内置 migrations 目录下的全部 SQL 脚本，按文件名顺序执行。
//
//go:embed *.sql
var FS embed.FS
