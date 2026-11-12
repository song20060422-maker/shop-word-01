package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/utils"
)

func TestCreateOrderSnapshotAndOccupy(t *testing.T) {
	g, _, _ := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")

	o, err := g.OrderService.Create(b1, pid, 2)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if o.PriceCents != 10000 || o.AmountCents != 20000 || o.Qty != 2 {
		t.Fatalf("价格快照错误: %+v", o)
	}
	if got := available(t, "SKU-A"); got != 3 {
		t.Fatalf("可售数量应反映占用: %d", got)
	}

	// 调价不影响已有订单金额
	newPrice := int64(12000)
	if _, err := g.CatalogService.Update(buyerID(t, "ops1"), "ops", pid, nil, &newPrice); err != nil {
		t.Fatalf("调价失败: %v", err)
	}
	old, _ := g.OrderService.Get(o.OrderNo)
	if old.AmountCents != 20000 || old.PriceCents != 10000 {
		t.Fatalf("已有订单金额被调价改变: %+v", old)
	}
	o2, _ := g.OrderService.Create(b1, pid, 1)
	if o2.PriceCents != 12000 {
		t.Fatalf("新订单应使用新价格: %d", o2.PriceCents)
	}

	var ledgers int64
	testDB.Model(&model.InventoryLedger{}).Where("order_no IN ?", []string{o.OrderNo, o2.OrderNo}).Count(&ledgers)
	if ledgers != 2 {
		t.Fatalf("应记录 2 条占用流水: %d", ledgers)
	}
}

func TestCreateOrderValidation(t *testing.T) {
	g, _, _ := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")

	if _, err := g.OrderService.Create(b1, pid, 0); !errors.Is(err, ErrQtyInvalid) {
		t.Fatalf("数量 0 应拒绝: %v", err)
	}
	if _, err := g.OrderService.Create(b1, pid, 3); !errors.Is(err, ErrQtyInvalid) {
		t.Fatalf("数量 3 应拒绝: %v", err)
	}
	if _, err := g.OrderService.Create(b1, 99999, 1); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("未知商品应拒绝: %v", err)
	}
	off := false
	if _, err := g.CatalogService.Update(buyerID(t, "ops1"), "ops", pid, &off, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := g.OrderService.Create(b1, pid, 1); !errors.Is(err, ErrProductOffSale) {
		t.Fatalf("下架商品应拒绝: %v", err)
	}
	on := true
	if _, err := g.CatalogService.Update(buyerID(t, "ops1"), "ops", pid, &on, nil); err != nil {
		t.Fatal(err)
	}
	// 库存 5 件：2+2+2 的第三单必须失败
	if _, err := g.OrderService.Create(b1, pid, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := g.OrderService.Create(b1, pid, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := g.OrderService.Create(b1, pid, 2); !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("库存不足应拒绝: %v", err)
	}
}

func TestConcurrentCreateNoOversell(t *testing.T) {
	g, _, _ := newEnv(t)
	pid := productID(t, "SKU-A")
	buyers := []uint64{buyerID(t, "buyer1"), buyerID(t, "buyer2"), buyerID(t, "buyer3")}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok, insufficient, other int
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := g.OrderService.Create(buyers[i%3], pid, 1)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrInsufficientStock):
				insufficient++
			default:
				other++
				t.Logf("意外错误: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if ok != 5 || insufficient != 15 || other != 0 {
		t.Fatalf("并发争抢结果错误: 成功=%d 不足=%d 其他=%d", ok, insufficient, other)
	}
	if got := available(t, "SKU-A"); got != 0 {
		t.Fatalf("售罄后应为 0: %d", got)
	}
}

