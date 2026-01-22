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

		msgNames := make([]string, 0, len(ir.Messages))
		for name := range ir.Messages {
			msgNames = append(msgNames, name)
		}
		sort.Strings(msgNames)

		out.P("package ", ir.File.GoPackageName)
		out.P()
		out.P("import (")
		out.P(`"strings"`)
		out.P(`"github.com/yaroher/protoc-gen-go-plain/into"`)
		out.P(")")
		out.P()

		for _, name := range msgNames {
			msg := ir.Messages[name]
			g.renderConvertersForMessage(out, ir, msg)
			out.P()
		}

	}
	return nil
}

func (g *Generator) renderConvertersForMessage(out typeWriter, ir *TypePbIR, msg *typepb.Type) {
	if !g.isPbMessage(ir, msg.Name) {
		return
	}
	msgPlain := g.plainTypeName(msg.Name)
	msgPb := strcase.ToCamel(getShortName(msg.Name))

	out.P("func (x *", msgPlain, ") IntoPb() *", msgPb, " {")
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

		switch field.Kind {
		case typepb.Field_TYPE_MESSAGE:
			out.P("\tif x.", fieldName, " != nil {")
			out.P("\t\tinto.SetMessage(out, ", pathVar, ", x.", fieldName, ".IntoPb())")
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

	for _, oneof := range g.collectOneofFieldNames(msg) {
		out.P("\tout.", oneof.fieldName, " = x.", oneof.fieldName)
	}

	out.P("\treturn out")
	out.P("}")
	out.P()

	out.P("func (x *", msgPb, ") IntoPlain() *", msgPlain, " {")
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
			out.P("\t\t\tout.", fieldName, " = mv.IntoPlain()")
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

	for _, oneof := range g.collectOneofFieldNames(msg) {
		out.P("\tout.", oneof.fieldName, " = x.", oneof.fieldName)
	}

	out.P("\treturn out")
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
