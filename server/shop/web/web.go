// Package web 内置买家端页面（单页，随二进制发布，无需前端构建）：
//
// buyer.html 买家端与模拟支付渠道页，由独立 Gin 服务（:8090）挂在 /shop。
// 运营/仓库/客服后台页面不在此处：统一由 GVA 管理端 Vue（web/src/view/shop/*.vue）
// 提供，接口走 GVA /shop/**（登录 + Casbin），避免两套后台页面重复维护。
package web

import "embed"

//go:embed buyer.html

var files embed.FS

// Buyer 返回买家端页面 HTML。

func Buyer() ([]byte, error) {
	return files.ReadFile("buyer.html")
}
