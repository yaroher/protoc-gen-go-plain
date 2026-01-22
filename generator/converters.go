package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/generator/empath"
	"google.golang.org/protobuf/types/known/typepb"
)

func (g *Generator) RenderConverters(typeIRs []*TypePbIR) error {
	for _, ir := range typeIRs {
		if ir.File == nil {
			continue
		}
		outName := ir.File.GeneratedFilenamePrefix + "_plain.conv.go"
		out := g.Plugin.NewGeneratedFile(outName, ir.File.GoImportPath)

		messageCasterTypes := g.collectMessageCasterTypes(ir)
		fileCasterTypes := g.collectFileCasterTypes(messageCasterTypes)

		msgNames := make([]string, 0, len(ir.Messages))
		for name := range ir.Messages {
			msgNames = append(msgNames, name)
		}
		sort.Strings(msgNames)

		out.P("package ", ir.File.GoPackageName)
		out.P()
		imports := map[string]struct{}{
			"strings": {},
			"github.com/yaroher/protoc-gen-go-plain/into": {},
		}
		if len(fileCasterTypes.list) > 0 {
			imports["fmt"] = struct{}{}
			imports["github.com/yaroher/protoc-gen-go-plain/cast"] = struct{}{}
			for _, ct := range fileCasterTypes.list {
				if ct.importPath != "" {
					imports[ct.importPath] = struct{}{}
				}
				if ct.needsProtoReflect {
					imports["google.golang.org/protobuf/reflect/protoreflect"] = struct{}{}
				}
			}
		}
		out.P("import (")
		paths := make([]string, 0, len(imports))
		for path := range imports {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			out.P(fmt.Sprintf("%q", path))
		}
		out.P(")")
		out.P()

		for _, name := range msgNames {
			msg := ir.Messages[name]
			casterTypes := messageCasterTypes[msg.Name]
			g.renderConvertersForMessage(out, ir, msg, casterTypes, messageCasterTypes)
			out.P()
		}

	}
	return nil
}

