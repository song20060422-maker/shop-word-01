package request

import (
	"shop/model/common/request"
	"shop/model/system"
)

type SysLoginLogSearch struct {
	system.SysLoginLog
	request.PageInfo
}
