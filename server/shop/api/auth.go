package api

import (
	"github.com/gin-gonic/gin"

	"shop/shop/middleware"
	"shop/shop/model/request"
	"shop/shop/service"
)

// AuthApi 登录与身份接口

type AuthApi struct {
	s *service.ServiceGroup
}

// Login 账号密码登录，返回 JWT 与用户信息（角色决定可见页面）

func (a AuthApi) Login(c *gin.Context) {
	var req request.Login
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	res, err := a.s.AuthService.Login(req.Username, req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// Me 当前登录用户

func (a AuthApi) Me(c *gin.Context) {
	claims := middleware.ClaimsOf(c)
	user, err := a.s.AuthService.User(claims.UserID)
	if err != nil {
		fail(c, service.ErrLoginFailed)
		return
	}
	ok(c, user)
}

// Users 账号列表（页面登录快捷入口用，不返回密码）

func (a AuthApi) Users(c *gin.Context) {
	list, err := a.s.AuthService.Users(c.Query("role"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}