func (g *Generator) renderConvertersForMessage(out typeWriter, ir *TypePbIR, msg *typepb.Type, casterTypes casterTypes, messageCasterTypes map[string]casterTypes) {
	if !g.isPbMessage(ir, msg.Name) {
		return
	}
	msgPlain := g.plainTypeName(msg.Name)
	msgPb := strcase.ToCamel(getShortName(msg.Name))
	paramList := g.casterParamList(casterTypes.list, true, false)
	paramListErr := g.casterParamList(casterTypes.list, true, true)

	out.P("func (x *", msgPlain, ") IntoPb(", paramList, ") *", msgPb, " {")
	out.P("\tif x == nil {")
	out.P("\t\treturn nil")
	out.P("\t}")
	out.P("\tout := &", msgPb, "{}")

	for _, field := range msg.Fields {
		if field.OneofIndex > 0 {
			continue
		}
		path := g.fieldPath(field)
		if len(path) == 0 {
			continue
		}
		fieldName := g.fieldGoName(field)

		pathVar := g.pathVarName(field)
		out.P("\t", pathVar, " := []string{", quoteSlice(path), "}")
		crfGo := ""
		if hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker) {
			crfGo = goFieldNameFromPlain(g.plainName(field) + "CRF")
			out.P("\tif x.", crfGo, " != \"\" {")
			out.P("\t\t", pathVar, " = into.ParseCRFPath(x.", crfGo, ")")
			out.P("\t}")
		}

		if g.hasOverride(field) {
			g.renderOverrideIntoPb(out, ir, msg, field, fieldName, pathVar, casterTypes)
		} else {
			switch field.Kind {
			case typepb.Field_TYPE_MESSAGE:
				out.P("\tif x.", fieldName, " != nil {")
				childArgs := g.childCasterArgs(ir, field, messageCasterTypes, true)
				out.P("\t\tinto.SetMessage(out, ", pathVar, ", x.", fieldName, ".IntoPb(", childArgs, "))")
				out.P("\t}")
			case typepb.Field_TYPE_STRING:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetString", pathVar)
			case typepb.Field_TYPE_BOOL:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetBool", pathVar)
			case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetInt32", pathVar)
			case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetUint32", pathVar)
			case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetInt64", pathVar)
			case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetUint64", pathVar)
			case typepb.Field_TYPE_FLOAT:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetFloat32", pathVar)
			case typepb.Field_TYPE_DOUBLE:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetFloat64", pathVar)
			case typepb.Field_TYPE_BYTES:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetBytes", pathVar)
			case typepb.Field_TYPE_ENUM:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetEnum", pathVar)
			default:
				// skip unsupported types for now
			}
		}
	}

	for _, oneof := range g.collectOneofFieldNames(msg) {
		out.P("\tout.", oneof.fieldName, " = x.", oneof.fieldName)
	}

	out.P("\treturn out")
	out.P("}")
	out.P()

	paramListPlain := g.casterParamList(casterTypes.list, false, false)
	paramListPlainErr := g.casterParamList(casterTypes.list, false, true)
	out.P("func (x *", msgPb, ") IntoPlain(", paramListPlain, ") *", msgPlain, " {")
	out.P("\tif x == nil {")
	out.P("\t\treturn nil")
	out.P("\t}")
	out.P("\tout := &", msgPlain, "{}")

	for _, field := range msg.Fields {
		if field.OneofIndex > 0 {
			continue
		}
		path := g.fieldPath(field)
		if len(path) == 0 {
			continue
		}
		fieldName := g.fieldGoName(field)

		pathVar := g.pathVarName(field)
		out.P("\t", pathVar, " := []string{", quoteSlice(path), "}")

		if g.hasOverride(field) {
			g.renderOverrideIntoPlain(out, ir, msg, field, fieldName, pathVar, casterTypes)
		} else {
			switch field.Kind {
			case typepb.Field_TYPE_MESSAGE:
				out.P("\tif v, ok := into.GetMessage(x, ", pathVar, "); ok {")
				pbType := g.resolvePbTypeName(ir, field.TypeUrl)
				if pbType == "" {
					out.P("\t\t// skip virtual types in converters")
					out.P("\t\t_ = v")
					out.P("\t}")
					break
				}
				out.P("\t\tif mv, ok := v.(*", pbType, "); ok {")
				childArgs := g.childCasterArgs(ir, field, messageCasterTypes, false)
				out.P("\t\t\tout.", fieldName, " = mv.IntoPlain(", childArgs, ")")
				out.P("\t\t}")
				if hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker) {
					out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
				}
				out.P("\t}")
			case typepb.Field_TYPE_STRING:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetString", pathVar)
			case typepb.Field_TYPE_BOOL:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetBool", pathVar)
			case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetInt32", pathVar)
			case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetUint32", pathVar)
			case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetInt64", pathVar)
			case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetUint64", pathVar)
			case typepb.Field_TYPE_FLOAT:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetFloat32", pathVar)
			case typepb.Field_TYPE_DOUBLE:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetFloat64", pathVar)
			case typepb.Field_TYPE_BYTES:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetBytes", pathVar)
			case typepb.Field_TYPE_ENUM:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetEnum", pathVar)
			default:
				// skip unsupported types for now
			}
		}
	}

	for _, oneof := range g.collectOneofFieldNames(msg) {
		out.P("\tout.", oneof.fieldName, " = x.", oneof.fieldName)
	}

	out.P("\treturn out")
	out.P("}")

	out.P()
	out.P("func (x *", msgPlain, ") IntoPbErr(", paramListErr, ") (*", msgPb, ", error) {")
	out.P("\tif x == nil {")
	out.P("\t\treturn nil, nil")
	out.P("\t}")
	out.P("\tout := &", msgPb, "{}")

	for _, field := range msg.Fields {
		if field.OneofIndex > 0 {
			continue
		}
		path := g.fieldPath(field)
		if len(path) == 0 {
			continue
		}
		fieldName := g.fieldGoName(field)

		pathVar := g.pathVarName(field)
		out.P("\t", pathVar, " := []string{", quoteSlice(path), "}")
		crfGo := ""
		if hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker) {
			crfGo = goFieldNameFromPlain(g.plainName(field) + "CRF")
			out.P("\tif x.", crfGo, " != \"\" {")
			out.P("\t\t", pathVar, " = into.ParseCRFPath(x.", crfGo, ")")
			out.P("\t}")
		}

		if g.hasOverride(field) {
			g.renderOverrideIntoPbErr(out, ir, msg, field, fieldName, pathVar, casterTypes)
		} else {
			switch field.Kind {
			case typepb.Field_TYPE_MESSAGE:
				out.P("\tif x.", fieldName, " != nil {")
				childArgs := g.childCasterArgs(ir, field, messageCasterTypes, true)
				out.P("\t\tmv, err := x.", fieldName, ".IntoPbErr(", childArgs, ")")
				out.P("\t\tif err != nil {")
				out.P("\t\t\treturn nil, err")
				out.P("\t\t}")
				out.P("\t\tinto.SetMessage(out, ", pathVar, ", mv)")
				out.P("\t}")
			case typepb.Field_TYPE_STRING:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetString", pathVar)
			case typepb.Field_TYPE_BOOL:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetBool", pathVar)
			case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetInt32", pathVar)
			case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetUint32", pathVar)
			case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetInt64", pathVar)
			case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetUint64", pathVar)
			case typepb.Field_TYPE_FLOAT:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetFloat32", pathVar)
			case typepb.Field_TYPE_DOUBLE:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetFloat64", pathVar)
			case typepb.Field_TYPE_BYTES:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetBytes", pathVar)
			case typepb.Field_TYPE_ENUM:
				g.renderScalarIntoPb(out, field, fieldName, "into.SetEnum", pathVar)
			default:
				// skip unsupported types for now
			}
		}
	}

	for _, oneof := range g.collectOneofFieldNames(msg) {
		out.P("\tout.", oneof.fieldName, " = x.", oneof.fieldName)
	}

	out.P("\treturn out, nil")
	out.P("}")
	out.P()

	out.P("func (x *", msgPb, ") IntoPlainErr(", paramListPlainErr, ") (*", msgPlain, ", error) {")
	out.P("\tif x == nil {")
	out.P("\t\treturn nil, nil")
	out.P("\t}")
	out.P("\tout := &", msgPlain, "{}")

	for _, field := range msg.Fields {
		if field.OneofIndex > 0 {
			continue
		}
		path := g.fieldPath(field)
		if len(path) == 0 {
			continue
		}
		fieldName := g.fieldGoName(field)

		pathVar := g.pathVarName(field)
		out.P("\t", pathVar, " := []string{", quoteSlice(path), "}")

		if g.hasOverride(field) {
			g.renderOverrideIntoPlainErr(out, ir, msg, field, fieldName, pathVar, casterTypes)
		} else {
			switch field.Kind {
			case typepb.Field_TYPE_MESSAGE:
				out.P("\tif v, ok := into.GetMessage(x, ", pathVar, "); ok {")
				pbType := g.resolvePbTypeName(ir, field.TypeUrl)
				if pbType == "" {
					out.P("\t\t// skip virtual types in converters")
					out.P("\t\t_ = v")
					out.P("\t}")
					break
				}
				out.P("\t\tif mv, ok := v.(*", pbType, "); ok {")
				childArgs := g.childCasterArgs(ir, field, messageCasterTypes, false)
				out.P("\t\t\tplainVal, err := mv.IntoPlainErr(", childArgs, ")")
				out.P("\t\t\tif err != nil {")
				out.P("\t\t\t\treturn nil, err")
				out.P("\t\t\t}")
				out.P("\t\t\tout.", fieldName, " = plainVal")
				out.P("\t\t}")
				if hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker) {
					out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
				}
				out.P("\t}")
			case typepb.Field_TYPE_STRING:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetString", pathVar)
			case typepb.Field_TYPE_BOOL:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetBool", pathVar)
			case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetInt32", pathVar)
			case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetUint32", pathVar)
			case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetInt64", pathVar)
			case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetUint64", pathVar)
			case typepb.Field_TYPE_FLOAT:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetFloat32", pathVar)
			case typepb.Field_TYPE_DOUBLE:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetFloat64", pathVar)
			case typepb.Field_TYPE_BYTES:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetBytes", pathVar)
			case typepb.Field_TYPE_ENUM:
				g.renderScalarIntoPlain(out, field, fieldName, "into.GetEnum", pathVar)
			default:
				// skip unsupported types for now
			}
		}
	}

	for _, oneof := range g.collectOneofFieldNames(msg) {
		out.P("\tout.", oneof.fieldName, " = x.", oneof.fieldName)
	}

	out.P("\treturn out, nil")
	out.P("}")
}