func TestCancelAndTimeoutIdempotent(t *testing.T) {
	g, _, clock := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")

	o, _ := g.OrderService.Create(b1, pid, 2)
	cancelled, err := g.OrderService.Cancel(b1, o.OrderNo)
	if err != nil || cancelled.Status != model.OrderStatusCancelled {
		t.Fatalf("取消失败: %v %+v", err, cancelled)
	}
	if got := available(t, "SKU-A"); got != 5 {
		t.Fatalf("取消后应释放为 5: %d", got)
	}
	if _, err := g.OrderService.Cancel(b1, o.OrderNo); err == nil {
		t.Fatal("重复取消应报错")
	}
	if got := available(t, "SKU-A"); got != 5 {
		t.Fatalf("重复取消不得重复释放: %d", got)
	}

	// 超时关闭使用服务端时间
	o2, _ := g.OrderService.Create(b1, pid, 2)
	clock.Advance(11 * time.Minute)
	n, err := g.OrderService.CloseExpired()
	if err != nil || n != 1 {
		t.Fatalf("超时关闭失败: n=%d err=%v", n, err)
	}
	got, _ := g.OrderService.Get(o2.OrderNo)
	if got.Status != model.OrderStatusClosed || got.ClosedAt == nil {
		t.Fatalf("超时订单状态错误: %+v", got)
	}
	if available(t, "SKU-A") != 5 {
		t.Fatal("超时关闭应释放占用")
	}
	if n, _ := g.OrderService.CloseExpired(); n != 0 {
		t.Fatalf("重复清扫不得重复关闭: %d", n)
	}
}

func TestNotifyRules(t *testing.T) {
	g, _, clock := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")
	o, _ := g.OrderService.Create(b1, pid, 1)

	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o.OrderNo, "TXN-1", o.AmountCents)
	res, err := g.PaymentService.Notify(o.OrderNo, "TXN-1", o.AmountCents, sign, "{}")
	if err != nil || res.Result != model.NoticeAccepted {
		t.Fatalf("有效通知应受理: %v %+v", err, res)
	}
	paid, _ := g.OrderService.Get(o.OrderNo)
	if paid.Status != model.OrderStatusPaid || paid.PaidAt == nil {
		t.Fatalf("订单未进入待发货: %+v", paid)
	}

	// 重复通知：同一交易号，不重复记款
	res, err = g.PaymentService.Notify(o.OrderNo, "TXN-1", o.AmountCents, sign, "{}")
	if !errors.Is(err, ErrDuplicatedPay) || res.Result != model.NoticeDuplicated {
		t.Fatalf("重复通知应拒绝记款: %v %+v", err, res)
	}
	var payCount int64
	testDB.Model(&model.Payment{}).Where("order_no = ?", o.OrderNo).Count(&payCount)
	if payCount != 1 {
		t.Fatalf("重复通知不得重复记款: %d", payCount)
	}

	// 金额不符
	o2, _ := g.OrderService.Create(b1, pid, 1)
	badAmount := utils.PaySign(testCfg.Shop.PayNotifySecret, o2.OrderNo, "TXN-2", o2.AmountCents+1)
	res, _ = g.PaymentService.Notify(o2.OrderNo, "TXN-2", o2.AmountCents+1, badAmount, "{}")
	if res.Result != model.NoticeRejected {
		t.Fatalf("金额不符应拒绝: %+v", res)
	}

	// 验签失败（且必须留痕可查）
	res, err = g.PaymentService.Notify(o2.OrderNo, "TXN-3", o2.AmountCents, "deadbeef", "{}")
	if !errors.Is(err, ErrBadSign) || res.Result != model.NoticeRejected {
		t.Fatalf("验签失败应拒绝: %v %+v", err, res)
	}
	notices, _ := g.PaymentService.Notices(o2.OrderNo)
	foundBadSign := false
	for _, n := range notices {
		if !n.SignOK && n.Result == model.NoticeRejected {
			foundBadSign = true
		}
	}
	if !foundBadSign {
		t.Fatal("验签失败应可查询处理结果")
	}

	// 未知订单
	unknownSign := utils.PaySign(testCfg.Shop.PayNotifySecret, "SO-NOPE", "TXN-4", 10000)
	res, _ = g.PaymentService.Notify("SO-NOPE", "TXN-4", 10000, unknownSign, "{}")
	if res.Result != model.NoticeRejected {
		t.Fatalf("未知订单应拒绝: %+v", res)
	}

	// 同一渠道交易号不得归属两个订单
	o3, _ := g.OrderService.Create(b1, pid, 1)
	signDup := utils.PaySign(testCfg.Shop.PayNotifySecret, o3.OrderNo, "TXN-1", o3.AmountCents)
	res, err = g.PaymentService.Notify(o3.OrderNo, "TXN-1", o3.AmountCents, signDup, "{}")
	if !errors.Is(err, ErrDuplicatedPay) || res.Result != model.NoticeDuplicated {
		t.Fatalf("交易号跨订单应拒绝: %v %+v", err, res)
	}
	o3db, _ := g.OrderService.Get(o3.OrderNo)
	if o3db.Status != model.OrderStatusPendingPayment {
		t.Fatalf("冲突通知不得改变另一订单状态: %s", o3db.Status)
	}

	// 有效期外到达 → 异常收款，不恢复订单/库存
	o4, _ := g.OrderService.Create(b1, pid, 1)
	clock.Advance(11 * time.Minute)
	if n, err := g.OrderService.CloseExpired(); err != nil || n != 1 {
		t.Fatalf("清扫器应关闭超时订单: %d %v", n, err)
	}
	sign4 := utils.PaySign(testCfg.Shop.PayNotifySecret, o4.OrderNo, "TXN-5", o4.AmountCents)
	res, err = g.PaymentService.Notify(o4.OrderNo, "TXN-5", o4.AmountCents, sign4, "{}")
	if err != nil || res.Result != model.NoticeAbnormal {
		t.Fatalf("迟到支付应转异常: %v %+v", err, res)
	}
	late, _ := g.OrderService.Get(o4.OrderNo)
	if late.Status != model.OrderStatusClosed {
		t.Fatalf("迟到通知不得恢复已关闭订单: %s", late.Status)
	}
	abns, _ := g.PaymentService.AbnormalList(model.AbnormalPending)
	if len(abns) == 0 || abns[0].ChannelTxnID != "TXN-5" {
		t.Fatalf("异常记录未落库: %+v", abns)
	}
}

