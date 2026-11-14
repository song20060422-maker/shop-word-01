package request

import (
	"shop/model/common/request"
	"shop/model/system"
)

type SysOperationRecordSearch struct {
	system.SysOperationRecord
	request.PageInfo
}