func (g *Generator) renderScalarIntoPb(out typeWriter, field *typepb.Field, fieldName, fn string, pathVar string) {
	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		out.P("\tif len(x.", fieldName, ") > 0 {")
		out.P("\t\t", fn, "List(out, ", pathVar, ", x.", fieldName, ")")
		out.P("\t}")
		return
	}

	if g.isPointerField(field) {
		out.P("\tif x.", fieldName, " != nil {")
		out.P("\t\t", fn, "(out, ", pathVar, ", *x.", fieldName, ")")
		out.P("\t}")
		return
	}

	out.P("\t", fn, "(out, ", pathVar, ", x.", fieldName, ")")
}

func (g *Generator) renderScalarIntoPlain(out typeWriter, field *typepb.Field, fieldName, fn string, pathVar string) {
	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		out.P("\tif v, ok := ", fn, "List(x, ", pathVar, "); ok {")
		out.P("\t\tout.", fieldName, " = v")
		out.P("\t}")
		return
	}

	if g.isPointerField(field) {
		out.P("\tif v, ok := ", fn, "(x, ", pathVar, "); ok {")
		out.P("\t\tout.", fieldName, " = &v")
		if hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker) {
			out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
		}
		out.P("\t}")
		return
	}

	out.P("\tif v, ok := ", fn, "(x, ", pathVar, "); ok {")
	out.P("\t\tout.", fieldName, " = v")
	if hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker) {
		out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
	}
	out.P("\t}")
}

func (g *Generator) isPointerField(field *typepb.Field) bool {
	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		return false
	}
	return hasMarker(field.TypeUrl, isOneoffedMarker) ||
		(hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker))
}

