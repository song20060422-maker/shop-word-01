package service

import (
	"errors"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/utils"
)

// 契约兼容层 service 扩展方法的真实业务路径测试（替代工具函数补洞）。

// TestProductBySKU 契约以 SKU 作为商品寻址：命中/空串/未命中三分支。
func TestProductBySKU(t *testing.T) {
	g, _, _ := newEnv(t)
	p, err := g.CatalogService.ProductBySKU("SKU-A")
	if err != nil || p.SKU != "SKU-A" {
		t.Fatalf("按 SKU 查询失败: %v %+v", err, p)
	}
	if _, err := g.CatalogService.ProductBySKU(""); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("空 SKU 应返回未找到: %v", err)
	}
	if _, err := g.CatalogService.ProductBySKU("NOPE"); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("未知 SKU 应返回未找到: %v", err)
	}
}

// TestOverwriteAvailable 运营覆盖可售数量：生效、追加流水、拒绝负数、无变化只记日志。
func TestOverwriteAvailable(t *testing.T) {
	g, _, _ := newEnv(t)
	pid := productID(t, "SKU-A")
	ops := buyerID(t, "ops1")

	p, err := g.CatalogService.OverwriteAvailable(ops, "ops", pid, 10)
	if err != nil || p.Available != 10 {
		t.Fatalf("覆盖可售数量失败: %v %d", err, p.Available)
	}
	var led int64
	testDB.Table("inventory_ledger").Where("product_id = ? AND change_type = ?", pid, "restore").Count(&led)
	if led != 1 {
		t.Fatalf("覆盖应追加 1 条库存流水: %d", led)
	}

	if _, err := g.CatalogService.OverwriteAvailable(ops, "ops", pid, -1); err == nil {
		t.Fatal("负数应被拒绝")
	}
	if _, err := g.CatalogService.OverwriteAvailable(ops, "ops", pid, 10); err != nil {
		t.Fatalf("同值覆盖（无变化）不应报错: %v", err)
	}
	var led2 int64
	testDB.Table("inventory_ledger").Where("product_id = ? AND change_type = ?", pid, "restore").Count(&led2)
	if led2 != 1 {
		t.Fatalf("无变化覆盖不得追加流水: %d", led2)
	}
	if _, err := g.CatalogService.OverwriteAvailable(ops, "ops", 999999, 5); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("未知商品应返回未找到: %v", err)
	}
}

// TestFindByRefDualAddressing 订单双寻址（order_no / 数字主键）与未命中。
func TestFindByRefDualAddressing(t *testing.T) {
	g, _, _ := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")
	o, _ := g.OrderService.Create(b1, pid, 1)

	byNo, err := g.OrderService.FindByRef(o.OrderNo)
	if err != nil || byNo.OrderNo != o.OrderNo {
		t.Fatalf("按订单号寻址失败: %v", err)
	}
	byID, err := g.OrderService.FindByRef(itoa(int(o.ID)))
	if err != nil || byID.OrderNo != o.OrderNo {
		t.Fatalf("按主键寻址失败: %v", err)
	}
	if _, err := g.OrderService.FindByRef(""); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("空引用应返回未找到: %v", err)
	}
	if _, err := g.OrderService.FindByRef("NOPE"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知引用应返回未找到: %v", err)
	}
}

// TestPendingAndForceExpire 待付款清单与强制过期钩子。
func TestPendingAndForceExpire(t *testing.T) {
	g, _, clock := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")
	o, _ := g.OrderService.Create(b1, pid, 1)

	nos, err := g.OrderService.PendingOrderNos()
	if err != nil || len(nos) != 1 || nos[0] != o.OrderNo {
		t.Fatalf("待付款清单应含新订单: %v %v", nos, err)
	}

	clock.Advance(11 * time.Minute)
	if err := g.OrderService.ForceExpire(o.OrderNo, clock.Now()); err != nil {
		t.Fatalf("强制过期失败: %v", err)
	}
	if err := g.OrderService.ForceExpire("", clock.Now()); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("空单号应返回未找到: %v", err)
	}
	if n, _ := g.OrderService.CloseExpired(); n != 1 {
		t.Fatalf("强制过期后应可被清扫: %d", n)
	}
	if nos2, _ := g.OrderService.PendingOrderNos(); len(nos2) != 0 {
		t.Fatalf("清扫后待付款清单应为空: %v", nos2)
	}
}

