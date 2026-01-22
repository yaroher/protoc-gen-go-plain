package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/generator/empath"
	"google.golang.org/protobuf/types/known/typepb"
)

func (g *Generator) Render(typeIRs []*TypePbIR) error {
	for _, ir := range typeIRs {
		if ir.File == nil {
			continue
		}
		outName := ir.File.GeneratedFilenamePrefix + "_plain.pb.go"
		out := g.Plugin.NewGeneratedFile(outName, ir.File.GoImportPath)

		imports := make(map[string]struct{})
		var body strings.Builder
		bodyWriter := bufferWriter{b: &body}

		msgNames := make([]string, 0, len(ir.Messages))
		for name := range ir.Messages {
			msgNames = append(msgNames, name)
		}
		sort.Strings(msgNames)

		for _, name := range msgNames {
			msg := ir.Messages[name]
			g.renderMessage(bodyWriter, ir, msg, imports)
			bodyWriter.P()
		}

		out.P("package ", ir.File.GoPackageName)
		out.P()

		if len(imports) > 0 {
			paths := make([]string, 0, len(imports))
			for path := range imports {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			out.P("import (")
			for _, path := range paths {
				out.P(fmt.Sprintf("%q", path))
			}
			out.P(")")
			out.P()
		}

		out.P(body.String())
	}
	return nil
}

func (g *Generator) renderMessage(out typeWriter, ir *TypePbIR, msg *typepb.Type, imports map[string]struct{}) {
	msgName := g.plainTypeName(msg.Name)
	out.P("type ", msgName, " struct {")

	oneofFieldNames := g.collectOneofFieldNames(msg)

	for _, field := range msg.Fields {
		if field.OneofIndex > 0 {
			continue
		}
		fieldName := g.fieldGoName(field)
		fieldType, tag := g.fieldGoTypeAndTag(ir, msg, field, imports)
		out.P("\t", fieldName, " ", fieldType, " `json:\"", tag, "\"`")
	}

	for _, oneofName := range oneofFieldNames {
		out.P("\t", oneofName.fieldName, " ", oneofName.fieldType, " `json:\"", oneofName.tag, "\"`")
	}

	out.P("}")
}

type oneofField struct {
	fieldName string
	fieldType string
	tag       string
}

func (g *Generator) collectOneofFieldNames(msg *typepb.Type) []oneofField {
	if len(msg.Oneofs) == 0 {
		return nil
	}
	msgShort := getShortName(msg.Name)
	msgGo := strcase.ToCamel(msgShort)
	var result []oneofField
	for _, oneof := range msg.Oneofs {
		oneofShort := getShortName(oneof)
		oneofGo := strcase.ToCamel(oneofShort)
		result = append(result, oneofField{
			fieldName: oneofGo,
			fieldType: "is" + msgGo + "_" + oneofGo,
			tag:       strcase.ToLowerCamel(oneofShort),
		})
	}
	return result
}

func (g *Generator) plainTypeName(fullName string) string {
	return strcase.ToCamel(getShortName(fullName)) + g.suffix
}

func (g *Generator) fieldGoName(field *typepb.Field) string {
	name := g.plainName(field)
	return goFieldNameFromPlain(name)
}

func (g *Generator) plainName(field *typepb.Field) string {
	path := empath.Parse(field.TypeUrl)
	for _, segment := range path {
		if pn := segment.GetMarker(plainName); pn != "" {
			return pn
		}
	}
	return getShortName(field.Name)
}

func (g *Generator) fieldGoTypeAndTag(ir *TypePbIR, msg *typepb.Type, field *typepb.Field, imports map[string]struct{}) (string, string) {
	isRepeated := field.Cardinality == typepb.Field_CARDINALITY_REPEATED
	isOneof := hasMarker(field.TypeUrl, isOneoffedMarker)
	isCRF := hasMarker(field.TypeUrl, crfMarker)
	isCRFField := hasMarker(field.TypeUrl, crfForMarker)

	goType := g.fieldGoType(ir, field, imports)
	tag := jsonTagFromPlain(g.plainName(field))

	if isRepeated {
		tag += ",omitempty"
	} else if strings.HasPrefix(goType, "*") || isOneof || (isCRF && !isCRFField) {
		tag += ",omitempty"
	}

	if isOneof && !isRepeated && !strings.HasPrefix(goType, "*") {
		goType = "*" + goType
	}

	return goType, tag
}

func (g *Generator) fieldGoType(ir *TypePbIR, field *typepb.Field, imports map[string]struct{}) string {
	isOptional := hasMarker(field.TypeUrl, isOneoffedMarker) ||
		(hasMarker(field.TypeUrl, crfMarker) && !hasMarker(field.TypeUrl, crfForMarker))
	if override, ok := g.overrideInfo(field); ok {
		if override.importPath != "" {
			imports[override.importPath] = struct{}{}
		}
		return g.wrapRepeatedOptional(field, override.name, isOptional)
	}
	switch field.Kind {
	case typepb.Field_TYPE_DOUBLE:
		return g.wrapRepeatedOptional(field, "float64", isOptional)
	case typepb.Field_TYPE_FLOAT:
		return g.wrapRepeatedOptional(field, "float32", isOptional)
	case typepb.Field_TYPE_INT64:
		return g.wrapRepeatedOptional(field, "int64", isOptional)
	case typepb.Field_TYPE_UINT64:
		return g.wrapRepeatedOptional(field, "uint64", isOptional)
	case typepb.Field_TYPE_INT32:
		return g.wrapRepeatedOptional(field, "int32", isOptional)
	case typepb.Field_TYPE_FIXED64:
		return g.wrapRepeatedOptional(field, "uint64", isOptional)
	case typepb.Field_TYPE_FIXED32:
		return g.wrapRepeatedOptional(field, "uint32", isOptional)
	case typepb.Field_TYPE_BOOL:
		return g.wrapRepeatedOptional(field, "bool", isOptional)
	case typepb.Field_TYPE_STRING:
		return g.wrapRepeatedOptional(field, "string", isOptional)
	case typepb.Field_TYPE_BYTES:
		return g.wrapRepeatedOptional(field, "[]byte", isOptional)
	case typepb.Field_TYPE_UINT32:
		return g.wrapRepeatedOptional(field, "uint32", isOptional)
	case typepb.Field_TYPE_SFIXED32:
		return g.wrapRepeatedOptional(field, "int32", isOptional)
	case typepb.Field_TYPE_SFIXED64:
		return g.wrapRepeatedOptional(field, "int64", isOptional)
	case typepb.Field_TYPE_SINT32:
		return g.wrapRepeatedOptional(field, "int32", isOptional)
	case typepb.Field_TYPE_SINT64:
		return g.wrapRepeatedOptional(field, "int64", isOptional)
	case typepb.Field_TYPE_ENUM:
		return g.wrapRepeatedOptional(field, "int32", isOptional)
	case typepb.Field_TYPE_MESSAGE:
		typeName := g.resolveMessageType(ir, field.TypeUrl)
		if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
			return "[]" + typeName
		}
		return "*" + typeName
	default:
		return g.wrapRepeated(field, "any")
	}
}

func (g *Generator) resolveMessageType(ir *TypePbIR, typeURL string) string {
	target := empath.Parse(typeURL).Last().Value()
	for _, m := range ir.Messages {
		if empath.Parse(m.Name).Last().Value() == target {
			return g.plainTypeName(m.Name)
		}
	}
	return strcase.ToCamel(getShortName(target)) + g.suffix
}

func (g *Generator) wrapRepeated(field *typepb.Field, base string) string {
	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		return "[]" + base
	}
	return base
}

func (g *Generator) wrapRepeatedOptional(field *typepb.Field, base string, optional bool) string {
	if field.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		return "[]" + base
	}
	if optional {
		return "*" + base
	}
	return base
}

func hasMarker(typeURL, key string) bool {
	if typeURL == "" {
		return false
	}
	path := empath.Parse(typeURL)
	for _, segment := range path {
		if segment.HasMarker(key) {
			return true
		}
	}
	return false
}

type typeWriter interface {
	P(v ...any)
}

type bufferWriter struct {
	b *strings.Builder
}

func (w bufferWriter) P(v ...any) {
	for _, part := range v {
		fmt.Fprint(w.b, part)
	}
	w.b.WriteByte('\n')
}
