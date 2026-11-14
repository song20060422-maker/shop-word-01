// Package middleware 提供 JWT 鉴权与角色校验中间件。
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"shop/shop/model"
	"shop/shop/model/response"
	"shop/shop/utils"
)

// claimsKey gin.Context 中存放已验签 Claims 的键名，RequireRole 从这里取角色。

const claimsKey = "shopClaims"

// JWT 校验令牌，支持 Authorization: Bearer 与 token Cookie（页面演示免传头）。

func JWT(secret string, now func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		}
		if token == "" {
			if ck, err := c.Cookie("token"); err == nil {
				token = ck
			}
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(401, "请先登录"))
			return
		}
		claims, err := utils.ParseJWT(secret, token, now())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(401, "登录状态已失效，请重新登录"))
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

// RequireRole 限制角色访问；买家越权访问员工接口时返回明确的 403。

func RequireRole(roles ...string) gin.HandlerFunc {
	allow := make(map[string]bool, len(roles))
	for _, r := range roles {
		allow[r] = true
	}
	return func(c *gin.Context) {
		claims := ClaimsOf(c)
		if !allow[claims.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(2002, "当前角色无权执行该操作"))
			return
		}
		c.Next()
	}
}

// ClaimsOf 读取当前请求的令牌载荷

func ClaimsOf(c *gin.Context) utils.Claims {
	if v, ok := c.Get(claimsKey); ok {
		if claims, ok := v.(utils.Claims); ok {
			return claims
		}
	}
	return utils.Claims{}
}

// IsBuyer 便于业务层判定

func IsBuyer(c *gin.Context) bool { return ClaimsOf(c).Role == model.RoleBuyer }
