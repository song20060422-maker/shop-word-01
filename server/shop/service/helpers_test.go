package service

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
)

func TestBizErrorHelpers(t *testing.T) {
	e := NewBizError(1003, "可售数量不足")
	if e.Error() != "可售数量不足" {
		t.Fatalf("错误文案错误: %s", e.Error())
	}
	if !IsBiz(e) {
		t.Fatal("BizError 应识别为业务错误")
	}
	if IsBiz(errors.New("plain")) {
		t.Fatal("普通错误不应识别为业务错误")
	}
	code, msg := BizCode(e)
	if code != 1003 || msg != "可售数量不足" {
		t.Fatalf("错误码解析错误: %d %s", code, msg)
	}
	if BizCodeMsg(e) != "可售数量不足" {
		t.Fatal("BizCodeMsg 错误")
	}
	// 未知错误统一 9000，保留底层原因
	code, msg = BizCode(errors.New("db down"))
	if code != 9000 || msg == "" {
		t.Fatalf("未知错误应映射 9000: %d %s", code, msg)
	}
	// nil 错误按未知错误处理（9000），提示非空但不泄露内部细节
	if code, _ := BizCode(nil); code != 9000 {
		t.Fatalf("nil 错误应映射 9000: %d", code)
	}
	if BizCodeMsg(errors.New("")) == "" {
		t.Fatal("未知错误应给出兜底提示")
	}
}

func TestStatusLabelsAndStateErrors(t *testing.T) {
	cases := map[string]string{
		model.OrderStatusPendingPayment:  "待付款",
		model.OrderStatusCancelled:       "取消",
		model.OrderStatusClosed:          "关闭",
		model.OrderStatusPaid:            "付款",
		model.OrderStatusShipped:         "发货",
		model.OrderStatusRefundRequested: "退款申请",
		model.OrderStatusRefundFailed:    "退款失败",
		model.OrderStatusRefunded:        "退款",
		"unknown":                        "unknown",
	}
	for status, want := range cases {
		if got := statusLabel(status); got != want {
			t.Fatalf("状态文案错误 %s: %s", status, got)
		}
	}

	if err := shipStateError(model.OrderStatusRefundRequested); !errors.Is(err, ErrRefundInProgress) {
		t.Fatalf("退款申请中应禁止发货: %v", err)
	}
	if err := shipStateError(model.OrderStatusRefunded); !errors.Is(err, ErrRefunded) {
		t.Fatalf("已退款应禁止发货: %v", err)
	}
	if err := shipStateError(model.OrderStatusRefundFailed); err == nil {
		t.Fatal("退款失败应禁止发货")
	}
	if err := shipStateError(model.OrderStatusPendingPayment); !errors.Is(err, ErrNotPaid) {
		t.Fatalf("待付款应禁止发货: %v", err)
	}
	if err := shipStateError(model.OrderStatusClosed); !errors.Is(err, ErrOrderClosed) {
		t.Fatalf("已关闭应禁止发货: %v", err)
	}
	if err := shipStateError(model.OrderStatusShipped); !errors.Is(err, ErrAlreadyShipped) {
		t.Fatalf("已发货应禁止重复发货: %v", err)
	}
}

func TestStateErrorReadsCurrentStatus(t *testing.T) {
	g, _, clock := newEnv(t)
	b1 := buyerID(t, "buyer1")
	pid := productID(t, "SKU-A")

	o, err := g.OrderService.Create(b1, pid, 1)
	if err != nil {
		t.Fatal(err)
	}
	// 待付款 → 状态不允许该操作
	if err := stateError(testDB, o.OrderNo); !errors.Is(err, ErrOrderNotPayable) {
		t.Fatalf("待付款订单状态错误: %v", err)
	}
	// 未知订单
	if err := stateError(testDB, "SO-NOT-EXIST"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("未知订单应报错: %v", err)
	}
	// 已关闭
	clock.Advance(11 * time.Minute)
	if _, err := g.OrderService.CloseExpired(); err != nil {
		t.Fatal(err)
	}
	if err := stateError(testDB, o.OrderNo); !errors.Is(err, ErrOrderClosed) {
		t.Fatalf("已关闭订单状态错误: %v", err)
	}
}

func TestRefundFindByRef(t *testing.T) {
	g, _, _ := newEnv(t)
	b1 := buyerID(t, "buyer1")
	orderNo, _, _ := payOrder(t, g, b1, 1)
	rf, err := g.RefundService.Apply(b1, orderNo, "测试")
	if err != nil {
		t.Fatal(err)
	}
	// 空引用
	if _, err := g.RefundService.FindByRef("  "); !errors.Is(err, ErrRefundNotFound) {
		t.Fatalf("空引用应报错: %v", err)
	}
	// 退款单号
	if got, err := g.RefundService.FindByRef(rf.RefundNo); err != nil || got.OrderNo != orderNo {
		t.Fatalf("按退款单号查询失败: %v %+v", err, got)
	}
	// 订单号
	if got, err := g.RefundService.FindByRef(orderNo); err != nil || got.RefundNo != rf.RefundNo {
		t.Fatalf("按订单号查询失败: %v %+v", err, got)
	}
	// 数字主键
	if got, err := g.RefundService.FindByRef(strconv.FormatUint(rf.ID, 10)); err != nil || got.ID != rf.ID {
		t.Fatalf("按主键查询失败: %v %+v", err, got)
	}
	// 不存在
	if _, err := g.RefundService.FindByRef("999999"); !errors.Is(err, ErrRefundNotFound) {
		t.Fatalf("不存在的主键应报错: %v", err)
	}
}
