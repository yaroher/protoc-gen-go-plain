package globals

import (
	"fmt"
	"sync/atomic"
)

var globalSuffix atomic.Pointer[string]

func init() {
	globalSuffix = atomic.Pointer[string]{}
	defaultSuffix := "Plain"
	globalSuffix.Store(&defaultSuffix)
}

func SetSuffix(suffix string) {
	globalSuffix.Store(&suffix)
}

func Suffix() string {
	return *globalSuffix.Load()
}

func ApplySuffix(suffix string) string {
	return fmt.Sprintf("%s%s", *globalSuffix.Load(), suffix)
}
