// Package adminapi 提供 GVA 管理端（后台）使用的商城运营/仓库/客服接口。
// 与买家端独立 Gin 服务（cmd/shop）共用同一套 service 层与数据库（gva 库）；
// 鉴权走 GVA JWT + Casbin（挂在 PrivateGroup），操作人以当前登录的 GVA 用户落 operation_logs。
package adminapi

import (
	"github.com/gin-gonic/gin"
	"shop/model/common/response"
	"shop/utils"

	"shop/shop/service"
)

// Api 商城后台管理接口（运营/仓库/客服）

type Api struct {
	s *service.ServiceGroup
}

// NewApi 组装接口层

func NewApi(g *service.ServiceGroup) *Api {
	return &Api{s: g}
}

// actor 返回操作人：GVA 登录用户 ID + 该接口对应的功能角色（ops/warehouse/cs）。
// service 层按角色做权限收敛，后台接口已在路由层经 GVA JWT + Casbin 鉴权。

func actor(c *gin.Context, role string) (uint64, string) {
	return uint64(utils.GetUserID(c)), role
}

// ok 统一成功响应（GVA 结构 {code:0,data,msg}）

func ok(c *gin.Context, data interface{}) {
	response.OkWithData(data, c)
}

// fail 业务错误统一转 GVA 失败响应（code:7），提示文案取业务错误信息

func fail(c *gin.Context, err error) {
	_, msg := service.BizCode(err)
	response.FailWithMessage(msg, c)
}
