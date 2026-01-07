package generator

import (
	"os"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/internal/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protojson"
)

type PluginSettings struct {
	TypeOverrides []*goplain.OverwriteType
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

	if overridesPath, ok := paramsMap["overrides_file"]; ok && overridesPath != "" {
		raw, err := os.ReadFile(overridesPath)
		if err != nil {
			return nil, err
		}
		var params goplain.PlainFileParams
		if err := protojson.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		settings.TypeOverrides = params.GetOverwrite()
	}

	return settings, nil
}
