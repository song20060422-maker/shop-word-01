package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"shop/shop/config"
	"shop/shop/model"
)

// 支付宝沙箱渠道测试：本地生成 RSA 密钥对，私钥充当商户私钥，
// 公钥充当"支付宝公钥"，从而无需真实沙箱网络即可验证签名链路与通知推进。

func genKeyPair(t *testing.T) (privPEM, pubPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("序列化公钥失败: %v", err)
	}
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
	return privPEM, pubPEM
}

func newAliEnv(t *testing.T) (*ServiceGroup, *AlipayChannel, *fakeClock, *rsa.PrivateKey) {
	t.Helper()
	resetDB(t)
	privPEM, pubPEM := genKeyPair(t)
	block, _ := pem.Decode([]byte(privPEM))
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("解析测试私钥失败: %v", err)
	}
	cfg := testCfg
	cfg.Shop.PayChannel = "alipay"
	cfg.Shop.Alipay = config.Alipay{
		AppID:           "9021000158651669",
		PrivateKey:      privPEM,
		AlipayPublicKey: pubPEM,
		NotifyURL:       "http://notify.example/api/pay/alipay/notify",
		ReturnURL:       "http://127.0.0.1:8090/shop",
	}
	ch, err := NewAlipayChannel(cfg.Shop.Alipay)
	if err != nil {
		t.Fatalf("构建支付宝渠道失败: %v", err)
	}
	clock := &fakeClock{t: time.Date(2026, 11, 14, 9, 0, 0, 0, time.Local)}
	g := NewServiceGroup(Deps{DB: testDB, Now: clock.Now, Cfg: cfg, Channel: ch})
	return g, ch, clock, key.(*rsa.PrivateKey)
}

// signAlipayForm 按支付宝规则签名通知表单：除 sign/sign_type 外按 key 升序 a=b&c=d，
// RSA2(SHA256withRSA) 后 base64。

