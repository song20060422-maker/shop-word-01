package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/utils"
)

// I-18 并发防御回归：渠道退款调用必须受闸门保护，绝不重复发起。

// TestConcurrentRetrySingleChannelCall 渠道失败的退款单并发重试：
// 无论多少协程同时 Retry，渠道受理恰好一次，其余请求被闸门拒绝。
func TestConcurrentRetrySingleChannelCall(t *testing.T) {
	g, ch, _ := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")
	cs := buyerID(t, "cs1")

	o, err := g.OrderService.Create(b1, pid, 1)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o.OrderNo, "TXN-G1", o.AmountCents)
	if _, err := g.PaymentService.Notify(o.OrderNo, "TXN-G1", o.AmountCents, sign, "{}"); err != nil {
		t.Fatalf("支付通知失败: %v", err)
	}
	if _, err := g.RefundService.Apply(b1, o.OrderNo, "不想要了"); err != nil {
		t.Fatalf("申请退款失败: %v", err)
	}

	ch.SetRefundMode("fail")
	if _, err := g.RefundService.Audit(cs, o.OrderNo, true, "同意退款"); !errors.Is(err, ErrChannelRefund) {
		t.Fatalf("首次退款应渠道失败: %v", err)
	}
	firstCalls := ch.RefundCalls()
	if firstCalls != 1 {
		t.Fatalf("首次退款应只调渠道 1 次: %d", firstCalls)
	}
	rfNo := mustRefundNo(t, g, o.OrderNo)

	// 切换渠道成功后 8 协程并发重试
	ch.SetRefundMode("success")
	const workers = 8
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = g.RefundService.Retry(cs, rfNo)
		}(i)
	}
	close(start)
	wg.Wait()

	okCount, gateCount := 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			okCount++
		case errors.Is(e, ErrRefundNotRetryable):
			gateCount++
		default:
			t.Fatalf("意外错误: %v", e)
		}
	}
	if okCount != 1 || gateCount != workers-1 {
		t.Fatalf("应恰好 1 个重试成功、%d 个被闸门拒绝，实际 ok=%d gate=%d", workers-1, okCount, gateCount)
	}
	if ch.RefundCalls() != firstCalls+1 {
		t.Fatalf("渠道受理应只增加 1 次（%d -> 应为 %d，实际 %d）", firstCalls, firstCalls+1, ch.RefundCalls())
	}

	// 订单已退款、售前退款恢复库存且只恢复一次
	o2, _ := g.OrderService.Get(o.OrderNo)
	if o2.Status != model.OrderStatusRefunded {
		t.Fatalf("订单应为已退款: %s", o2.Status)
	}
	var restores int64
	testDB.Table("inventory_ledger").Where("order_no = ? AND change_type = ?", o.OrderNo, model.LedgerRestore).Count(&restores)
	if restores != 1 {
		t.Fatalf("库存恢复流水应恰好 1 条: %d", restores)
	}
}

// TestConcurrentAbnormalRefundSingleChannelCall 迟到支付异常退款并发处理：
// 同一交易号两个协程并发 HandleAbnormal，渠道受理恰好一次，后到者报"已处理"。
func TestConcurrentAbnormalRefundSingleChannelCall(t *testing.T) {
	g, ch, clock := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")
	cs := buyerID(t, "cs1")

	o, _ := g.OrderService.Create(b1, pid, 1)
	clock.Advance(11 * time.Minute)
	if _, err := g.OrderService.CloseExpired(); err != nil {
		t.Fatalf("超时关单失败: %v", err)
	}
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o.OrderNo, "TXN-G2", o.AmountCents)
	if _, err := g.PaymentService.Notify(o.OrderNo, "TXN-G2", o.AmountCents, sign, "{}"); err != nil {
		t.Fatalf("迟到通知应受理为异常: %v", err)
	}

	ch.SetRefundMode("success")
	before := ch.RefundCalls()
	errs := make([]error, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = g.RefundService.HandleAbnormal(cs, "TXN-G2", "核实后退款")
		}(i)
	}
	close(start)
	wg.Wait()

	if ch.RefundCalls() != before+1 {
		t.Fatalf("渠道受理应只增加 1 次（%d -> 应为 %d，实际 %d）", before, before+1, ch.RefundCalls())
	}
	okCount := 0
	for _, e := range errs {
		if e == nil {
			okCount++
		}
	}
	if okCount != 1 {
		t.Fatalf("应恰好一个处理成功: %d (%v / %v)", okCount, errs[0], errs[1])
	}
	var handled int64
	testDB.Model(&model.AbnormalPayment{}).Where("channel_txn_id = ? AND status = ?", "TXN-G2", model.AbnormalHandled).Count(&handled)
	if handled != 1 {
		t.Fatalf("异常记录应恰好标记已处理 1 条: %d", handled)
	}
}

// mustRefundNo 按订单号查询退款单号
func mustRefundNo(t *testing.T, g *ServiceGroup, orderNo string) string {
	t.Helper()
	rf, err := g.RefundService.Get(orderNo)
	if err != nil {
		t.Fatalf("查询退款单失败: %v", err)
	}
	return rf.RefundNo
}
