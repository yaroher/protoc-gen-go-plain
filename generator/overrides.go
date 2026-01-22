package generator

import (
	"path"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/generator/empath"
	"github.com/yaroher/protoc-gen-go-plain/generator/marker"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/known/typepb"
)

const (
	overrideTypeNameMarker   = "override_type"
	overrideTypeImportMarker = "override_import"
)

type overrideInfo struct {
	name       string
	importPath string
}

func (g *Generator) castersTypeName(ir *TypePbIR) string {
	if ir == nil || ir.File == nil {
		return "PlainCasters"
	}
	base := path.Base(ir.File.GeneratedFilenamePrefix)
	if base == "" {
		base = "Plain"
	}
	return strcase.ToCamel(base) + "PlainCasters"
}

func (g *Generator) overrideInfo(field *typepb.Field) (overrideInfo, bool) {
	for _, segment := range empath.Parse(field.TypeUrl) {
		if name := segment.GetMarker(overrideTypeNameMarker); name != "" {
			return overrideInfo{
				name:       decodeEmpath(name),
				importPath: decodeEmpath(segment.GetMarker(overrideTypeImportMarker)),
			}, true
		}
	}
	return overrideInfo{}, false
}

func (g *Generator) resolveFieldOverride(
	field *protogen.Field,
	fieldOverride *goplain.GoIdent,
	fileOverrides []*goplain.TypeOverride,
) *goplain.GoIdent {
	if fieldOverride != nil && fieldOverride.GetName() != "" {
		return fieldOverride
	}
	for _, override := range fileOverrides {
		if g.matchOverride(override.GetSelector(), field) {
			return override.GetTargetGoType()
		}
	}
	for _, override := range g.overrides {
		if g.matchOverride(override.GetSelector(), field) {
			return override.GetTargetGoType()
		}
	}
	return nil
}

func (g *Generator) matchOverride(sel *goplain.OverrideSelector, field *protogen.Field) bool {
	if sel == nil || field == nil {
		return false
	}
	if sel.TargetFullPath != nil && *sel.TargetFullPath != string(field.Desc.FullName()) {
		return false
	}
	if sel.FieldKind != nil && typepb.Field_Kind(field.Desc.Kind()) != *sel.FieldKind {
		return false
	}
	if sel.FieldCardinality != nil && typepb.Field_Cardinality(field.Desc.Cardinality()) != *sel.FieldCardinality {
		return false
	}
	if sel.FieldTypeUrl != nil && g.fieldTypeURL(field) != *sel.FieldTypeUrl {
		return false
	}
	return true
}

func (g *Generator) fieldTypeURL(field *protogen.Field) string {
	if field == nil {
		return ""
	}
	if field.Message != nil {
		return string(field.Message.Desc.FullName())
	}
	if field.Enum != nil {
		return string(field.Enum.Desc.FullName())
	}
	return ""
}

func (g *Generator) applyOverrideMarkers(field *typepb.Field, override *goplain.GoIdent) {
	if field == nil || override == nil || override.GetName() == "" {
		return
	}
	overrideName := override.GetName()
	overrideImport := override.GetImportPath()
	if overrideImport != "" && !strings.Contains(overrideName, ".") {
		overrideName = path.Base(overrideImport) + "." + overrideName
	}
	markers := map[string]string{overrideTypeNameMarker: encodeMarkerValue(overrideName)}
	if overrideImport != "" {
		markers[overrideTypeImportMarker] = encodeMarkerValue(overrideImport)
	}
	base := field.TypeUrl
	if base == "" {
		base = field.Name
	}
	segments := empath.Parse(base)
	if len(segments) == 0 {
		segments = empath.New(marker.Parse(field.Name))
	}
	lastIdx := len(segments) - 1
	segments[lastIdx] = segments[lastIdx].AddMarkers(markers)
	field.TypeUrl = segments.String()
}
