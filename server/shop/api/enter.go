// Package api 承载 HTTP 接口层（类 GVA api 分层），只做参数绑定、权限与响应封装，
// 业务规则全部收敛在 service 层。
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model/response"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

// ApiGroup 全部接口处理器
type ApiGroup struct {
	AuthApi
	BuyerApi
	StaffApi
	PayApi
	DemoApi
}

// NewApiGroup 组装接口层
func NewApiGroup(g *service.ServiceGroup) *ApiGroup {
	return &ApiGroup{
		AuthApi:  AuthApi{s: g},
		BuyerApi: BuyerApi{s: g},
		StaffApi: StaffApi{s: g},
		PayApi:   PayApi{s: g},
		DemoApi:  DemoApi{s: g},
	}
}

// fail 把业务错误翻译为 HTTP 状态 + 稳定错误码，便于三类使用者看到明确原因。
func fail(c *gin.Context, err error) {
	code, msg := service.BizCode(err)
	c.JSON(statusOf(code), response.Fail(code, msg))
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, response.OK(data))
}

func statusOf(code int) int {
	switch code {
	case 1000, 3001, 3002, 4010:
		return http.StatusBadRequest
	case 1001, 2001, 3005, 4001:
		return http.StatusNotFound
	case 2002:
		return http.StatusForbidden
	case 9000:
		return http.StatusInternalServerError
	default:
		// 其余为业务状态冲突：订单已关闭、重复退款、已发货等，语义上是 409
		return http.StatusConflict
	}
}