func (g *Generator) fieldPath(field *typepb.Field) []string {
	raw := ""
	for _, segment := range empath.Parse(field.TypeUrl) {
		if v := segment.GetMarker(empathMarker); v != "" {
			raw = v
			break
		}
	}
	if raw == "" {
		raw = field.Name
	} else {
		raw = decodeEmpath(raw)
	}
	path := empath.Parse(raw)
	var parts []string
	for _, segment := range path {
		if segment.HasMarker(isOneoffedMarker) {
			continue
		}
		name := getShortName(segment.Value())
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	return parts
}

func (g *Generator) pathVarName(field *typepb.Field) string {
	return "_path" + strcase.ToCamel(g.plainName(field))
}

func quoteSlice(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%q", p))
	}
	return b.String()
}

func (g *Generator) resolvePbTypeName(ir *TypePbIR, typeURL string) string {
	target := empath.Parse(typeURL).Last().Value()
	for _, m := range ir.Messages {
		if empath.Parse(m.Name).Last().Value() == target {
			if !g.isPbMessage(ir, m.Name) {
				return ""
			}
			return strcase.ToCamel(getShortName(m.Name))
		}
	}
	return strcase.ToCamel(getShortName(target))
}

func (g *Generator) isPbMessage(ir *TypePbIR, fullName string) bool {
	if ir == nil || ir.File == nil {
		return false
	}
	target := getShortName(fullName)
	for _, m := range ir.File.Messages {
		if string(m.Desc.Name()) == target {
			return true
		}
	}
	return false
}

// helper functions are in package into

type casterTypes struct {
	list  []casterTypeInfo
	byKey map[string]casterTypeInfo
}

type casterTypeInfo struct {
	pbElemType        string
	plainElemType     string
	importPath        string
	needsProtoReflect bool
}

func (g *Generator) collectMessageCasterTypes(ir *TypePbIR) map[string]casterTypes {
	result := make(map[string]casterTypes)
	if ir == nil {
		return result
	}
	visiting := make(map[string]bool)
	for _, msg := range ir.Messages {
		_ = g.messageCasterTypes(ir, msg, visiting, result)
	}
	return result
}

func (g *Generator) collectFileCasterTypes(messageCasterTypes map[string]casterTypes) casterTypes {
	result := casterTypes{byKey: make(map[string]casterTypeInfo)}
	for _, ct := range messageCasterTypes {
		result = g.mergeCasterTypes(result, ct)
	}
	result.list = g.sortedCasterTypes(result.byKey)
	return result
}

func (g *Generator) messageCasterTypes(ir *TypePbIR, msg *typepb.Type, visiting map[string]bool, cache map[string]casterTypes) casterTypes {
	if msg == nil {
		return casterTypes{byKey: make(map[string]casterTypeInfo)}
	}
	if cached, ok := cache[msg.Name]; ok {
		return cached
	}
	if visiting[msg.Name] {
		return casterTypes{byKey: make(map[string]casterTypeInfo)}
	}
	visiting[msg.Name] = true
	result := casterTypes{byKey: make(map[string]casterTypeInfo)}
	for _, field := range msg.Fields {
		if field.OneofIndex > 0 {
			continue
		}
		if override, ok := g.overrideInfo(field); ok {
			pbElemType, needsProtoReflect := g.overridePbElemType(ir, field)
			result = g.addCasterType(result, pbElemType, override.name, override.importPath, needsProtoReflect)
			continue
		}
		if field.Kind == typepb.Field_TYPE_MESSAGE {
			embedded, ok := g.findMessage(ir, field.TypeUrl)
			if !ok {
				continue
			}
			child := g.messageCasterTypes(ir, embedded, visiting, cache)
			result = g.mergeCasterTypes(result, child)
		}
	}
	result.list = g.sortedCasterTypes(result.byKey)
	cache[msg.Name] = result
	visiting[msg.Name] = false
	return result
}

func (g *Generator) addCasterType(
	dst casterTypes,
	pbElemType, plainElemType, importPath string,
	needsProtoReflect bool,
) casterTypes {
	if dst.byKey == nil {
		dst.byKey = make(map[string]casterTypeInfo)
	}
	key := g.casterTypeKey(pbElemType, plainElemType)
	if _, ok := dst.byKey[key]; ok {
		return dst
	}
	dst.byKey[key] = casterTypeInfo{
		pbElemType:        pbElemType,
		plainElemType:     plainElemType,
		importPath:        importPath,
		needsProtoReflect: needsProtoReflect,
	}
	return dst
}

func (g *Generator) mergeCasterTypes(dst, src casterTypes) casterTypes {
	for key, ct := range src.byKey {
		if _, ok := dst.byKey[key]; ok {
			continue
		}
		dst.byKey[key] = ct
	}
	return dst
}

func (g *Generator) sortedCasterTypes(byKey map[string]casterTypeInfo) []casterTypeInfo {
	if len(byKey) == 0 {
		return nil
	}
	list := make([]casterTypeInfo, 0, len(byKey))
	for _, ct := range byKey {
		list = append(list, ct)
	}
	sort.Slice(list, func(i, j int) bool {
		left := g.casterTypeKey(list[i].pbElemType, list[i].plainElemType)
		right := g.casterTypeKey(list[j].pbElemType, list[j].plainElemType)
		return left < right
	})
	return list
}

