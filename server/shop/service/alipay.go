package service

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/smartwalle/alipay/v3"

	"shop/shop/config"
)

// 支付宝真实渠道（沙箱演示扩展）：电脑网站支付（alipay.trade.page.pay）+ 退款（alipay.trade.refund）。
// 启用条件：shop.pay-channel=alipay 且配置完整；模拟渠道仍是默认，保证验收口径不变。
// 通知推进与模拟渠道共用同一套状态机/幂等/异常路径（payment.go applyPayment）。

// NotifyVerifier 支付宝渠道的附加能力：校验异步通知签名并提取业务字段。

type NotifyVerifier interface {
	VerifyNotify(ctx context.Context, form url.Values) (NotifyPayload, error)
}

// NotifyPayload 异步通知验签后提取的字段（已把元金额换算为分）

type NotifyPayload struct {
	OrderNo     string // out_trade_no
	Txn         string // trade_no
	AmountCents int64  // total_amount 换算为分
	TradeStatus string // TRADE_SUCCESS | TRADE_FINISHED | ...
	AppID       string // 通知中的 app_id，须与本渠道一致
}

// AlipayChannel 支付宝渠道实现（实现 ChannelClient + NotifyVerifier）。

type AlipayChannel struct {
	client *alipay.Client
	cfg    config.Alipay
	mu     sync.RWMutex
	calls  int
}

// NewAlipayChannel 构建支付宝渠道；沙箱网关由 SDK 按 production=false 自动选择。

func NewAlipayChannel(cfg config.Alipay) (*AlipayChannel, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.PrivateKey) == "" || strings.TrimSpace(cfg.AlipayPublicKey) == "" {
		return nil, fmt.Errorf("支付宝渠道配置不完整（app-id / private-key / alipay-public-key）")
	}
	client, err := alipay.New(cfg.AppID, cfg.PrivateKey, false)
	if err != nil {
		return nil, fmt.Errorf("初始化支付宝客户端失败: %w", err)
	}
	if err := client.LoadAliPayPublicKey(cfg.AlipayPublicKey); err != nil {
		return nil, fmt.Errorf("加载支付宝公钥失败: %w", err)
	}
	return &AlipayChannel{client: client, cfg: cfg}, nil
}

// PagePayURL 生成电脑网站支付的收银台跳转地址（GET，直接 302/window.open 即可）。

func (c *AlipayChannel) PagePayURL(orderNo string, amountCents int64, subject string) (string, error) {
	var p alipay.TradePagePay
	p.OutTradeNo = orderNo
	p.TotalAmount = centsToYuan(amountCents)
	p.Subject = subject
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"
	p.NotifyURL = c.cfg.NotifyURL
	p.ReturnURL = c.cfg.ReturnURL
	u, err := c.client.TradePagePay(p)
	if err != nil {
		return "", fmt.Errorf("构造支付宝收银台地址失败: %w", err)
	}
	return u.String(), nil
}

// VerifyNotify 校验异步通知签名（RSA2，支付宝公钥）并提取业务字段。

func (c *AlipayChannel) VerifyNotify(ctx context.Context, form url.Values) (NotifyPayload, error) {
	n, err := c.client.DecodeNotification(ctx, form)
	if err != nil {
		return NotifyPayload{}, err
	}
	if n.AppId != c.cfg.AppID {
		return NotifyPayload{}, fmt.Errorf("通知 app_id 不匹配：期望 %s 实际 %s", c.cfg.AppID, n.AppId)
	}
	cents, err := yuanToCents(n.TotalAmount)
	if err != nil {
		return NotifyPayload{}, fmt.Errorf("通知金额非法 %q: %w", n.TotalAmount, err)
	}
	return NotifyPayload{
		OrderNo:     n.OutTradeNo,
		Txn:         n.TradeNo,
		AmountCents: cents,
		TradeStatus: string(n.TradeStatus),
		AppID:       n.AppId,
	}, nil
}

// Refund 调用 alipay.trade.refund（同步接口）：code=10000 视为受理成功（含重复请求的幂等返回）。

func (c *AlipayChannel) Refund(orderNo, refundNo string, amountCents int64) (bool, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	var p alipay.TradeRefund
	p.OutTradeNo = orderNo
	p.OutRequestNo = refundNo
	p.RefundAmount = centsToYuan(amountCents)
	p.RefundReason = "商城整单退款"
	rsp, err := c.client.TradeRefund(context.Background(), p)
	if err != nil {
		return false, err
	}
	if rsp.Code != alipay.CodeSuccess {
		return false, fmt.Errorf("支付宝退款受理失败: code=%s msg=%s sub=%s(%s)", rsp.Code, rsp.Msg, rsp.SubCode, rsp.SubMsg)
	}
	return true, nil
}

// SetRefundMode 真实渠道不支持切换退款结果，忽略（保持 ChannelClient 兼容）。

func (c *AlipayChannel) SetRefundMode(mode string) {}

// RefundMode 渠道标识（真实渠道）

func (c *AlipayChannel) RefundMode() string { return "real-alipay" }

// RefundCalls 累计退款调用次数（并发闸门测试断言仍可用）

func (c *AlipayChannel) RefundCalls() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calls
}

// centsToYuan 分转元字符串：纯整数运算，避免浮点误差（两位小数）。

func centsToYuan(cents int64) string {
	if cents < 0 {
		cents = 0
	}
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// yuanToCents 元字符串转分：要求至多两位小数（支付宝 total_amount 格式）。

func yuanToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, fmt.Errorf("金额整数部分非法")
	}
	var frac int64
	if len(parts) == 2 {
		f := parts[1]
		if len(f) > 2 {
			return 0, fmt.Errorf("金额小数位超过两位")
		}
		for len(f) < 2 {
			f += "0"
		}
		frac, err = strconv.ParseInt(f, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("金额小数部分非法")
		}
	}
	return whole*100 + frac, nil
}
