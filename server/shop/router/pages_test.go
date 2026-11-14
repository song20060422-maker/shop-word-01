package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"shop/shop/config"
	"shop/shop/service"
)

// newServerWithCfg 与 newServer 相同，但允许用例自定义配置（如 system.admin-target）。
func newServerWithCfg(t *testing.T, cfg config.Config) *gin.Engine {
	t.Helper()
	resetTables(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ch := service.NewSimulatedChannel(true)
	g := service.NewServiceGroup(service.Deps{DB: testDB, Now: time.Now, Cfg: cfg, Channel: ch})
	InitShopRouter(r.Group(""), g, cfg, time.Now)
	return r
}

// TestAdminPagesServed 买家端页随服务托管；根路径重定向；后台三页已迁移 GVA 管理端（404 提示去向）。
func TestAdminPagesServed(t *testing.T) {
	r, _ := newServer(t, true)
	// 买家页：/shop、/shop/ 直接返回 HTML
	for _, p := range []string{"/shop", "/shop/"} {
		w := doJSON(r, "GET", p, "", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应 200, got %d", p, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("%s Content-Type 应为 html: %s", p, ct)
		}
		if !strings.Contains(w.Body.String(), "买家") {
			t.Fatalf("%s 页面缺少标记", p)
		}
	}
	// 根路径与 /index.html 重定向到 /shop（历史入口兼容）
	for _, p := range []string{"/", "/index.html"} {
		w := doJSON(r, "GET", p, "", "", nil)
		if w.Code != http.StatusFound || w.Header().Get("Location") != "/shop" {
			t.Fatalf("%s 应 302 到 /shop, got %d %q", p, w.Code, w.Header().Get("Location"))
		}
	}
	// 后台三页已迁移：返回 404 + 明确去向提示，不做静默 404
	for _, c := range []struct{ path, marker string }{
		{"/ops", "运营核查"},
		{"/warehouse", "仓库发货"},
		{"/cs", "客服退款"},
	} {
		w := doJSON(r, "GET", c.path, "", "", nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s 应 404, got %d", c.path, w.Code)
		}
		if !strings.Contains(w.Body.String(), "GVA 管理端") || !strings.Contains(w.Body.String(), c.marker) {
			t.Fatalf("%s 提示缺少去向说明: %s", c.path, w.Body.String())
		}
	}
}

// TestGVAReverseProxy 后台独立页通过 /gva/* 访问管理端：剥掉 /gva 前缀并透传 x-token。
func TestGVAReverseProxy(t *testing.T) {
	var gotPath, gotToken, gotBody, gotMethod string
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		gotToken = req.Header.Get("x-token")
		b, _ := io.ReadAll(req.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"token":"T"},"msg":"ok"}`))
	}))
	defer admin.Close()

	cfg := testCfg
	cfg.System.AdminTarget = admin.URL
	r := newServerWithCfg(t, cfg)
	// ReverseProxy 需要真实 ResponseWriter（httptest.ResponseRecorder 缺 CloseNotifier/Flusher），
	// 因此这里起真实监听再请求。
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/gva/base/login", "application/json", strings.NewReader(`{"username":"admin","password":"123456"}`))
	if err != nil {
		t.Fatalf("反代登录请求失败: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"code":0`) {
		t.Fatalf("反代登录应透传响应: %d %s", resp.StatusCode, string(body))
	}
	if gotPath != "/base/login" || gotMethod != "POST" {
		t.Fatalf("/gva 前缀未剥离: path=%s method=%s", gotPath, gotMethod)
	}
	if !strings.Contains(gotBody, "admin") {
		t.Fatalf("请求体未透传: %s", gotBody)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/gva/shop/ops/summary", nil)
	req.Header.Set("x-token", "TOKEN-X")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("反代查询失败: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("反代查询应 200: %d", resp2.StatusCode)
	}
	if gotPath != "/shop/ops/summary" {
		t.Fatalf("管理端接口路径错误: %s", gotPath)
	}
	if gotToken != "TOKEN-X" {
		t.Fatalf("x-token 未透传: %q", gotToken)
	}
}

// TestGVAReverseProxyInvalidTarget 目标地址非法时不注册反代，服务不崩溃。
func TestGVAReverseProxyInvalidTarget(t *testing.T) {
	cfg := testCfg
	cfg.System.AdminTarget = "://bad-target"
	r := newServerWithCfg(t, cfg)

	if w := doJSON(r, "GET", "/shop", "", "", nil); w.Code != http.StatusOK {
		t.Fatalf("买家页仍应可访问: %d", w.Code)
	}
	if w := doJSON(r, "GET", "/gva/health", "", "", nil); w.Code == http.StatusOK {
		t.Fatalf("非法目标不应注册反代: %d", w.Code)
	}
}