func (g *Generator) hasOverride(field *typepb.Field) bool {
	_, ok := g.overrideInfo(field)
	return ok
}

func (g *Generator) casterTypeKey(pbType, plainType string) string {
	return pbType + "|" + plainType
}

func (g *Generator) casterTypeIdent(t string) string {
	ident := t
	ident = strings.ReplaceAll(ident, "[]", "Slice")
	ident = strings.ReplaceAll(ident, "*", "Ptr")
	ident = strings.ReplaceAll(ident, ".", "_")
	ident = strings.ReplaceAll(ident, " ", "")
	return strcase.ToCamel(ident)
}

func (g *Generator) casterParamNameForTypes(pbType, plainType string, toPb bool) string {
	if toPb {
		return "caster" + g.casterTypeIdent(plainType) + "To" + g.casterTypeIdent(pbType)
	}
	return "caster" + g.casterTypeIdent(pbType) + "To" + g.casterTypeIdent(plainType)
}

func (g *Generator) casterParamList(list []casterTypeInfo, toPb, withErr bool) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	for i, ct := range list {
		if i > 0 {
			b.WriteString(", ")
		}
		casterType := "cast.Caster"
		if withErr {
			casterType = "cast.CasterErr"
		}
		paramName := g.casterParamNameForTypes(ct.pbElemType, ct.plainElemType, toPb)
		if toPb {
			fmt.Fprintf(&b, "%s %s[%s, %s]", paramName, casterType, ct.plainElemType, ct.pbElemType)
		} else {
			fmt.Fprintf(&b, "%s %s[%s, %s]", paramName, casterType, ct.pbElemType, ct.plainElemType)
		}
	}
	return b.String()
}

func (g *Generator) casterArgList(list []casterTypeInfo, toPb bool) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	for i, ct := range list {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(g.casterParamNameForTypes(ct.pbElemType, ct.plainElemType, toPb))
	}
	return b.String()
}

func (g *Generator) casterParamName(ir *TypePbIR, field *typepb.Field, casterTypes casterTypes, toPb bool) string {
	override, ok := g.overrideInfo(field)
	if !ok {
		return ""
	}
	pbElemType, _ := g.overridePbElemType(ir, field)
	key := g.casterTypeKey(pbElemType, override.name)
	if ct, ok := casterTypes.byKey[key]; ok {
		return g.casterParamNameForTypes(ct.pbElemType, ct.plainElemType, toPb)
	}
	return ""
}

func (g *Generator) childCasterArgs(
	ir *TypePbIR,
	field *typepb.Field,
	messageCasterTypes map[string]casterTypes,
	toPb bool,
) string {
	if field == nil || field.Kind != typepb.Field_TYPE_MESSAGE {
		return ""
	}
	embedded, ok := g.findMessage(ir, field.TypeUrl)
	if !ok {
		return ""
	}
	child := messageCasterTypes[embedded.Name]
	return g.casterArgList(child.list, toPb)
}

func (g *Generator) overridePbElemType(ir *TypePbIR, field *typepb.Field) (string, bool) {
	switch field.Kind {
	case typepb.Field_TYPE_DOUBLE:
		return "float64", false
	case typepb.Field_TYPE_FLOAT:
		return "float32", false
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "int64", false
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "uint64", false
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "int32", false
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "uint32", false
	case typepb.Field_TYPE_BOOL:
		return "bool", false
	case typepb.Field_TYPE_STRING:
		return "string", false
	case typepb.Field_TYPE_BYTES:
		return "[]byte", false
	case typepb.Field_TYPE_ENUM:
		return "protoreflect.EnumNumber", true
	case typepb.Field_TYPE_MESSAGE:
		pbType := g.resolvePbTypeName(ir, field.TypeUrl)
		if pbType == "" {
			pbType = strcase.ToCamel(getShortName(field.Name))
		}
		return "*" + pbType, false
	default:
		return "any", false
	}
}

func (g *Generator) scalarSetter(kind typepb.Field_Kind) string {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "into.SetString"
	case typepb.Field_TYPE_BOOL:
		return "into.SetBool"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "into.SetInt32"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "into.SetUint32"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "into.SetInt64"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "into.SetUint64"
	case typepb.Field_TYPE_FLOAT:
		return "into.SetFloat32"
	case typepb.Field_TYPE_DOUBLE:
		return "into.SetFloat64"
	case typepb.Field_TYPE_BYTES:
		return "into.SetBytes"
	case typepb.Field_TYPE_ENUM:
		return "into.SetEnum"
	default:
		return ""
	}
}

func (g *Generator) scalarListSetter(kind typepb.Field_Kind) string {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "into.SetStringList"
	case typepb.Field_TYPE_BOOL:
		return "into.SetBoolList"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "into.SetInt32List"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "into.SetUint32List"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "into.SetInt64List"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "into.SetUint64List"
	case typepb.Field_TYPE_FLOAT:
		return "into.SetFloat32List"
	case typepb.Field_TYPE_DOUBLE:
		return "into.SetFloat64List"
	case typepb.Field_TYPE_BYTES:
		return "into.SetBytesList"
	case typepb.Field_TYPE_ENUM:
		return "into.SetEnumList"
	default:
		return ""
	}
}

