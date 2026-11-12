package service

import (
	"errors"
	"testing"
)

// TestUtilityStringBranches 覆盖 itoa 正/负/零与 boolLabel 两个分支。
func TestUtilityStringBranches(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{7, "7"},
		{-5, "-5"},
		{123456, "123456"},
		{-987654, "-987654"},
	}
	for _, c := range cases {
		if got := itoa(c.in); got != c.want {
			t.Fatalf("itoa(%d) = %s, want %s", c.in, got, c.want)
		}
	}
	if boolLabel(true) != "是" || boolLabel(false) != "否" {
		t.Fatal("boolLabel 分支错误")
	}
}

// TestCatalogGetNotFound 覆盖商品查询的未命中分支。
func TestCatalogGetNotFound(t *testing.T) {
	g, _, _ := newEnv(t)
	_, err := g.CatalogService.Get(999999)
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("应返回商品不存在: %v", err)
	}
	pid := productID(t, "SKU-A")
	p, err := g.CatalogService.Get(pid)
	if err != nil || p.SKU != "SKU-A" {
		t.Fatalf("命中查询失败: %v %+v", err, p)
	}
}

// TestListByBuyerEmpty 覆盖买家订单列表空结果分支。
func TestListByBuyerEmpty(t *testing.T) {
	g, _, _ := newEnv(t)
	list, err := g.OrderService.ListByBuyer(buyerID(t, "buyer2"), "")
	if err != nil || len(list) != 0 {
		t.Fatalf("新买家列表应为空: %v %d", err, len(list))
	}
}