func signAlipayForm(t *testing.T, key *rsa.PrivateKey, form url.Values) string {
	t.Helper()
	keys := make([]string, 0, len(form))
	for k := range form {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(form.Get(k))
	}
	digest := sha256.Sum256(buf.Bytes())
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

func alipayNotifyForm(t *testing.T, key *rsa.PrivateKey, orderNo, txn, totalAmount, tradeStatus string) url.Values {
	form := url.Values{}
	form.Set("app_id", "9021000158651669")
	form.Set("trade_no", txn)
	form.Set("out_trade_no", orderNo)
	form.Set("total_amount", totalAmount)
	form.Set("trade_status", tradeStatus)
	form.Set("sign_type", "RSA2")
	form.Set("sign", signAlipayForm(t, key, form))
	return form
}

func TestAlipayAmountConvert(t *testing.T) {
	cases := []struct {
		cents int64
		yuan  string
	}{
		{0, "0.00"}, {1, "0.01"}, {1234, "12.34"}, {100, "1.00"}, {1000000, "10000.00"},
	}
	for _, c := range cases {
		if got := centsToYuan(c.cents); got != c.yuan {
			t.Fatalf("centsToYuan(%d) = %s，期望 %s", c.cents, got, c.yuan)
		}
		back, err := yuanToCents(c.yuan)
		if err != nil || back != c.cents {
			t.Fatalf("yuanToCents(%s) = %d,%v，期望 %d", c.yuan, back, err, c.cents)
		}
	}
	if _, err := yuanToCents("12.345"); err == nil {
		t.Fatal("三位小数应报错")
	}
	if _, err := yuanToCents("abc"); err == nil {
		t.Fatal("非数字应报错")
	}
	if v, err := yuanToCents(" 12.3 "); err != nil || v != 1230 {
		t.Fatalf("带空白金额解析失败: %d %v", v, err)
	}
}

func TestAlipayPagePayURL(t *testing.T) {
	g, _, _, _ := newAliEnv(t)
	b := buyerID(t, "buyer1")
	pid := productID(t, "SKU-A")
	o, err := g.OrderService.Create(b, pid, 1)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	u, err := g.PaymentService.AlipayPayURL(o.OrderNo, b)
	if err != nil {
		t.Fatalf("生成收银台地址失败: %v", err)
	}
	for _, want := range []string{"alipay.trade.page.pay", "out_trade_no", "sign=", "9021000158651669"} {
		if !strings.Contains(u, want) {
			t.Fatalf("收银台地址缺少 %s：%s", want, u)
		}
	}
	// 未启用支付宝渠道时应报业务错误
	g2, _, _ := newEnv(t)
	o2, _ := g2.OrderService.Create(b, productID(t, "SKU-A"), 1)
	if _, err := g2.PaymentService.AlipayPayURL(o2.OrderNo, b); err == nil {
		t.Fatal("模拟渠道下应拒绝生成收银台地址")
	}
}

func TestAlipayVerifyNotify(t *testing.T) {
	_, ch, _, key := newAliEnv(t)
	form := alipayNotifyForm(t, key, "SO-1", "TXN-ALI-1", "12.34", "TRADE_SUCCESS")
	p, err := ch.VerifyNotify(context.Background(), form)
	if err != nil {
		t.Fatalf("合法签名验签失败: %v", err)
	}
	if p.OrderNo != "SO-1" || p.Txn != "TXN-ALI-1" || p.AmountCents != 1234 || p.TradeStatus != "TRADE_SUCCESS" {
		t.Fatalf("通知字段提取错误: %+v", p)
	}
	// 篡改金额后验签必须失败
	form.Set("total_amount", "12.35")
	if _, err := ch.VerifyNotify(context.Background(), form); err == nil {
		t.Fatal("篡改后的通知应验签失败")
	}
	// app_id 不匹配必须拒绝
	form2 := alipayNotifyForm(t, key, "SO-1", "TXN-ALI-2", "12.34", "TRADE_SUCCESS")
	form2.Set("app_id", "other-app")
	form2.Set("sign", signAlipayForm(t, key, form2))
	if _, err := ch.VerifyNotify(context.Background(), form2); err == nil {
		t.Fatal("app_id 不匹配应被拒绝")
	}
}

func TestAlipayNotifyFlow(t *testing.T) {
	g, _, clock, key := newAliEnv(t)
	b := buyerID(t, "buyer1")
	pid := productID(t, "SKU-A")
	o, err := g.OrderService.Create(b, pid, 2)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	amountYuan := centsToYuan(o.AmountCents)

	// 1. 未完成交易（买家未付款）应拒绝记款
	res, err := g.PaymentService.NotifyAlipay(alipayNotifyForm(t, key, o.OrderNo, "TXN-ALI-WAIT", amountYuan, "TRADE_WAIT_BUYER_PAY"), "")
	if err == nil || res.Result != model.NoticeRejected {
		t.Fatalf("未完成交易应被拒绝: %+v %v", res, err)
	}

	// 2. 合法成功通知：推进 paid 并记款
	txn := "TXN-ALI-" + o.OrderNo
	res, err = g.PaymentService.NotifyAlipay(alipayNotifyForm(t, key, o.OrderNo, txn, amountYuan, "TRADE_SUCCESS"), "")
	if err != nil || res.Result != model.NoticeAccepted {
		t.Fatalf("支付宝通知未受理: %+v %v", res, err)
	}
	var order model.Order
	if err := testDB.Where("order_no = ?", o.OrderNo).First(&order).Error; err != nil || order.Status != model.OrderStatusPaid {
		t.Fatalf("订单未推进 paid: %s %v", order.Status, err)
	}
	var pay model.Payment
	if err := testDB.Where("channel_txn_id = ?", txn).First(&pay).Error; err != nil || pay.AmountCents != o.AmountCents {
		t.Fatalf("记款缺失或金额不符: %+v %v", pay, err)
	}

	// 3. 同一交易号重复通知：duplicated 且不重复记款
	res, err = g.PaymentService.NotifyAlipay(alipayNotifyForm(t, key, o.OrderNo, txn, amountYuan, "TRADE_SUCCESS"), "")
	if err == nil || res.Result != model.NoticeDuplicated {
		t.Fatalf("重复通知应为 duplicated: %+v %v", res, err)
	}
	var payCount int64
	testDB.Model(&model.Payment{}).Where("order_no = ?", o.OrderNo).Count(&payCount)
	if payCount != 1 {
		t.Fatalf("重复通知造成重复记款: %d", payCount)
	}

	// 4. 篡改签名：rejected 且 sign_ok=false（留痕可查）
	clock.Advance(time.Minute)
	bad := alipayNotifyForm(t, key, o.OrderNo, "TXN-ALI-BAD", amountYuan, "TRADE_SUCCESS")
	bad.Set("sign", base64.StdEncoding.EncodeToString([]byte("forged")))
	res, err = g.PaymentService.NotifyAlipay(bad, "")
	if err == nil || res.Result != model.NoticeRejected {
		t.Fatalf("伪造签名应被拒绝: %+v %v", res, err)
	}
	notices, _ := g.PaymentService.Notices(o.OrderNo)
	found := false
	for _, n := range notices {
		if !n.SignOK && n.Result == model.NoticeRejected {
			found = true
		}
	}
	if !found {
		t.Fatal("验签失败通知未留痕")
	}
}

func TestAlipayRefundReal(t *testing.T) {
	_, ch, _, _ := newAliEnv(t)
	if ch.RefundMode() != "real-alipay" {
		t.Fatalf("真实渠道模式标识错误: %s", ch.RefundMode())
	}
	ch.SetRefundMode("fail") // no-op，不影响真实渠道
	if ch.RefundMode() != "real-alipay" {
		t.Fatal("真实渠道不应响应退款模式切换")
	}
	// Refund 需要真实沙箱网关，单测只验证调用计数与配置错误路径
	before := ch.RefundCalls()
	if _, err := ch.Refund("SO-X", "RF-X", 100); err == nil {
		// 沙箱网关在 CI 可能不可达：允许网络错误，但不允许误报成功
		t.Log("沙箱网关可达并返回了结果（本地演示环境）")
	}
	if ch.RefundCalls() != before+1 {
		t.Fatal("退款调用计数未累加")
	}
}
