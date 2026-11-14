package service

import "sync"

// ChannelClient 支付渠道：受理退款并报告退款结果。
// 默认实现为可控内存模拟渠道，保证「首次失败、再次成功」可复现；
// 真实渠道（支付宝沙箱）见 alipay.go，经 shop.pay-channel=alipay 启用。

type ChannelClient interface {
	// Refund 受理退款：orderNo 用于真实渠道定位原交易（模拟渠道忽略）
	Refund(orderNo, refundNo string, amountCents int64) (bool, error)
	SetRefundMode(mode string)
	RefundMode() string
	RefundCalls() int
}

// SimulatedChannel 模拟渠道实现

type SimulatedChannel struct {
	mu    sync.RWMutex
	mode  string // success | fail
	calls int
}

// NewSimulatedChannel 创建模拟渠道；allow=true 时退款直接成功。

func NewSimulatedChannel(allow bool) *SimulatedChannel {
	mode := "fail"
	if allow {
		mode = "success"
	}
	return &SimulatedChannel{mode: mode}
}

// Refund 受理退款（模拟渠道忽略订单号）

func (c *SimulatedChannel) Refund(orderNo, refundNo string, amountCents int64) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.mode == "success", nil
}

// SetRefundMode 切换退款受理结果（演示用）

func (c *SimulatedChannel) SetRefundMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if mode == "fail" || mode == "success" {
		c.mode = mode
	}
}

// RefundMode 当前退款受理结果

func (c *SimulatedChannel) RefundMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mode
}

// RefundCalls 累计受理次数（用于演示与测试断言）

func (c *SimulatedChannel) RefundCalls() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calls
}
