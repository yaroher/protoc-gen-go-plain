package generator

import (
	"strings"

	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
)

type PluginSettings struct {
	JSONJX bool
	// JXPB generates MarshalJX/UnmarshalJX methods for original protobuf structs.
	// This allows nested messages to use fast JX instead of protojson fallback.
	JXPB bool
	// SparseJSON enables sparse serialization mode with Src_ field checks.
	// When false: faster serialization without sparse field tracking.
	// When true (default): only fields in Src_ are serialized.
	SparseJSON bool
	// GeneratePool generates sync.Pool, Reset(), Get/Put methods for Plain structs.
	// Enables zero-allocation reuse of Plain objects in hot paths.
	GeneratePool bool
	// CastersAsStruct controls how casters are passed to IntoPlain/IntoPb methods:
	// - true (default): pass as struct parameter, e.g. IntoPlain(c *MsgCasters)
	// - false: pass as separate arguments, e.g. IntoPlain(fieldACaster cast.Caster[A,B], ...)
	CastersAsStruct bool
	// UnifiedOneofJSON controls JSON naming for oneof variant fields:
	// - true: all variants use original field name in JSON (e.g., "target" for all)
	//         Go struct has unique names (ProcessCreateTarget, ProcessExecTarget)
	//         Unmarshal uses oneof case field to dispatch to correct Go field
	// - false (default): Go field name is used in JSON (with variant prefix)
	UnifiedOneofJSON bool
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
		JSONJX:           mapGetOrDefault(paramsMap, "json_jx", "false") == "true",
		JXPB:             mapGetOrDefault(paramsMap, "jx_pb", "false") == "true",
		SparseJSON:       mapGetOrDefault(paramsMap, "sparse_json", "true") == "true", // default true for backward compat
		GeneratePool:     mapGetOrDefault(paramsMap, "pool", "false") == "true",
		CastersAsStruct:  mapGetOrDefault(paramsMap, "casters_as_struct", "true") == "true", // default true
		UnifiedOneofJSON: mapGetOrDefault(paramsMap, "unified_oneof_json", "false") == "true",
	}
	return settings, nil
}
