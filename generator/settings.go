package generator

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
)

type PluginSettings struct {
	EnumAsString bool
	EnumAsInt    bool
}

func mapGetOrDefault(paramsMap map[string]string, key string, defaultValue string) string {
	if val, ok := paramsMap[key]; ok {
		return val
	}
	return defaultValue
}

func NewPluginSettingsFromPlugin(p *protogen.Plugin) (*PluginSettings, error) {
	paramsMap := make(map[string]string)
	zap.L().Debug(p.Request.GetParameter())
	params := strings.Split(p.Request.GetParameter(), ",")
	zap.L().Debug("len(params)", zap.Int("len", len(params)))
	for _, param := range params {
		paramSplit := strings.Split(param, "=")
		if len(paramSplit) != 2 {
			continue
		}
		paramsMap[paramSplit[0]] = paramSplit[1]
	}

	settings := &PluginSettings{
		EnumAsString: mapGetOrDefault(paramsMap, "enum_as_string", "false") == "true",
		EnumAsInt:    mapGetOrDefault(paramsMap, "enum_as_int", "false") == "true",
	}
	if settings.EnumAsString && settings.EnumAsInt {
		return nil, fmt.Errorf("enum_as_string and enum_as_int cannot be both true")
	}
	return settings, nil
}