func TestRefundApplyPauseShipAndReject(t *testing.T) {
	g, _, _ := newEnv(t)
	b1 := buyerID(t, "buyer1")
	orderNo, _, _ := payOrder(t, g, b1, 2)

	if _, err := g.RefundService.Apply(buyerID(t, "buyer2"), orderNo, "x"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("他人订单应拒绝: %v", err)
	}
	refund, err := g.RefundService.Apply(b1, orderNo, "不想要了")
	if err != nil || refund.AuditStatus != model.AuditPending {
		t.Fatalf("申请退款失败: %v %+v", err, refund)
	}
	if _, err := g.ShipmentService.Ship(buyerID(t, "wh1"), orderNo, "SF", "SF1"); !errors.Is(err, ErrRefundInProgress) {
		t.Fatalf("退款申请期间发货应拒绝: %v", err)
	}
	if _, err := g.RefundService.Apply(b1, orderNo, "again"); !errors.Is(err, ErrRefundExists) {
		t.Fatalf("重复申请应拒绝: %v", err)
	}

	rejected, err := g.RefundService.Audit(buyerID(t, "cs1"), orderNo, false, "发货在即")
	if err != nil || rejected.AuditStatus != model.AuditRejected {
		t.Fatalf("拒绝退款失败: %v %+v", err, rejected)
	}
	paid, _ := g.OrderService.Get(orderNo)
	if paid.Status != model.OrderStatusPaid {
		t.Fatalf("拒绝后订单应回到待发货: %s", paid.Status)
	}
	if _, err := g.ShipmentService.Ship(buyerID(t, "wh1"), orderNo, "SF", "SF2"); err != nil {
		t.Fatalf("拒绝后应允许发货: %v", err)
	}
	if _, err := g.RefundService.Apply(b1, orderNo, "售后"); err == nil {
		t.Fatal("已发货订单不应受理售前退款")
	}
}

