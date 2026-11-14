package response

import "shop/config"

type SysConfigResponse struct {
	Config config.Server `json:"config"`
}
