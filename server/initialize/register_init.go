package initialize

import (
	_ "shop/source/example"
	_ "shop/source/system"
)

func init() {
	// do nothing,only import source package so that inits can be registered
}
