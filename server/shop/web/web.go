// Package web 内置页面（单页，随二进制发布，无需前端构建）：
// buyer.html 买家端与模拟支付渠道页（独立 Gin 服务使用），
// index.html 历史集成版原型页（保留归档，不再挂载）。
package web

import "embed"

//go:embed buyer.html index.html
var files embed.FS

// Index 返回历史集成版原型页 HTML（保留，不再对外挂载）。
func Index() ([]byte, error) {
	return files.ReadFile("index.html")
}

// Buyer 返回买家端页面 HTML。
func Buyer() ([]byte, error) {
	return files.ReadFile("buyer.html")
}
