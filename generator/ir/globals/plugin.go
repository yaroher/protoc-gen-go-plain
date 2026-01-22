package globals

import (
	"sync/atomic"

	"google.golang.org/protobuf/compiler/protogen"
)

var globalPlugin atomic.Pointer[protogen.Plugin]

func SetPlugin(plugin *protogen.Plugin) {
	globalPlugin.Store(plugin)
}

func Plugin() *protogen.Plugin {
	pl := globalPlugin.Load()
	if pl == nil {
		panic("plugin not set global")
	}
	return pl
}