// TestPaymentsAndAbnormalQueries 收款记录与异常记录查询路径。
func TestPaymentsAndAbnormalQueries(t *testing.T) {
	g, _, clock := newEnv(t)
	pid := productID(t, "SKU-A")
	b1 := buyerID(t, "buyer1")
	o, _ := g.OrderService.Create(b1, pid, 1)

	// 正常记款 → PaymentsByOrder 1 条
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o.OrderNo, "TXN-C1", o.AmountCents)
	if _, err := g.PaymentService.Notify(o.OrderNo, "TXN-C1", o.AmountCents, sign, "{}"); err != nil {
		t.Fatalf("记款失败: %v", err)
	}
	pays, err := g.PaymentService.PaymentsByOrder(o.OrderNo)
	if err != nil || len(pays) != 1 || pays[0].ChannelTxnID != "TXN-C1" {
		t.Fatalf("收款记录查询错误: %v %d", err, len(pays))
	}

	// 迟到支付 → 异常记录，AbnormalByID 命中/未命中
	o2, _ := g.OrderService.Create(b1, pid, 1)
	clock.Advance(11 * time.Minute)
	if _, err := g.OrderService.CloseExpired(); err != nil {
		t.Fatalf("超时关单失败: %v", err)
	}
	sign2 := utils.PaySign(testCfg.Shop.PayNotifySecret, o2.OrderNo, "TXN-C2", o2.AmountCents)
	if _, err := g.PaymentService.Notify(o2.OrderNo, "TXN-C2", o2.AmountCents, sign2, "{}"); err != nil {
		t.Fatalf("迟到通知失败: %v", err)
	}
	ab, err := g.PaymentService.AbnormalList("")
	if err != nil || len(ab) != 1 {
		t.Fatalf("异常列表应有 1 条: %v %d", err, len(ab))
	}
	got, err := g.PaymentService.AbnormalByID(ab[0].ID)
	if err != nil || got.ChannelTxnID != "TXN-C2" {
		t.Fatalf("按主键查异常失败: %v %+v", err, got)
	}
	if _, err := g.PaymentService.AbnormalByID(999999); !errors.Is(err, ErrAbnormalNotFound) {
		t.Fatalf("未知异常主键应返回未找到: %v", err)
	}
}

// TestApplyRefundGuards 退款申请的归属与状态守卫。
func TestApplyRefundGuards(t *testing.T) {
	g, _, _ := newEnv(t)
	pid := productID(t, "SKU-A")
	b1, b2 := buyerID(t, "buyer1"), buyerID(t, "buyer2")
	cs := buyerID(t, "cs1")

	o, _ := g.OrderService.Create(b1, pid, 1)
	if _, err := g.RefundService.Apply(b1, o.OrderNo, "未付款退款"); err == nil {
		t.Fatal("待付款订单不得申请售前退款")
	}

	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o.OrderNo, "TXN-C3", o.AmountCents)
	if _, err := g.PaymentService.Notify(o.OrderNo, "TXN-C3", o.AmountCents, sign, "{}"); err != nil {
		t.Fatalf("记款失败: %v", err)
	}
	if _, err := g.RefundService.Apply(b2, o.OrderNo, "他人订单"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("非归属买家应被拒绝: %v", err)
	}

	// 拒绝路径：拒绝后订单回可发货，可正常发货
	if _, err := g.RefundService.Apply(b1, o.OrderNo, "无理由"); err != nil {
		t.Fatalf("合法退款申请失败: %v", err)
	}
	if _, err := g.RefundService.Audit(cs, o.OrderNo, false, "证据不足"); err != nil {
		t.Fatalf("审核拒绝失败: %v", err)
	}
	o3, _ := g.OrderService.Get(o.OrderNo)
	if o3.Status != model.OrderStatusPaid {
		t.Fatalf("拒绝后订单应回待发货: %s", o3.Status)
	}
	// 被拒绝的退款单允许重新申请，复用同一张退款单（整单退款一单一张）
	rf2, err := g.RefundService.Apply(b1, o.OrderNo, "补充理由后再次申请")
	if err != nil {
		t.Fatalf("重新申请应成功: %v", err)
	}
	rfOld, _ := g.RefundService.Get(o.OrderNo)
	if rf2.ID != rfOld.ID || rf2.AuditStatus != model.AuditPending {
		t.Fatalf("应复用原退款单并回到待审核: %+v %+v", rf2, rfOld)
	}
}
