package service

import (
	"strings"
	"time"

	"shop/shop/model"
	"shop/shop/model/response"
	"shop/shop/utils"
)

// AuthService 账号与令牌

type AuthService struct {
	deps Deps
}

// Login 校验账号密码并签发 JWT

func (s AuthService) Login(username, password string) (response.LoginResult, error) {
	var out response.LoginResult
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return out, ErrLoginFailed
	}
	var user model.User
	if err := s.deps.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return out, ErrLoginFailed
	}
	if user.PasswordHash == "" || !utils.VerifyPassword(password, user.PasswordHash) {
		return out, ErrLoginFailed
	}
	now := s.deps.Now()
	expire := now.Add(time.Duration(s.deps.Cfg.Jwt.ExpireHours) * time.Hour)
	if s.deps.Cfg.Jwt.ExpireHours <= 0 {
		expire = now.Add(12 * time.Hour)
	}
	token, err := utils.SignJWT(s.deps.Cfg.Jwt.Secret, utils.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		IssuedAt: now.Unix(),
		ExpireAt: expire.Unix(),
	})
	if err != nil {
		return out, err
	}
	return response.LoginResult{Token: token, User: user, ExpireAt: expire.Format("2006-01-02 15:04:05")}, nil
}

// User 按 ID 查账号

func (s AuthService) User(id uint64) (model.User, error) {
	var user model.User
	if err := s.deps.DB.Where("id = ?", id).First(&user).Error; err != nil {
		return user, ErrLoginFailed
	}
	return user, nil
}

// Users 按角色列出账号（运营核查页可查看角色分布）

func (s AuthService) Users(role string) ([]model.User, error) {
	var users []model.User
	q := s.deps.DB
	if role != "" {
		q = q.Where("role = ?", role)
	}
	err := q.Order("id asc").Find(&users).Error
	return users, err
}
