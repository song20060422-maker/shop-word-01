// Package web 内置可操作原型页面（单页，随二进制发布，无需前端构建）。
package web

import "embed"

//go:embed index.html
var files embed.FS

// Index 返回原型页面 HTML。
func Index() ([]byte, error) {
	return files.ReadFile("index.html")
}