func TestRefundChannelFailThenRetrySuccess(t *testing.T) {
	g, ch, _ := newEnv(t)
	b1 := buyerID(t, "buyer1")
	cs := buyerID(t, "cs1")
	orderNo, _, amount := payOrder(t, g, b1, 2)
	before := available(t, "SKU-A")

	refund, err := g.RefundService.Apply(b1, orderNo, "整单退款")
	if err != nil {
		t.Fatal(err)
	}
	ch.SetRefundMode("fail")
	if _, err := g.RefundService.Audit(cs, orderNo, true, "同意"); !errors.Is(err, ErrChannelRefund) {
		t.Fatalf("首次退款应失败: %v", err)
	}
	r, _ := g.RefundService.Get(orderNo)
	if r.ChannelStatus != model.ChannelFailed || r.RetryCount != 1 {
		t.Fatalf("失败应保留待处理记录: %+v", r)
	}
	if available(t, "SKU-A") != before {
		t.Fatal("退款失败不得恢复库存")
	}
	order, _ := g.OrderService.Get(orderNo)
	if order.Status != model.OrderStatusRefundFailed {
		t.Fatalf("退款失败订单状态错误: %s", order.Status)
	}

	ch.SetRefundMode("success")
	retried, err := g.RefundService.Retry(cs, refund.RefundNo)
	if err != nil || retried.ChannelStatus != model.ChannelSuccess {
		t.Fatalf("重试退款应成功: %v %+v", err, retried)
	}
	order, _ = g.OrderService.Get(orderNo)
	if order.Status != model.OrderStatusRefunded {
		t.Fatalf("退款成功后订单状态错误: %s", order.Status)
	}
	if available(t, "SKU-A") != before+2 {
		t.Fatalf("售前退款应恢复 2 件: %d", available(t, "SKU-A"))
	}
	if _, err := g.RefundService.Retry(cs, refund.RefundNo); !errors.Is(err, ErrRefundNotRetryable) {
		t.Fatalf("成功后退款不可重试: %v", err)
	}
	// 重复退款不会重复恢复：直接再次调用 callChannel 等价的 Audit/Retry 已被状态拦住
	var restored int64
	testDB.Model(&model.Refund{}).Where("order_no = ? AND restored = ?", orderNo, true).Count(&restored)
	if restored != 1 {
		t.Fatalf("restored 闸门应只生效一次: %d", restored)
	}
	if amount != 20000 {
		t.Fatalf("退款金额应等于订单金额: %d", amount)
	}
}

func TestShipOnlyOnceAndStateErrors(t *testing.T) {
	g, _, _ := newEnv(t)
	b1 := buyerID(t, "buyer1")
	wh := buyerID(t, "wh1")
	orderNo, _, _ := payOrder(t, g, b1, 1)

	if _, err := g.ShipmentService.Ship(wh, orderNo, "", "SF"); !errors.Is(err, ErrInvalidParam) {
		t.Fatalf("缺少承运方应拒绝: %v", err)
	}
	if _, err := g.ShipmentService.Ship(wh, orderNo, "SF", "SF100"); err != nil {
		t.Fatalf("发货失败: %v", err)
	}
	if _, err := g.ShipmentService.Ship(wh, orderNo, "SF", "SF101"); !errors.Is(err, ErrAlreadyShipped) {
		t.Fatalf("重复发货应拒绝: %v", err)
	}
	sh, err := g.ShipmentService.Get(orderNo)
	if err != nil || sh == nil || sh.TrackingNo != "SF100" {
		t.Fatalf("运单查询失败: %v %+v", err, sh)
	}
	if list, _ := g.ShipmentService.Shipable(); len(list) != 0 {
		t.Fatalf("已发货订单不应出现在待发货列表: %d", len(list))
	}

	// 未付款订单发货
	o2, _ := g.OrderService.Create(b1, productID(t, "SKU-A"), 1)
	if _, err := g.ShipmentService.Ship(wh, o2.OrderNo, "SF", "SF200"); !errors.Is(err, ErrNotPaid) {
		t.Fatalf("未付款订单发货应拒绝: %v", err)
	}
}

