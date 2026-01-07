package generator

import (
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/internal/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
)

type PluginSettings struct {
}

func mapGetOrDefault(paramsMap map[string]string, key string, defaultValue string) string {
	if val, ok := paramsMap[key]; ok {
		return val
	}
	return defaultValue
}

func NewPluginSettingsFromPlugin(p *protogen.Plugin) (*PluginSettings, error) {
	paramsMap := make(map[string]string)
	logger.Debug(p.Request.GetParameter())
	params := strings.Split(p.Request.GetParameter(), ",")
	logger.Debug("len(params)", zap.Int("len", len(params)))
	for _, param := range params {
		paramSplit := strings.Split(param, "=")
		if len(paramSplit) != 2 {
			continue
		}
		paramsMap[paramSplit[0]] = paramSplit[1]
	}

	settings := &PluginSettings{}

	return settings, nil
}
