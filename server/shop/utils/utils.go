// Package utils 提供密码哈希、JWT 与业务单号生成等基础工具。
package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const pbkdf2Iter = 4096

// HashPassword 生成 pbkdf2$iter$salt$hash 形式的密码摘要。
func HashPassword(plain string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := pbkdf2.Key([]byte(plain), salt, pbkdf2Iter, 32, sha256.New)
	return fmt.Sprintf("pbkdf2$%d$%s$%s", pbkdf2Iter, hex.EncodeToString(salt), hex.EncodeToString(sum)), nil
}

// VerifyPassword 校验明文密码与摘要是否匹配，比较过程使用常量时间。
func VerifyPassword(plain, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got := pbkdf2.Key([]byte(plain), salt, iter, len(want), sha256.New)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---------- JWT（HS256，标准库实现，避免额外依赖） ----------

// Claims JWT 载荷
type Claims struct {
	UserID   uint64 `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	ExpireAt int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
}

// ErrInvalidToken 令牌非法
var ErrInvalidToken = errors.New("令牌无效或已过期")

func b64(buf []byte) string { return base64.RawURLEncoding.EncodeToString(buf) }

// SignJWT 签发令牌
func SignJWT(secret string, c Claims) (string, error) {
	header := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadJSON, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := b64(payloadJSON)
	signing := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	return signing + "." + b64(mac.Sum(nil)), nil
}

// ParseJWT 解析并校验令牌
func ParseJWT(secret, token string, now time.Time) (Claims, error) {
	var claims Claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, ErrInvalidToken
	}
	signing := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return claims, ErrInvalidToken
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, ErrInvalidToken
	}
	if err := json.Unmarshal(body, &claims); err != nil {
		return claims, ErrInvalidToken
	}
	if claims.ExpireAt <= now.Unix() {
		return claims, ErrInvalidToken
	}
	return claims, nil
}

// ---------- 业务单号 ----------

const noCharset = "0123456789"

func randDigits(n int) string {
	buf := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			buf = append(buf, noCharset[0])
			continue
		}
		buf = append(buf, noCharset[v.Int64()])
	}
	return string(buf)
}

// NewOrderNo 生成 20 位订单号：SH + yyMMddHHmmss + 6 位随机数字。
func NewOrderNo(now time.Time) string {
	return "SH" + now.Format("060102150405") + randDigits(6)
}

// NewRefundNo 生成 20 位退款单号：RF + yyMMddHHmmss + 6 位随机数字。
func NewRefundNo(now time.Time) string {
	return "RF" + now.Format("060102150405") + randDigits(6)
}

// PaySign 模拟渠道通知签名：HMAC-SHA256(secret, orderNo|txn|amount)。
func PaySign(secret, orderNo, txn string, amount int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%s|%s|%d", orderNo, txn, amount)))
	return hex.EncodeToString(mac.Sum(nil))
}

// Fen 把分转换为人民币字符串，便于页面展示。
func Fen(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}