func TestLatePaymentAbnormalRefund(t *testing.T) {
	g, ch, clock := newEnv(t)
	b1 := buyerID(t, "buyer1")
	cs := buyerID(t, "cs1")
	o, _ := g.OrderService.Create(b1, productID(t, "SKU-A"), 2)
	clock.Advance(11 * time.Minute)
	if n, err := g.OrderService.CloseExpired(); err != nil || n != 1 {
		t.Fatalf("超时关闭失败: %d %v", n, err)
	}
	stockAfterClose := available(t, "SKU-A")

	txn := "TXN-LATE-" + o.OrderNo
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o.OrderNo, txn, o.AmountCents)
	res, err := g.PaymentService.Notify(o.OrderNo, txn, o.AmountCents, sign, "{}")
	if err != nil || res.Result != model.NoticeAbnormal {
		t.Fatalf("迟到支付应转异常: %v %+v", err, res)
	}
	abns, _ := g.PaymentService.AbnormalList("")
	if len(abns) != 1 {
		t.Fatalf("异常记录应为 1 条: %d", len(abns))
	}

	ch.SetRefundMode("fail")
	if _, err := g.RefundService.HandleAbnormal(cs, txn, "核实迟到支付"); !errors.Is(err, ErrChannelRefund) {
		t.Fatalf("渠道失败应返回待重试: %v", err)
	}
	if available(t, "SKU-A") != stockAfterClose {
		t.Fatal("异常退款失败不得改变库存")
	}

	ch.SetRefundMode("success")
	refund, err := g.RefundService.HandleAbnormal(cs, txn, "重试退款")
	if err != nil || refund.ChannelStatus != model.ChannelSuccess {
		t.Fatalf("异常退款重试失败: %v %+v", err, refund)
	}
	if available(t, "SKU-A") != stockAfterClose {
		t.Fatalf("迟到支付退款不得恢复已释放数量: %d", available(t, "SKU-A"))
	}
	abns, _ = g.PaymentService.AbnormalList("")
	if abns[0].Status != model.AbnormalHandled {
		t.Fatalf("异常记录应标记已处理: %s", abns[0].Status)
	}
	if _, err := g.RefundService.HandleAbnormal(cs, txn, "再次处理"); err == nil {
		t.Fatal("已处理异常不应重复退款")
	}
}

func TestConcurrentRefundAndShipOnlyOneResult(t *testing.T) {
	g, _, _ := newEnv(t)
	b1 := buyerID(t, "buyer1")
	wh := buyerID(t, "wh1")
	orderNo, _, _ := payOrder(t, g, b1, 2)
	before := available(t, "SKU-A")

	var wg sync.WaitGroup
	var shipErr, refundErr error
	wg.Add(2)
	go func() { defer wg.Done(); _, shipErr = g.ShipmentService.Ship(wh, orderNo, "SF", "SF-RACE") }()
	go func() { defer wg.Done(); _, refundErr = g.RefundService.Apply(b1, orderNo, "并发退款") }()
	wg.Wait()

	order, _ := g.OrderService.Get(orderNo)
	switch {
	case shipErr == nil && refundErr != nil:
		if order.Status != model.OrderStatusShipped {
			t.Fatalf("发货成功则应为已发货: %s", order.Status)
		}
		if available(t, "SKU-A") != before {
			t.Fatal("发货路径不应恢复库存")
		}
	case refundErr == nil && shipErr != nil:
		if order.Status != model.OrderStatusRefundRequested {
			t.Fatalf("退款申请成功则应为退款申请中: %s", order.Status)
		}
	default:
		t.Fatalf("并发必须恰好一种成功: ship=%v refund=%v", shipErr, refundErr)
	}
}

