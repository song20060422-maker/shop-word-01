package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"shop/shop/model"
	"shop/shop/utils"
)

func newCtx(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, w
}

func TestJWTBranches(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 11, 12, 10, 0, 0, 0, time.Local) }
	secret := "unit-secret"
	token, err := utils.SignJWT(secret, utils.Claims{
		UserID: 7, Username: "buyer1", Role: model.RoleBuyer,
		ExpireAt: now().Add(time.Hour).Unix(), IssuedAt: now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 缺少令牌
	c, w := newCtx("GET", "/api/me")
	JWT(secret, now)(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无令牌应 401: %d", w.Code)
	}

	// 伪造令牌
	c, w = newCtx("GET", "/api/me")
	c.Request.Header.Set("Authorization", "Bearer forged")
	JWT(secret, now)(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("伪造令牌应 401: %d", w.Code)
	}

	// Bearer 令牌
	c, w = newCtx("GET", "/api/me")
	c.Request.Header.Set("Authorization", "Bearer "+token)
	JWT(secret, now)(c)
	if claims := ClaimsOf(c); claims.UserID != 7 || claims.Role != model.RoleBuyer {
		t.Fatalf("令牌载荷错误: %+v", claims)
	}

	// Cookie 令牌
	c, w = newCtx("GET", "/api/me")
	c.Request.AddCookie(&http.Cookie{Name: "token", Value: token})
	JWT(secret, now)(c)
	if claims := ClaimsOf(c); claims.Username != "buyer1" {
		t.Fatalf("Cookie 令牌未生效: %+v", claims)
	}
}

func TestRequireRoleAndHelpers(t *testing.T) {
	// 无 claims 时拒绝
	c, w := newCtx("GET", "/api/ops/summary")
	RequireRole(model.RoleOps)(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("无角色应 403: %d", w.Code)
	}

	// 角色不符
	c, w = newCtx("GET", "/api/ops/summary")
	c.Set(claimsKey, utils.Claims{UserID: 1, Role: model.RoleBuyer})
	RequireRole(model.RoleOps)(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("角色不符应 403: %d", w.Code)
	}

	// 角色命中（使用新的记录器，避免复用上一次的 403 结果）
	c, w2 := newCtx("GET", "/api/ops/summary")
	c.Set(claimsKey, utils.Claims{UserID: 1, Role: model.RoleOps})
	RequireRole(model.RoleOps)(c)
	if w2.Code != http.StatusOK {
		t.Fatalf("角色命中不应被拦截: %d", w2.Code)
	}

	// IsBuyer 正例
	c, _ = newCtx("GET", "/api/me")
	c.Set(claimsKey, utils.Claims{UserID: 1, Role: model.RoleBuyer})
	if !IsBuyer(c) {
		t.Fatal("买家判定失败")
	}

	// 空上下文 ClaimsOf 返回零值
	c, _ = newCtx("GET", "/api/me")
	if claims := ClaimsOf(c); claims.UserID != 0 {
		t.Fatalf("空 claims 应返回零值: %+v", claims)
	}
}
