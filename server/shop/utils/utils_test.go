package utils

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHelpers(t *testing.T) {
	hash, err := HashPassword("shop123")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("shop123", hash) {
		t.Fatal("正确密码校验失败")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("错误密码不应通过")
	}
	// 非法编码格式
	for _, bad := range []string{"", "plain", "x$1$aa$bb", "pbkdf2$0$aa$bb", "pbkdf2$1$zz$bb"} {
		if VerifyPassword("shop123", bad) {
			t.Fatalf("非法哈希不应通过: %s", bad)
		}
	}
}

func TestJWTAndSign(t *testing.T) {
	secret := "unit-secret"
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.Local)
	claims := Claims{UserID: 9, Username: "buyer1", Role: "buyer",
		ExpireAt: now.Add(time.Hour).Unix(), IssuedAt: now.Unix()}
	token, err := SignJWT(secret, claims)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseJWT(secret, token, now)
	if err != nil || got.UserID != 9 || got.Role != "buyer" {
		t.Fatalf("令牌解析失败: %v %+v", err, got)
	}
	// 过期令牌
	if _, err := ParseJWT(secret, token, now.Add(2*time.Hour)); err == nil {
		t.Fatal("过期令牌应拒绝")
	}
	// 篡改签名
	parts := strings.Split(token, ".")
	if _, err := ParseJWT(secret, parts[0]+"."+parts[1]+".deadbeef", now); err == nil {
		t.Fatal("篡改签名应拒绝")
	}
	// 结构非法
	if _, err := ParseJWT(secret, "a.b", now); err == nil {
		t.Fatal("非法结构应拒绝")
	}
	if _, err := ParseJWT(secret, "a.b.c", now); err == nil {
		t.Fatal("非法载荷应拒绝")
	}

	// 支付签名对字段顺序敏感
	s1 := PaySign(secret, "SO1", "TXN1", 100)
	s2 := PaySign(secret, "SO1", "TXN1", 100)
	if s1 != s2 || len(s1) != 64 {
		t.Fatal("支付签名不稳定")
	}
	if s1 == PaySign(secret, "SO2", "TXN1", 100) {
		t.Fatal("不同订单签名不应相同")
	}
}

func TestNumberAndFormatHelpers(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.Local)
	orderNo := NewOrderNo(now)
	if !strings.HasPrefix(orderNo, "SH") || len(orderNo) != 20 {
		t.Fatalf("订单号格式错误: %s", orderNo)
	}
	if !strings.HasPrefix(NewRefundNo(now), "RF") {
		t.Fatal("退款单号格式错误")
	}
	if Fen(0) == "" || Fen(12345) == "" || Fen(-100) == "" {
		t.Fatal("金额格式化不应为空")
	}
	if !strings.Contains(Fen(12345), "123") {
		t.Fatalf("金额格式化错误: %s", Fen(12345))
	}
}
