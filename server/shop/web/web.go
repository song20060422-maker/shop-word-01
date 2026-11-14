// Package web 内置页面（单页，随二进制发布，无需前端构建）：
//
// buyer.html 买家端与模拟支付渠道页（独立 Gin 服务使用），
// ops.html / warehouse.html / cs.html 后台管理独立页（运营/仓库/客服，
// SHOP·OS 原型深色风格，经本服务 /gva/* 反向代理调用 GVA 管理端接口），
// index.html 集成版全功能页（买家/渠道/仓库/客服/运营 五个页签同页），挂在根路径 "/"。
package web

import "embed"

//go:embed buyer.html index.html ops.html warehouse.html cs.html

var files embed.FS

// Index 返回集成版全功能页 HTML（五个页签同页，挂在根路径）。

func Index() ([]byte, error) {
	return files.ReadFile("index.html")
}

// Buyer 返回买家端页面 HTML。

func Buyer() ([]byte, error) {
	return files.ReadFile("buyer.html")
}

// Ops 返回运营核查独立页面 HTML。

func Ops() ([]byte, error) {
	return files.ReadFile("ops.html")
}

// Warehouse 返回仓库发货独立页面 HTML。

func Warehouse() ([]byte, error) {
	return files.ReadFile("warehouse.html")
}

// CS 返回客服退款独立页面 HTML。

func CS() ([]byte, error) {
	return files.ReadFile("cs.html")
}
