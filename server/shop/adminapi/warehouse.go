package adminapi

import (
	"github.com/gin-gonic/gin"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model/request"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

// ---------- 仓库（GVA 管理端） ----------

// Shipable 待发货列表
func (a *Api) Shipable(c *gin.Context) {
	list, err := a.s.ShipmentService.Shipable()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// Ship 发货登记（一单只能登记一次）
func (a *Api) Ship(c *gin.Context) {
	var req request.Ship
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	actorID, _ := actor(c, "warehouse")
	order, err := a.s.ShipmentService.Ship(actorID, req.OrderNo, req.Carrier, req.TrackingNo)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, order)
}

// Shipments 发货记录
func (a *Api) Shipments(c *gin.Context) {
	list, err := a.s.ShipmentService.List()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// WarehouseOrderTrace 订单全链路视图（仓库视角）
func (a *Api) WarehouseOrderTrace(c *gin.Context) {
	actorID, role := actor(c, "warehouse")
	detail, err := a.s.ReportService.OrderDetail(c.Param("orderNo"), actorID, role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, detail)
}