func (g *Generator) scalarGetter(kind typepb.Field_Kind) string {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "into.GetString"
	case typepb.Field_TYPE_BOOL:
		return "into.GetBool"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "into.GetInt32"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "into.GetUint32"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "into.GetInt64"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "into.GetUint64"
	case typepb.Field_TYPE_FLOAT:
		return "into.GetFloat32"
	case typepb.Field_TYPE_DOUBLE:
		return "into.GetFloat64"
	case typepb.Field_TYPE_BYTES:
		return "into.GetBytes"
	case typepb.Field_TYPE_ENUM:
		return "into.GetEnum"
	default:
		return ""
	}
}

func (g *Generator) scalarListGetter(kind typepb.Field_Kind) string {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "into.GetStringList"
	case typepb.Field_TYPE_BOOL:
		return "into.GetBoolList"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "into.GetInt32List"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "into.GetUint32List"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "into.GetInt64List"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "into.GetUint64List"
	case typepb.Field_TYPE_FLOAT:
		return "into.GetFloat32List"
	case typepb.Field_TYPE_DOUBLE:
		return "into.GetFloat64List"
	case typepb.Field_TYPE_BYTES:
		return "into.GetBytesList"
	case typepb.Field_TYPE_ENUM:
		return "into.GetEnumList"
	default:
		return ""
	}
}

func (g *Generator) renderOverrideIntoPb(out typeWriter, ir *TypePbIR, msg *typepb.Type, field *typepb.Field, fieldName, pathVar string, casterTypes casterTypes) {
	casterName := g.casterParamName(ir, field, casterTypes, true)
	if casterName == "" {
		out.P("\t// missing caster mapping for override")
		return
	}
	setFn := g.scalarSetter(field.Kind)
	setListFn := g.scalarListSetter(field.Kind)
	if field.Kind == typepb.Field_TYPE_MESSAGE {
		if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
			out.P("\t// repeated message with override is not supported")
			return
		}
		if g.isPointerField(field) {
			out.P("\tif x.", fieldName, " != nil {")
			out.P("\t\tif ", casterName, " == nil {")
			out.P("\t\t\tpanic(\"missing caster: ", casterName, "\")")
			out.P("\t\t}")
			out.P("\t\tpbVal := ", casterName, ".Cast(*x.", fieldName, ")")
			out.P("\t\tif pbVal != nil {")
			out.P("\t\t\tinto.SetMessage(out, ", pathVar, ", pbVal)")
			out.P("\t\t}")
			out.P("\t}")
			return
		}
		out.P("\tif ", casterName, " == nil {")
		out.P("\t\tpanic(\"missing caster: ", casterName, "\")")
		out.P("\t}")
		out.P("\tpbVal := ", casterName, ".Cast(x.", fieldName, ")")
		out.P("\tif pbVal != nil {")
		out.P("\t\tinto.SetMessage(out, ", pathVar, ", pbVal)")
		out.P("\t}")
		return
	}

	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		out.P("\tif len(x.", fieldName, ") > 0 {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\tpanic(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		pbElemType, _ := g.overridePbElemType(ir, field)
		out.P("\t\tvals := make([]", pbElemType, ", len(x.", fieldName, "))")
		out.P("\t\tfor i, el := range x.", fieldName, " {")
		out.P("\t\t\tvals[i] = ", casterName, ".Cast(el)")
		out.P("\t\t}")
		if setListFn != "" {
			out.P("\t\t", setListFn, "(out, ", pathVar, ", vals)")
		}
		out.P("\t}")
		return
	}

	if g.isPointerField(field) {
		out.P("\tif x.", fieldName, " != nil {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\tpanic(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		out.P("\t\tval := ", casterName, ".Cast(*x.", fieldName, ")")
		if setFn != "" {
			out.P("\t\t", setFn, "(out, ", pathVar, ", val)")
		}
		out.P("\t}")
		return
	}

	out.P("\tif ", casterName, " == nil {")
	out.P("\t\tpanic(\"missing caster: ", casterName, "\")")
	out.P("\t}")
	out.P("\tval := ", casterName, ".Cast(x.", fieldName, ")")
	if setFn != "" {
		out.P("\t", setFn, "(out, ", pathVar, ", val)")
	}
}