func TestReportDetailTrailLedgerAndSummary(t *testing.T) {
	g, _, clock := newEnv(t)
	b1, b2 := buyerID(t, "buyer1"), buyerID(t, "buyer2")
	ops := buyerID(t, "ops1")
	orderNo, _, _ := payOrder(t, g, b1, 2)

	// 越权视图
	if _, err := g.ReportService.OrderDetail(orderNo, b2, "buyer"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("他人订单详情应拒绝: %v", err)
	}
	detail, err := g.ReportService.OrderDetail(orderNo, b1, "buyer")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Payment == nil || detail.ProductInfo == nil || !detail.ShipableNow {
		t.Fatalf("订单详情聚合字段缺失: %+v", detail)
	}
	if len(detail.Timeline) == 0 {
		t.Fatal("订单轨迹应可查")
	}

	// 迟到支付收款 10000，验证运营汇总口径
	o2, _ := g.OrderService.Create(b2, productID(t, "SKU-A"), 1)
	clock.Advance(11 * time.Minute)
	g.OrderService.CloseExpired()
	txn := "TXN-LATE-" + o2.OrderNo
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o2.OrderNo, txn, o2.AmountCents)
	if _, err := g.PaymentService.Notify(o2.OrderNo, txn, o2.AmountCents, sign, "{}"); err != nil {
		t.Fatal(err)
	}

	sum, err := g.ReportService.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalPaidCents != 30000 {
		t.Fatalf("已收款应含迟到支付（30000）: %d", sum.TotalPaidCents)
	}
	if sum.TotalRefundedCents != 0 {
		t.Fatalf("未退款时应为 0: %d", sum.TotalRefundedCents)
	}
	var prodPaid int64
	for _, p := range sum.Products {
		if p.SKU == "SKU-A" {
			prodPaid = p.PaidCents
			if p.Available != 3 {
				t.Fatalf("商品可售数量错误: %d", p.Available)
			}
		}
	}
	if prodPaid != 30000 {
		t.Fatalf("商品维度已收款应含异常收款: %d", prodPaid)
	}
	if sum.OrderCount != 2 {
		t.Fatalf("订单总数错误: %d", sum.OrderCount)
	}

	trail, err := g.ReportService.Trail(orderNo)
	if err != nil || len(trail) == 0 {
		t.Fatalf("操作轨迹应可查: %v %d", err, len(trail))
	}
	ledger, err := g.ReportService.Ledger(productID(t, "SKU-A"))
	if err != nil || len(ledger) == 0 {
		t.Fatalf("库存流水应可查: %v %d", err, len(ledger))
	}
	if _, err := g.ReportService.OrderDetail("SO-NOPE", ops, "ops"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单详情应报错: %v", err)
	}
}

func TestCatalogAndAuthAndDemoReset(t *testing.T) {
	g, _, _ := newEnv(t)
	ops := buyerID(t, "ops1")
	pid := productID(t, "SKU-A")

	if _, err := g.CatalogService.Update(buyerID(t, "buyer1"), "buyer", pid, nil, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("买家不能维护商品: %v", err)
	}
	if _, err := g.CatalogService.Update(ops, "ops", 99999, nil, nil); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("未知商品应报错: %v", err)
	}
	if list, err := g.CatalogService.OnSale(); err != nil || len(list) != 2 {
		t.Fatalf("在售商品应为 2 个: %v %d", err, len(list))
	}
	if list, err := g.CatalogService.All(); err != nil || len(list) != 2 {
		t.Fatalf("全部商品应为 2 个: %v %d", err, len(list))
	}

	// 登录
	if _, err := g.AuthService.Login("buyer1", "wrong"); !errors.Is(err, ErrLoginFailed) {
		t.Fatalf("错误密码应拒绝: %v", err)
	}
	login, err := g.AuthService.Login("buyer1", "shop123")
	if err != nil || login.Token == "" || login.User.Role != "buyer" {
		t.Fatalf("登录失败: %v %+v", err, login)
	}
	if u, err := g.AuthService.User(login.User.ID); err != nil || u.Username != "buyer1" {
		t.Fatalf("用户查询失败: %v", err)
	}
	if users, err := g.AuthService.Users("buyer"); err != nil || len(users) != 3 {
		t.Fatalf("买家账号应为 3 个: %v %d", err, len(users))
	}
	if users, err := g.AuthService.Users(""); err != nil || len(users) < 6 {
		t.Fatalf("全部账号应至少 6 个: %v %d", err, len(users))
	}

	// 演示重置
	if _, err := g.OrderService.Create(buyerID(t, "buyer1"), pid, 2); err != nil {
		t.Fatal(err)
	}
	if err := g.DemoService.Reset(); err != nil {
		t.Fatalf("重置失败: %v", err)
	}
	if available(t, "SKU-A") != 5 {
		t.Fatal("重置应恢复初始库存")
	}
	if list, _ := g.OrderService.ListByStatus("", 0); len(list) != 0 {
		t.Fatalf("重置应清空订单: %d", len(list))
	}
	if users, _ := g.AuthService.Users(""); len(users) < 6 {
		t.Fatal("重置应保留账号")
	}
}

