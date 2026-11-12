package api

import (
	"github.com/gin-gonic/gin"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

// DemoApi 演示辅助接口（仅 shop.demo-mode=true 时由路由层放行）
type DemoApi struct {
	s *service.ServiceGroup
}

// Reset 重置演示业务数据：清空订单/收款/退款/发货/异常/流水，恢复商品初始可售数量。
func (a DemoApi) Reset(c *gin.Context) {
	if err := a.s.DemoService.Reset(); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"reset": true})
}