func (g *Generator) renderOverrideIntoPlain(out typeWriter, ir *TypePbIR, msg *typepb.Type, field *typepb.Field, fieldName, pathVar string, casterTypes casterTypes) {
	casterName := g.casterParamName(ir, field, casterTypes, false)
	if casterName == "" {
		out.P("\t// missing caster mapping for override")
		return
	}
	getFn := g.scalarGetter(field.Kind)
	getListFn := g.scalarListGetter(field.Kind)
	crf := hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker)
	override, _ := g.overrideInfo(field)

	if field.Kind == typepb.Field_TYPE_MESSAGE {
		if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
			out.P("\t// repeated message with override is not supported")
			return
		}
		out.P("\tif v, ok := into.GetMessage(x, ", pathVar, "); ok {")
		pbType := g.resolvePbTypeName(ir, field.TypeUrl)
		if pbType == "" {
			out.P("\t\t// skip virtual types in converters")
			out.P("\t\t_ = v")
			out.P("\t}")
			return
		}
		out.P("\t\tif mv, ok := v.(*", pbType, "); ok {")
		out.P("\t\t\tif ", casterName, " == nil {")
		out.P("\t\t\t\tpanic(\"missing caster: ", casterName, "\")")
		out.P("\t\t\t}")
		if g.isPointerField(field) {
			out.P("\t\t\tval := ", casterName, ".Cast(mv)")
			out.P("\t\t\tout.", fieldName, " = &val")
		} else {
			out.P("\t\t\tout.", fieldName, " = ", casterName, ".Cast(mv)")
		}
		if crf {
			out.P("\t\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
		}
		out.P("\t\t}")
		out.P("\t}")
		return
	}

	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		if getListFn == "" {
			return
		}
		out.P("\tif v, ok := ", getListFn, "(x, ", pathVar, "); ok {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\tpanic(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		out.P("\t\tout.", fieldName, " = make([]", override.name, ", len(v))")
		out.P("\t\tfor i, el := range v {")
		out.P("\t\t\tout.", fieldName, "[i] = ", casterName, ".Cast(el)")
		out.P("\t\t}")
		if crf {
			out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
		}
		out.P("\t}")
		return
	}

	if g.isPointerField(field) {
		if getFn == "" {
			return
		}
		out.P("\tif v, ok := ", getFn, "(x, ", pathVar, "); ok {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\tpanic(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		out.P("\t\tval := ", casterName, ".Cast(v)")
		out.P("\t\tout.", fieldName, " = &val")
		if crf {
			out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
		}
		out.P("\t}")
		return
	}

	if getFn == "" {
		return
	}
	out.P("\tif v, ok := ", getFn, "(x, ", pathVar, "); ok {")
	out.P("\t\tif ", casterName, " == nil {")
	out.P("\t\t\tpanic(\"missing caster: ", casterName, "\")")
	out.P("\t\t}")
	out.P("\t\tout.", fieldName, " = ", casterName, ".Cast(v)")
	if crf {
		out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
	}
	out.P("\t}")
}