func TestSimulatedChannel(t *testing.T) {
	ch := NewSimulatedChannel(false)
	if ch.RefundMode() != "fail" {
		t.Fatalf("初始模式错误: %s", ch.RefundMode())
	}
	if ok, _ := ch.Refund("RF1", 100); ok {
		t.Fatal("fail 模式应返回失败")
	}
	ch.SetRefundMode("success")
	if ok, _ := ch.Refund("RF1", 100); !ok {
		t.Fatal("success 模式应返回成功")
	}
	ch.SetRefundMode("非法值")
	if ch.RefundMode() != "success" {
		t.Fatal("非法模式不应生效")
	}
	if ch.RefundCalls() != 2 {
		t.Fatalf("调用次数错误: %d", ch.RefundCalls())
	}
	if NewSimulatedChannel(true).RefundMode() != "success" {
		t.Fatal("允许退款时应为 success")
	}
}

func TestOrderErrorsAndHelpers(t *testing.T) {
	g, _, _ := newEnv(t)
	b1 := buyerID(t, "buyer1")
	if _, err := g.OrderService.Get("SO-NOPE"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单应报错: %v", err)
	}
	if _, err := g.OrderService.Cancel(b1, "SO-NOPE"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("取消未知订单应报错: %v", err)
	}
	if _, err := g.RefundService.Apply(b1, "SO-NOPE", "x"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单退款应报错: %v", err)
	}
	if _, err := g.RefundService.Audit(buyerID(t, "cs1"), "SO-NOPE", true, "x"); !errors.Is(err, ErrRefundNotFound) {
		t.Fatalf("未知退款审核应报错: %v", err)
	}
	if _, err := g.RefundService.Retry(buyerID(t, "cs1"), "RF-NOPE"); !errors.Is(err, ErrRefundNotFound) {
		t.Fatalf("未知退款重试应报错: %v", err)
	}
	if _, err := g.RefundService.HandleAbnormal(buyerID(t, "cs1"), "TXN-NOPE", "x"); !errors.Is(err, ErrAbnormalNotFound) {
		t.Fatalf("未知异常应报错: %v", err)
	}
	if _, err := g.ShipmentService.Ship(buyerID(t, "wh1"), "SO-NOPE", "SF", "1"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单发货应报错: %v", err)
	}
	sh, err := g.ShipmentService.Get("SO-NOPE")
	if err != nil || sh != nil {
		t.Fatalf("无发货记录应返回 nil: %v %+v", err, sh)
	}
	if list, err := g.ShipmentService.List(); err != nil || len(list) != 0 {
		t.Fatalf("发货列表应为空: %v %d", err, len(list))
	}
	if list, err := g.RefundService.List(model.AuditPending); err != nil || len(list) != 0 {
		t.Fatalf("退款列表应为空: %v %d", err, len(list))
	}
	if list, err := g.PaymentService.NoticesByResult(model.NoticeAccepted, 10); err != nil || len(list) != 0 {
		t.Fatalf("通知过滤应为空: %v %d", err, len(list))
	}
	if list, err := g.OrderService.ListByBuyer(b1, ""); err != nil || len(list) != 0 {
		t.Fatalf("买家订单应为空: %v %d", err, len(list))
	}

	// 状态错误翻译
	o, _ := g.OrderService.Create(b1, productID(t, "SKU-A"), 1)
	if _, err := g.RefundService.Apply(b1, o.OrderNo, "未付款"); !errors.Is(err, ErrNotPaid) {
		t.Fatalf("未付款申请退款应报未支付: %v", err)
	}
	if _, err := g.ShipmentService.Ship(buyerID(t, "wh1"), o.OrderNo, "SF", "1"); !errors.Is(err, ErrNotPaid) {
		t.Fatalf("未付款发货应报未支付: %v", err)
	}
	if _, err := g.OrderService.Cancel(buyerID(t, "buyer2"), o.OrderNo); !errors.Is(err, ErrForbidden) {
		t.Fatalf("他人取消应拒绝: %v", err)
	}
}
