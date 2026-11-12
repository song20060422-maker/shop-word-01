package router

import (
	"net/http"
	"testing"
)

// TestDemoModeGatesTestHooks 安全整改回归：
// 测试钩子（渠道退款开关、强制过期、手动关单）与账号列表、演示重置
// 仅在 shop.demo-mode=true 时可用；生产配置下必须全部 403。
func TestDemoModeGatesTestHooks(t *testing.T) {
	hooks := []struct {
		method, path, body string
	}{
		{"POST", "/api/test/expire-orders", ""},
		{"POST", "/api/pay/channel-mode", `{"refund_result":"SUCCESS"}`},
		{"POST", "/api/jobs/close-expired", ""},
		{"GET", "/api/users", ""},
		{"POST", "/api/demo/reset", ""},
	}

	prod, _ := newServer(t, false) // 生产配置：demo-mode=false
	for _, h := range hooks {
		rr := doJSON(prod, h.method, h.path, h.body, "", nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("生产配置下 %s %s 应 403，实际 %d: %s", h.method, h.path, rr.Code, rr.Body.String())
		}
	}

	demo, _ := newServer(t, true) // 演示配置：demo-mode=true
	for _, h := range hooks {
		rr := doJSON(demo, h.method, h.path, h.body, "", nil)
		if rr.Code >= 400 {
			t.Fatalf("演示配置下 %s %s 应可用，实际 %d: %s", h.method, h.path, rr.Code, rr.Body.String())
		}
	}
}

// TestBuyerRoleGate 安全整改回归（I-02）：
// 买家区路由仅 buyer 角色可用，员工角色登录后调用必须 403。
func TestBuyerRoleGate(t *testing.T) {
	r, _ := newServer(t, true)

	opsToken := login(t, r, "ops1")
	whToken := login(t, r, "wh1")
	csToken := login(t, r, "cs1")
	buyerToken := login(t, r, "buyer1")

	for _, tc := range []struct {
		name, token string
	}{
		{"ops", opsToken},
		{"warehouse", whToken},
		{"cs", csToken},
	} {
		rr := doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-A","quantity":1}`, tc.token, nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s 角色下单应 403，实际 %d: %s", tc.name, rr.Code, rr.Body.String())
		}
		rr = doJSON(r, "GET", "/api/orders", "", tc.token, nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("%s 角色查买家订单应 403，实际 %d", tc.name, rr.Code)
		}
	}

	rr := doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-A","quantity":1}`, buyerToken, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("买家下单应成功: %d %s", rr.Code, rr.Body.String())
	}
}