func (g *Generator) renderOverrideIntoPbErr(out typeWriter, ir *TypePbIR, msg *typepb.Type, field *typepb.Field, fieldName, pathVar string, casterTypes casterTypes) {
	casterName := g.casterParamName(ir, field, casterTypes, true)
	if casterName == "" {
		out.P("\t// missing caster mapping for override")
		return
	}
	setFn := g.scalarSetter(field.Kind)
	setListFn := g.scalarListSetter(field.Kind)
	if field.Kind == typepb.Field_TYPE_MESSAGE {
		if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
			out.P("\t// repeated message with override is not supported")
			return
		}
		if g.isPointerField(field) {
			out.P("\tif x.", fieldName, " != nil {")
			out.P("\t\tif ", casterName, " == nil {")
			out.P("\t\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
			out.P("\t\t}")
			out.P("\t\tpbVal, err := ", casterName, ".CastErr(*x.", fieldName, ")")
			out.P("\t\tif err != nil {")
			out.P("\t\t\treturn nil, err")
			out.P("\t\t}")
			out.P("\t\tif pbVal != nil {")
			out.P("\t\t\tinto.SetMessage(out, ", pathVar, ", pbVal)")
			out.P("\t\t}")
			out.P("\t}")
			return
		}
		out.P("\tif ", casterName, " == nil {")
		out.P("\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
		out.P("\t}")
		out.P("\tpbVal, err := ", casterName, ".CastErr(x.", fieldName, ")")
		out.P("\tif err != nil {")
		out.P("\t\treturn nil, err")
		out.P("\t}")
		out.P("\tif pbVal != nil {")
		out.P("\t\tinto.SetMessage(out, ", pathVar, ", pbVal)")
		out.P("\t}")
		return
	}

	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		out.P("\tif len(x.", fieldName, ") > 0 {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		pbElemType, _ := g.overridePbElemType(ir, field)
		out.P("\t\tvals := make([]", pbElemType, ", len(x.", fieldName, "))")
		out.P("\t\tfor i, el := range x.", fieldName, " {")
		out.P("\t\t\tval, err := ", casterName, ".CastErr(el)")
		out.P("\t\t\tif err != nil {")
		out.P("\t\t\t\treturn nil, err")
		out.P("\t\t\t}")
		out.P("\t\t\tvals[i] = val")
		out.P("\t\t}")
		if setListFn != "" {
			out.P("\t\t", setListFn, "(out, ", pathVar, ", vals)")
		}
		out.P("\t}")
		return
	}

	if g.isPointerField(field) {
		out.P("\tif x.", fieldName, " != nil {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		out.P("\t\tval, err := ", casterName, ".CastErr(*x.", fieldName, ")")
		out.P("\t\tif err != nil {")
		out.P("\t\t\treturn nil, err")
		out.P("\t\t}")
		if setFn != "" {
			out.P("\t\t", setFn, "(out, ", pathVar, ", val)")
		}
		out.P("\t}")
		return
	}

	out.P("\tif ", casterName, " == nil {")
	out.P("\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
	out.P("\t}")
	out.P("\tval, err := ", casterName, ".CastErr(x.", fieldName, ")")
	out.P("\tif err != nil {")
	out.P("\t\treturn nil, err")
	out.P("\t}")
	if setFn != "" {
		out.P("\t", setFn, "(out, ", pathVar, ", val)")
	}
}

func (g *Generator) renderOverrideIntoPlainErr(out typeWriter, ir *TypePbIR, msg *typepb.Type, field *typepb.Field, fieldName, pathVar string, casterTypes casterTypes) {
	casterName := g.casterParamName(ir, field, casterTypes, false)
	if casterName == "" {
		out.P("\t// missing caster mapping for override")
		return
	}
	getFn := g.scalarGetter(field.Kind)
	getListFn := g.scalarListGetter(field.Kind)
	crf := hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker)
	override, _ := g.overrideInfo(field)

	if field.Kind == typepb.Field_TYPE_MESSAGE {
		if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
			out.P("\t// repeated message with override is not supported")
			return
		}
		out.P("\tif v, ok := into.GetMessage(x, ", pathVar, "); ok {")
		pbType := g.resolvePbTypeName(ir, field.TypeUrl)
		if pbType == "" {
			out.P("\t\t// skip virtual types in converters")
			out.P("\t\t_ = v")
			out.P("\t}")
			return
		}
		out.P("\t\tif mv, ok := v.(*", pbType, "); ok {")
		out.P("\t\t\tif ", casterName, " == nil {")
		out.P("\t\t\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
		out.P("\t\t\t}")
		if g.isPointerField(field) {
			out.P("\t\t\tval, err := ", casterName, ".CastErr(mv)")
			out.P("\t\t\tif err != nil {")
			out.P("\t\t\t\treturn nil, err")
			out.P("\t\t\t}")
			out.P("\t\t\tout.", fieldName, " = &val")
		} else {
			out.P("\t\t\tval, err := ", casterName, ".CastErr(mv)")
			out.P("\t\t\tif err != nil {")
			out.P("\t\t\t\treturn nil, err")
			out.P("\t\t\t}")
			out.P("\t\t\tout.", fieldName, " = val")
		}
		if crf {
			out.P("\t\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
		}
		out.P("\t\t}")
		out.P("\t}")
		return
	}

	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		if getListFn == "" {
			return
		}
		out.P("\tif v, ok := ", getListFn, "(x, ", pathVar, "); ok {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		out.P("\t\tout.", fieldName, " = make([]", override.name, ", len(v))")
		out.P("\t\tfor i, el := range v {")
		out.P("\t\t\tval, err := ", casterName, ".CastErr(el)")
		out.P("\t\t\tif err != nil {")
		out.P("\t\t\t\treturn nil, err")
		out.P("\t\t\t}")
		out.P("\t\t\tout.", fieldName, "[i] = val")
		out.P("\t\t}")
		if crf {
			out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
		}
		out.P("\t}")
		return
	}

	if g.isPointerField(field) {
		if getFn == "" {
			return
		}
		out.P("\tif v, ok := ", getFn, "(x, ", pathVar, "); ok {")
		out.P("\t\tif ", casterName, " == nil {")
		out.P("\t\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
		out.P("\t\t}")
		out.P("\t\tval, err := ", casterName, ".CastErr(v)")
		out.P("\t\tif err != nil {")
		out.P("\t\t\treturn nil, err")
		out.P("\t\t}")
		out.P("\t\tout.", fieldName, " = &val")
		if crf {
			out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
		}
		out.P("\t}")
		return
	}

	if getFn == "" {
		return
	}
	out.P("\tif v, ok := ", getFn, "(x, ", pathVar, "); ok {")
	out.P("\t\tif ", casterName, " == nil {")
	out.P("\t\t\treturn nil, fmt.Errorf(\"missing caster: ", casterName, "\")")
	out.P("\t\t}")
	out.P("\t\tval, err := ", casterName, ".CastErr(v)")
	out.P("\t\tif err != nil {")
	out.P("\t\t\treturn nil, err")
	out.P("\t\t}")
	out.P("\t\tout.", fieldName, " = val")
	if crf {
		out.P("\t\tout.", goFieldNameFromPlain(g.plainName(field)+"CRF"), " = strings.Join(", pathVar, ", \"/\")")
	}
	out.P("\t}")
}
