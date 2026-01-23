package generator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/known/typepb"
)

type jxFieldInfo struct {
	Name        string
	GoName      string
	Kind        typepb.Field_Kind
	TypeURL     string
	Cardinality typepb.Field_Cardinality
	GoType      string

	IsMap         bool
	MapKeyKind    typepb.Field_Kind
	MapKeyType    string
	MapValKind    typepb.Field_Kind
	MapValType    string
	MapValTypeURL string
}

type jxStrconvRefs struct {
	FormatBool string
	FormatInt  string
	FormatUint string
	ParseBool  string
	ParseInt   string
	ParseUint  string
}

type jxRefs struct {
	EncoderType string
	DecoderType string
	Null        string
	DecodeBytes string
}

type jxJSONRefs struct {
	Marshal   string
	Unmarshal string
}

func (g *Generator) renderJSONJX(ctx *renderContext, plainName string, wrapper *TypeWrapper) error {
	if wrapper == nil {
		return nil
	}

	var infos []*jxFieldInfo
	useJSON := false
	useProtoJSON := false
	useStrconv := false

	for _, fw := range wrapper.Fields {
		if fw == nil || fw.Field == nil {
			continue
		}
		info, err := g.jxFieldInfo(ctx, fw)
		if err != nil {
			return err
		}
		if info == nil {
			continue
		}
		infos = append(infos, info)
		if info.IsMap && info.MapKeyKind != typepb.Field_TYPE_STRING {
			useStrconv = true
		}
		if info.IsMap && info.MapValKind != typepb.Field_TYPE_MESSAGE && jxNeedsJSONFallback(info.MapValKind) {
			useJSON = true
		}
		if !info.IsMap && info.Kind != typepb.Field_TYPE_MESSAGE && jxNeedsJSONFallback(info.Kind) {
			useJSON = true
		}
		if info.IsMap && info.MapValKind == typepb.Field_TYPE_MESSAGE && !g.isPlainMessage(ctx, info.MapValTypeURL) {
			if g.shouldUseProtoJSON(ctx, info.MapValTypeURL) {
				useProtoJSON = true
			}
		}
		if !info.IsMap && info.Kind == typepb.Field_TYPE_MESSAGE && !g.isPlainMessage(ctx, info.TypeURL) {
			if g.shouldUseProtoJSON(ctx, info.TypeURL) {
				useProtoJSON = true
			}
		}
	}

	jx := jxRefs{
		EncoderType: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Encoder", GoImportPath: "github.com/go-faster/jx"}),
		DecoderType: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Decoder", GoImportPath: "github.com/go-faster/jx"}),
		Null:        ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Null", GoImportPath: "github.com/go-faster/jx"}),
		DecodeBytes: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "DecodeBytes", GoImportPath: "github.com/go-faster/jx"}),
	}
	fmtErrorf := ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Errorf", GoImportPath: "fmt"})

	var jsonRefs *jxJSONRefs
	if useJSON {
		jsonRefs = &jxJSONRefs{
			Marshal:   ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Marshal", GoImportPath: "encoding/json"}),
			Unmarshal: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Unmarshal", GoImportPath: "encoding/json"}),
		}
	}
	var protoJSONRefs *jxJSONRefs
	if useProtoJSON {
		protoJSONRefs = &jxJSONRefs{
			Marshal:   ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Marshal", GoImportPath: "google.golang.org/protobuf/encoding/protojson"}),
			Unmarshal: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Unmarshal", GoImportPath: "google.golang.org/protobuf/encoding/protojson"}),
		}
	}
	var strconvRefs *jxStrconvRefs
	if useStrconv {
		strconvRefs = &jxStrconvRefs{
			FormatBool: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatBool", GoImportPath: "strconv"}),
			FormatInt:  ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatInt", GoImportPath: "strconv"}),
			FormatUint: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatUint", GoImportPath: "strconv"}),
			ParseBool:  ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseBool", GoImportPath: "strconv"}),
			ParseInt:   ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseInt", GoImportPath: "strconv"}),
			ParseUint:  ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseUint", GoImportPath: "strconv"}),
		}
	}

	ctx.g.P("// MarshalJX сериализует структуру в jx.Encoder")
	ctx.g.P("func (x *", plainName, ") MarshalJX(e *", jx.EncoderType, ") error {")
	ctx.g.P("\tif x == nil {")
	ctx.g.P("\t\te.Null()")
	ctx.g.P("\t\treturn nil")
	ctx.g.P("\t}")
	ctx.g.P("\te.ObjStart()")
	if wrapper.CRF != nil && wrapper.CRF.HasEntries() {
		ctx.g.P("\tif x.CRF != nil {")
		ctx.g.P("\t\te.FieldStart(\"crf\")")
		ctx.g.P("\t\tx.CRF.MarshalJX(e)")
		ctx.g.P("\t}")
	}
	for _, info := range infos {
		g.renderJXMarshalField(ctx, info, &jx, jsonRefs, protoJSONRefs, strconvRefs)
	}
	ctx.g.P("\te.ObjEnd()")
	ctx.g.P("\treturn nil")
	ctx.g.P("}")
	ctx.g.P()

	ctx.g.P("// UnmarshalJX десериализует структуру из jx.Decoder")
	ctx.g.P("func (x *", plainName, ") UnmarshalJX(d *", jx.DecoderType, ") error {")
	ctx.g.P("\tif d.Next() == ", jx.Null, " {")
	ctx.g.P("\t\treturn d.Null()")
	ctx.g.P("\t}")
	ctx.g.P("\treturn d.Obj(func(d *", jx.DecoderType, ", key string) error {")
	ctx.g.P("\t\tswitch key {")
	if wrapper.CRF != nil && wrapper.CRF.HasEntries() {
		crfIdent := protogen.GoIdent{GoName: "CRF", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/crf"}
		crfName := ctx.g.QualifiedGoIdent(crfIdent)
		ctx.g.P("\t\tcase \"crf\":")
		ctx.g.P("\t\t\t{")
		ctx.g.P("\t\t\t\tif d.Next() == ", jx.Null, " {")
		ctx.g.P("\t\t\t\t\tx.CRF = nil")
		ctx.g.P("\t\t\t\t\treturn d.Null()")
		ctx.g.P("\t\t\t\t}")
		ctx.g.P("\t\t\t\tcrfVal := new(", crfName, ")")
		ctx.g.P("\t\t\t\tif err := crfVal.UnmarshalJX(d); err != nil {") // Прямой вызов
		ctx.g.P("\t\t\t\t\treturn err")
		ctx.g.P("\t\t\t\t}")
		ctx.g.P("\t\t\t\tx.CRF = crfVal")
		ctx.g.P("\t\t\t\treturn nil")
		ctx.g.P("\t\t\t}")
	}
	for _, info := range infos {
		g.renderJXUnmarshalField(ctx, info, jx.DecoderType, jsonRefs, protoJSONRefs, strconvRefs, fmtErrorf, jx.Null)
	}
	ctx.g.P("\t\tdefault:")
	ctx.g.P("\t\t\treturn d.Skip()")
	ctx.g.P("\t\t}")
	ctx.g.P("\t})")
	ctx.g.P("}")
	ctx.g.P()

	return nil
}

func (g *Generator) renderProtoJSONJX(ctx *renderContext, msg *protogen.Message, plainName string, hasPlain bool) error {
	if msg == nil || msg.Desc.IsMapEntry() {
		return nil
	}

	jx := jxRefs{
		EncoderType: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Encoder", GoImportPath: "github.com/go-faster/jx"}),
		DecoderType: ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Decoder", GoImportPath: "github.com/go-faster/jx"}),
		Null:        ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Null", GoImportPath: "github.com/go-faster/jx"}),
	}
	fmtErrorf := ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Errorf", GoImportPath: "fmt"})

	msgName := msg.GoIdent.GoName
	if hasPlain {
		// Protobuf message с Plain - делегируем в Plain
		ctx.g.P("// MarshalJX сериализует protobuf сообщение через Plain в jx.Encoder")
		ctx.g.P("func (x *", msgName, ") MarshalJX(e *", jx.EncoderType, ") error {")
		ctx.g.P("\tif x == nil {")
		ctx.g.P("\t\te.Null()")
		ctx.g.P("\t\treturn nil")
		ctx.g.P("\t}")
		ctx.g.P("\tplain, err := x.IntoPlainErr()")
		ctx.g.P("\tif err != nil {")
		ctx.g.P("\t\treturn err")
		ctx.g.P("\t}")
		ctx.g.P("\treturn plain.MarshalJX(e)")
		ctx.g.P("}")
		ctx.g.P()

		ctx.g.P("// UnmarshalJX десериализует protobuf сообщение через Plain из jx.Decoder")
		ctx.g.P("func (x *", msgName, ") UnmarshalJX(d *", jx.DecoderType, ") error {")
		ctx.g.P("\tif x == nil {")
		ctx.g.P("\t\treturn ", fmtErrorf, "(\"", msgName, ": UnmarshalJX on nil pointer\")")
		ctx.g.P("\t}")
		ctx.g.P("\tif d.Next() == ", jx.Null, " {")
		ctx.g.P("\t\treturn d.Null()")
		ctx.g.P("\t}")
		ctx.g.P("\tplain := new(", plainName, ")")
		ctx.g.P("\tif err := plain.UnmarshalJX(d); err != nil {")
		ctx.g.P("\t\treturn err")
		ctx.g.P("\t}")
		ctx.g.P("\tpb, err := plain.IntoPbErr()")
		ctx.g.P("\tif err != nil {")
		ctx.g.P("\t\treturn err")
		ctx.g.P("\t}")
		ctx.g.P("\tif pb != nil {")
		ctx.g.P("\t\t*x = *pb")
		ctx.g.P("\t}")
		ctx.g.P("\treturn nil")
		ctx.g.P("}")
		ctx.g.P()
		return nil
	}

	// Protobuf message без Plain - используем protojson
	protoJSONMarshal := ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Marshal", GoImportPath: "google.golang.org/protobuf/encoding/protojson"})
	protoJSONUnmarshal := ctx.g.QualifiedGoIdent(protogen.GoIdent{GoName: "Unmarshal", GoImportPath: "google.golang.org/protobuf/encoding/protojson"})

	ctx.g.P("// MarshalJX сериализует protobuf сообщение через protojson в jx.Encoder")
	ctx.g.P("func (x *", msgName, ") MarshalJX(e *", jx.EncoderType, ") error {")
	ctx.g.P("\tif x == nil {")
	ctx.g.P("\t\te.Null()")
	ctx.g.P("\t\treturn nil")
	ctx.g.P("\t}")
	ctx.g.P("\traw, err := ", protoJSONMarshal, "(x)")
	ctx.g.P("\tif err != nil {")
	ctx.g.P("\t\treturn err")
	ctx.g.P("\t}")
	ctx.g.P("\te.Raw(raw)")
	ctx.g.P("\treturn nil")
	ctx.g.P("}")
	ctx.g.P()

	ctx.g.P("// UnmarshalJX десериализует protobuf сообщение через protojson из jx.Decoder")
	ctx.g.P("func (x *", msgName, ") UnmarshalJX(d *", jx.DecoderType, ") error {")
	ctx.g.P("\tif x == nil {")
	ctx.g.P("\t\treturn ", fmtErrorf, "(\"", msgName, ": UnmarshalJX on nil pointer\")")
	ctx.g.P("\t}")
	ctx.g.P("\tif d.Next() == ", jx.Null, " {")
	ctx.g.P("\t\treturn d.Null()")
	ctx.g.P("\t}")
	ctx.g.P("\traw, err := d.Raw()")
	ctx.g.P("\tif err != nil {")
	ctx.g.P("\t\treturn err")
	ctx.g.P("\t}")
	ctx.g.P("\treturn ", protoJSONUnmarshal, "(raw, x)")
	ctx.g.P("}")
	ctx.g.P()

	return nil
}

func (g *Generator) jxFieldInfo(ctx *renderContext, fw *FieldWrapper) (*jxFieldInfo, error) {
	if fw == nil || fw.Field == nil {
		return nil, nil
	}
	kind := fw.Field.GetKind()
	typeURL := fw.Field.GetTypeUrl()
	kind, typeURL = g.resolveAliasKind(ctx, kind, typeURL)

	goType, err := g.goTypeForField(ctx, fw)
	if err != nil {
		return nil, err
	}

	info := &jxFieldInfo{
		Name:        fw.Field.GetName(),
		GoName:      goFieldName(fw.Field.GetName()),
		Kind:        kind,
		TypeURL:     typeURL,
		Cardinality: fw.Field.GetCardinality(),
		GoType:      goType,
	}

	if fw.Source != nil && fw.Source.Desc.IsMap() {
		keyField := fw.Source.Message.Fields[0]
		valField := fw.Source.Message.Fields[1]
		keyKind, _ := fieldKindAndURL(keyField)
		valKind, valURL := fieldKindAndURL(valField)
		valKind, valURL = g.resolveAliasKind(ctx, valKind, valURL)

		keyType, _ := g.goTypeFromField(ctx, keyField, nil)
		valType, _ := g.goTypeFromField(ctx, valField, nil)

		info.IsMap = true
		info.MapKeyKind = keyKind
		info.MapKeyType = keyType
		info.MapValKind = valKind
		info.MapValType = valType
		info.MapValTypeURL = valURL
	}

	return info, nil
}

func (g *Generator) resolveAliasKind(ctx *renderContext, kind typepb.Field_Kind, typeURL string) (typepb.Field_Kind, string) {
	if kind != typepb.Field_TYPE_MESSAGE || typeURL == "" {
		return kind, typeURL
	}
	full := strings.TrimPrefix(typeURL, "type.googleapis.com/")
	if full == "" {
		return kind, typeURL
	}
	if alias := ctx.builder.aliases[full]; alias != nil {
		return alias.Kind, alias.TypeUrl
	}
	return kind, typeURL
}

func (g *Generator) isPlainMessage(ctx *renderContext, typeURL string) bool {
	full := strings.TrimPrefix(typeURL, "type.googleapis.com/")
	if full == "" {
		return false
	}
	if alias := ctx.builder.aliases[full]; alias != nil {
		return false
	}
	return ctx.builder.generatedMessages[full]
}

func (g *Generator) renderJXMarshalField(ctx *renderContext, info *jxFieldInfo, jx *jxRefs, jsonRefs, protoJSONRefs *jxJSONRefs, strconvRefs *jxStrconvRefs) {
	fieldExpr := "x." + info.GoName
	if info.IsMap {
		ctx.g.P("\tif len(", fieldExpr, ") > 0 {")
		ctx.g.P("\t\te.FieldStart(\"", info.Name, "\")")
		ctx.g.P("\t\te.ObjStart()")
		ctx.g.P("\t\tfor k, v := range ", fieldExpr, " {")
		keyExpr := g.jxMapKeyStringExpr(info.MapKeyKind, "k", strconvRefs)
		ctx.g.P("\t\t\te.FieldStart(", keyExpr, ")")
		if strings.HasPrefix(info.MapValType, "*") || info.MapValKind == typepb.Field_TYPE_BYTES {
			ctx.g.P("\t\t\tif v == nil {")
			ctx.g.P("\t\t\t\te.Null()")
			ctx.g.P("\t\t\t\tcontinue")
			ctx.g.P("\t\t\t}")
		}
		g.renderJXMarshalValue(ctx, info.MapValKind, info.MapValTypeURL, "v", info.MapValType, jx, jsonRefs, protoJSONRefs, "\t\t\t")
		ctx.g.P("\t\t}")
		ctx.g.P("\t\te.ObjEnd()")
		ctx.g.P("\t}")
		return
	}

	if info.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		ctx.g.P("\tif len(", fieldExpr, ") > 0 {")
		ctx.g.P("\t\te.FieldStart(\"", info.Name, "\")")
		ctx.g.P("\t\te.ArrStart()")
		ctx.g.P("\t\tfor _, v := range ", fieldExpr, " {")
		elemType := strings.TrimPrefix(info.GoType, "[]")
		if strings.HasPrefix(elemType, "*") {
			ctx.g.P("\t\t\tif v == nil {")
			ctx.g.P("\t\t\t\te.Null()")
			ctx.g.P("\t\t\t\tcontinue")
			ctx.g.P("\t\t\t}")
			g.renderJXMarshalValue(ctx, info.Kind, info.TypeURL, "v", elemType, jx, jsonRefs, protoJSONRefs, "\t\t\t")
		} else if info.Kind == typepb.Field_TYPE_BYTES {
			ctx.g.P("\t\t\tif v == nil {")
			ctx.g.P("\t\t\t\te.Null()")
			ctx.g.P("\t\t\t\tcontinue")
			ctx.g.P("\t\t\t}")
			g.renderJXMarshalValue(ctx, info.Kind, info.TypeURL, "v", elemType, jx, jsonRefs, protoJSONRefs, "\t\t\t")
		} else {
			g.renderJXMarshalValue(ctx, info.Kind, info.TypeURL, "v", elemType, jx, jsonRefs, protoJSONRefs, "\t\t\t")
		}
		ctx.g.P("\t\t}")
		ctx.g.P("\t\te.ArrEnd()")
		ctx.g.P("\t}")
		return
	}

	cond := g.jxOmitCondition(info)
	if cond != "" {
		ctx.g.P("\tif ", cond, " {")
	}
	indent := "\t\t"
	if cond == "" {
		indent = "\t"
	}
	ctx.g.P(indent, "e.FieldStart(\"", info.Name, "\")")

	valueExpr := fieldExpr
	valueType := info.GoType
	if strings.HasPrefix(valueType, "*") && info.Kind != typepb.Field_TYPE_MESSAGE {
		valueExpr = "*" + valueExpr
		valueType = strings.TrimPrefix(valueType, "*")
	}
	g.renderJXMarshalValue(ctx, info.Kind, info.TypeURL, valueExpr, valueType, jx, jsonRefs, protoJSONRefs, indent)

	if cond != "" {
		ctx.g.P("\t}")
	}
}

func (g *Generator) renderJXMarshalValue(ctx *renderContext, kind typepb.Field_Kind, typeURL, valueExpr, valueType string, jx *jxRefs, jsonRefs, protoJSONRefs *jxJSONRefs, indent string) {
	if kind == typepb.Field_TYPE_MESSAGE {
		if g.shouldUseProtoJSON(ctx, typeURL) {
			// Well-known types - используем protojson
			ctx.g.P(indent, "if ", valueExpr, " == nil {")
			ctx.g.P(indent, "\te.Null()")
			ctx.g.P(indent, "} else {")
			ctx.g.P(indent, "\traw, err := ", protoJSONRefs.Marshal, "(", valueExpr, ")")
			ctx.g.P(indent, "\tif err != nil {")
			ctx.g.P(indent, "\t\treturn err")
			ctx.g.P(indent, "\t}")
			ctx.g.P(indent, "\te.Raw(raw)")
			ctx.g.P(indent, "}")
			return
		}
		// Наши сгенерированные типы - прямой вызов MarshalJX
		ctx.g.P(indent, "if err := ", valueExpr, ".MarshalJX(e); err != nil {")
		ctx.g.P(indent, "\treturn err")
		ctx.g.P(indent, "}")
		return
	}

	method, baseType := jxMethodAndBaseType(kind)
	if method == "" {
		ctx.g.P(indent, "raw, err := ", jsonRefs.Marshal, "(", valueExpr, ")")
		ctx.g.P(indent, "if err != nil {")
		ctx.g.P(indent, "\treturn err")
		ctx.g.P(indent, "}")
		ctx.g.P(indent, "e.Raw(raw)")
		return
	}

	castExpr := fmt.Sprintf("%s(%s)", baseType, valueExpr)
	ctx.g.P(indent, "e.", method, "(", castExpr, ")")
	_ = valueType
}

func jxMethodAndBaseType(kind typepb.Field_Kind) (string, string) {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "StrEscape", "string"
	case typepb.Field_TYPE_BOOL:
		return "Bool", "bool"
	case typepb.Field_TYPE_FLOAT:
		return "Float32", "float32"
	case typepb.Field_TYPE_DOUBLE:
		return "Float64", "float64"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "Int64", "int64"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "UInt64", "uint64"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "Int32", "int32"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "UInt32", "uint32"
	case typepb.Field_TYPE_BYTES:
		return "Base64", "[]byte"
	case typepb.Field_TYPE_ENUM:
		return "Int32", "int32"
	default:
		return "", ""
	}
}

func jxNeedsJSONFallback(kind typepb.Field_Kind) bool {
	method, _ := jxMethodAndBaseType(kind)
	return method == ""
}

func (g *Generator) renderJXUnmarshalField(ctx *renderContext, info *jxFieldInfo, jxDecoderType string, jsonRefs, protoJSONRefs *jxJSONRefs, strconvRefs *jxStrconvRefs, fmtErrorf, jxNull string) {
	ctx.g.P("\t\tcase \"", info.Name, "\":")
	ctx.g.P("\t\t\t{")
	if info.IsMap {
		g.renderJXUnmarshalMap(ctx, info, jxDecoderType, jsonRefs, protoJSONRefs, strconvRefs, fmtErrorf, jxNull)
		ctx.g.P("\t\t\t}")
		return
	}
	if info.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		g.renderJXUnmarshalRepeated(ctx, info, jxDecoderType, jsonRefs, protoJSONRefs, jxNull)
		ctx.g.P("\t\t\t}")
		return
	}
	g.renderJXUnmarshalScalar(ctx, info, jxDecoderType, jsonRefs, protoJSONRefs, jxNull)
	ctx.g.P("\t\t\t}")
}

func (g *Generator) renderJXUnmarshalScalar(ctx *renderContext, info *jxFieldInfo, jxDecoderType string, jsonRefs, protoJSONRefs *jxJSONRefs, jxNull string) {
	fieldExpr := "x." + info.GoName
	goType := info.GoType

	if strings.HasPrefix(goType, "*") {
		baseType := strings.TrimPrefix(goType, "*")
		ctx.g.P("\t\t\t\tif d.Next() == ", jxNull, " {")
		ctx.g.P("\t\t\t\t\t", fieldExpr, " = nil")
		ctx.g.P("\t\t\t\t\treturn d.Null()")
		ctx.g.P("\t\t\t\t}")
		if info.Kind == typepb.Field_TYPE_MESSAGE {
			ctx.g.P("\t\t\t\tval := new(", baseType, ")")
			if g.shouldUseProtoJSON(ctx, info.TypeURL) {
				ctx.g.P("\t\t\t\traw, err := d.Raw()")
				ctx.g.P("\t\t\t\tif err != nil {")
				ctx.g.P("\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t}")
				ctx.g.P("\t\t\t\tif err := ", protoJSONRefs.Unmarshal, "(raw, val); err != nil {")
				ctx.g.P("\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t}")
			} else {
				// Прямой вызов UnmarshalJX
				ctx.g.P("\t\t\t\tif err := val.UnmarshalJX(d); err != nil {")
				ctx.g.P("\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t}")
			}
			ctx.g.P("\t\t\t\t", fieldExpr, " = val")
			ctx.g.P("\t\t\t\treturn nil")
			return
		}

		method, _ := jxDecodeMethodAndBaseType(info.Kind)
		ctx.g.P("\t\t\t\tval, err := d.", method, "()")
		ctx.g.P("\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\treturn err")
		ctx.g.P("\t\t\t\t}")
		ctx.g.P("\t\t\t\ttmp := ", baseType, "(val)")
		ctx.g.P("\t\t\t\t", fieldExpr, " = &tmp")
		ctx.g.P("\t\t\t\treturn nil")
		return
	}

	if info.Kind == typepb.Field_TYPE_MESSAGE {
		if g.shouldUseProtoJSON(ctx, info.TypeURL) {
			ctx.g.P("\t\t\t\traw, err := d.Raw()")
			ctx.g.P("\t\t\t\tif err != nil {")
			ctx.g.P("\t\t\t\t\treturn err")
			ctx.g.P("\t\t\t\t}")
			ctx.g.P("\t\t\t\treturn ", protoJSONRefs.Unmarshal, "(raw, &", fieldExpr, ")")
		} else {
			// Прямой вызов UnmarshalJX
			ctx.g.P("\t\t\t\treturn (&", fieldExpr, ").UnmarshalJX(d)")
		}
		return
	}

	ctx.g.P("\t\t\t\tif d.Next() == ", jxNull, " {")
	ctx.g.P("\t\t\t\t\t", fieldExpr, " = ", goType, "(", zeroValueForKind(info.Kind, false), ")")
	ctx.g.P("\t\t\t\t\treturn d.Null()")
	ctx.g.P("\t\t\t\t}")
	method, _ := jxDecodeMethodAndBaseType(info.Kind)
	ctx.g.P("\t\t\t\tval, err := d.", method, "()")
	ctx.g.P("\t\t\t\tif err != nil {")
	ctx.g.P("\t\t\t\t\treturn err")
	ctx.g.P("\t\t\t\t}")
	ctx.g.P("\t\t\t\t", fieldExpr, " = ", goType, "(val)")
	ctx.g.P("\t\t\t\treturn nil")
}

func (g *Generator) renderJXUnmarshalRepeated(ctx *renderContext, info *jxFieldInfo, jxDecoderType string, jsonRefs, protoJSONRefs *jxJSONRefs, jxNull string) {
	fieldExpr := "x." + info.GoName
	elemType := strings.TrimPrefix(info.GoType, "[]")
	elemIsPtr := strings.HasPrefix(elemType, "*")
	baseElem := strings.TrimPrefix(elemType, "*")

	ctx.g.P("\t\t\t\tif d.Next() == ", jxNull, " {")
	ctx.g.P("\t\t\t\t\t", fieldExpr, " = nil")
	ctx.g.P("\t\t\t\t\treturn d.Null()")
	ctx.g.P("\t\t\t\t}")
	ctx.g.P("\t\t\t\t", fieldExpr, " = ", fieldExpr, "[:0]") // Переиспользуем slice
	ctx.g.P("\t\t\t\treturn d.Arr(func(d *", jxDecoderType, ") error {")
	if elemIsPtr {
		ctx.g.P("\t\t\t\t\tif d.Next() == ", jxNull, " {")
		ctx.g.P("\t\t\t\t\t\t_ = d.Null()")
		ctx.g.P("\t\t\t\t\t\t", fieldExpr, " = append(", fieldExpr, ", nil)")
		ctx.g.P("\t\t\t\t\t\treturn nil")
		ctx.g.P("\t\t\t\t\t}")
		if info.Kind == typepb.Field_TYPE_MESSAGE {
			ctx.g.P("\t\t\t\t\tval := new(", baseElem, ")")
			if g.shouldUseProtoJSON(ctx, info.TypeURL) {
				ctx.g.P("\t\t\t\t\traw, err := d.Raw()")
				ctx.g.P("\t\t\t\t\tif err != nil {")
				ctx.g.P("\t\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t\t}")
				ctx.g.P("\t\t\t\t\tif err := ", protoJSONRefs.Unmarshal, "(raw, val); err != nil {")
				ctx.g.P("\t\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t\t}")
			} else {
				// Прямой вызов UnmarshalJX
				ctx.g.P("\t\t\t\t\tif err := val.UnmarshalJX(d); err != nil {")
				ctx.g.P("\t\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t\t}")
			}
			ctx.g.P("\t\t\t\t\t", fieldExpr, " = append(", fieldExpr, ", val)")
			ctx.g.P("\t\t\t\t\treturn nil")
			ctx.g.P("\t\t\t\t})")
			return
		}
		method, _ := jxDecodeMethodAndBaseType(info.Kind)
		ctx.g.P("\t\t\t\t\tval, err := d.", method, "()")
		ctx.g.P("\t\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\t\treturn err")
		ctx.g.P("\t\t\t\t\t}")
		ctx.g.P("\t\t\t\t\ttmp := ", baseElem, "(val)")
		ctx.g.P("\t\t\t\t\t", fieldExpr, " = append(", fieldExpr, ", &tmp)")
		ctx.g.P("\t\t\t\t\treturn nil")
		ctx.g.P("\t\t\t\t})")
		return
	}

	ctx.g.P("\t\t\t\t\tif d.Next() == ", jxNull, " {")
	ctx.g.P("\t\t\t\t\t\t_ = d.Null()")
	ctx.g.P("\t\t\t\t\t\t", fieldExpr, " = append(", fieldExpr, ", ", elemType, "(", zeroValueForKind(info.Kind, false), "))")
	ctx.g.P("\t\t\t\t\t\treturn nil")
	ctx.g.P("\t\t\t\t\t}")
	if info.Kind == typepb.Field_TYPE_MESSAGE {
		ctx.g.P("\t\t\t\t\tvar val ", elemType)
		if g.shouldUseProtoJSON(ctx, info.TypeURL) {
			ctx.g.P("\t\t\t\t\traw, err := d.Raw()")
			ctx.g.P("\t\t\t\t\tif err != nil {")
			ctx.g.P("\t\t\t\t\t\treturn err")
			ctx.g.P("\t\t\t\t\t}")
			ctx.g.P("\t\t\t\t\tif err := ", protoJSONRefs.Unmarshal, "(raw, &val); err != nil {")
			ctx.g.P("\t\t\t\t\t\treturn err")
			ctx.g.P("\t\t\t\t\t}")
		} else {
			// Прямой вызов UnmarshalJX
			ctx.g.P("\t\t\t\t\tif err := (&val).UnmarshalJX(d); err != nil {")
			ctx.g.P("\t\t\t\t\t\treturn err")
			ctx.g.P("\t\t\t\t\t}")
		}
		ctx.g.P("\t\t\t\t\t", fieldExpr, " = append(", fieldExpr, ", val)")
		ctx.g.P("\t\t\t\t\treturn nil")
		ctx.g.P("\t\t\t\t})")
		return
	}
	method, _ := jxDecodeMethodAndBaseType(info.Kind)
	ctx.g.P("\t\t\t\t\tval, err := d.", method, "()")
	ctx.g.P("\t\t\t\t\tif err != nil {")
	ctx.g.P("\t\t\t\t\t\treturn err")
	ctx.g.P("\t\t\t\t\t}")
	ctx.g.P("\t\t\t\t\t", fieldExpr, " = append(", fieldExpr, ", ", elemType, "(val))")
	ctx.g.P("\t\t\t\t\treturn nil")
	ctx.g.P("\t\t\t\t})")
}

func (g *Generator) renderJXUnmarshalMap(ctx *renderContext, info *jxFieldInfo, jxDecoderType string, jsonRefs, protoJSONRefs *jxJSONRefs, strconvRefs *jxStrconvRefs, fmtErrorf, jxNull string) {
	fieldExpr := "x." + info.GoName
	valType := info.MapValType
	valIsPtr := strings.HasPrefix(valType, "*")
	valBase := strings.TrimPrefix(valType, "*")

	ctx.g.P("\t\t\t\tif d.Next() == ", jxNull, " {")
	ctx.g.P("\t\t\t\t\t", fieldExpr, " = nil")
	ctx.g.P("\t\t\t\t\treturn d.Null()")
	ctx.g.P("\t\t\t\t}")
	ctx.g.P("\t\t\t\t", fieldExpr, " = make(map[", info.MapKeyType, "]", info.MapValType, ")")
	ctx.g.P("\t\t\t\treturn d.Obj(func(d *", jxDecoderType, ", key string) error {")
	ctx.g.P("\t\t\t\t\tvar mapKey ", info.MapKeyType)
	switch info.MapKeyKind {
	case typepb.Field_TYPE_STRING:
		ctx.g.P("\t\t\t\t\tmapKey = ", info.MapKeyType, "(key)")
	case typepb.Field_TYPE_BOOL:
		ctx.g.P("\t\t\t\t\tparsed, err := ", strconvRefs.ParseBool, "(key)")
		ctx.g.P("\t\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\t\treturn ", fmtErrorf, "(\"", info.Name, ": %w\", err)")
		ctx.g.P("\t\t\t\t\t}")
		ctx.g.P("\t\t\t\t\tmapKey = ", info.MapKeyType, "(parsed)")
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		ctx.g.P("\t\t\t\t\tparsed, err := ", strconvRefs.ParseInt, "(key, 10, 32)")
		ctx.g.P("\t\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\t\treturn ", fmtErrorf, "(\"", info.Name, ": %w\", err)")
		ctx.g.P("\t\t\t\t\t}")
		ctx.g.P("\t\t\t\t\tmapKey = ", info.MapKeyType, "(parsed)")
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		ctx.g.P("\t\t\t\t\tparsed, err := ", strconvRefs.ParseUint, "(key, 10, 32)")
		ctx.g.P("\t\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\t\treturn ", fmtErrorf, "(\"", info.Name, ": %w\", err)")
		ctx.g.P("\t\t\t\t\t}")
		ctx.g.P("\t\t\t\t\tmapKey = ", info.MapKeyType, "(parsed)")
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		ctx.g.P("\t\t\t\t\tparsed, err := ", strconvRefs.ParseInt, "(key, 10, 64)")
		ctx.g.P("\t\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\t\treturn ", fmtErrorf, "(\"", info.Name, ": %w\", err)")
		ctx.g.P("\t\t\t\t\t}")
		ctx.g.P("\t\t\t\t\tmapKey = ", info.MapKeyType, "(parsed)")
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		ctx.g.P("\t\t\t\t\tparsed, err := ", strconvRefs.ParseUint, "(key, 10, 64)")
		ctx.g.P("\t\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\t\treturn ", fmtErrorf, "(\"", info.Name, ": %w\", err)")
		ctx.g.P("\t\t\t\t\t}")
		ctx.g.P("\t\t\t\t\tmapKey = ", info.MapKeyType, "(parsed)")
	default:
		ctx.g.P("\t\t\t\t\treturn ", fmtErrorf, "(\"", info.Name, ": unsupported map key type\")")
	}

	if valIsPtr {
		ctx.g.P("\t\t\t\t\tif d.Next() == ", jxNull, " {")
		ctx.g.P("\t\t\t\t\t\t_ = d.Null()")
		ctx.g.P("\t\t\t\t\t\t", fieldExpr, "[mapKey] = nil")
		ctx.g.P("\t\t\t\t\t\treturn nil")
		ctx.g.P("\t\t\t\t\t}")
		if info.MapValKind == typepb.Field_TYPE_MESSAGE {
			ctx.g.P("\t\t\t\t\tval := new(", valBase, ")")
			if g.shouldUseProtoJSON(ctx, info.MapValTypeURL) {
				ctx.g.P("\t\t\t\t\traw, err := d.Raw()")
				ctx.g.P("\t\t\t\t\tif err != nil {")
				ctx.g.P("\t\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t\t}")
				ctx.g.P("\t\t\t\t\tif err := ", protoJSONRefs.Unmarshal, "(raw, val); err != nil {")
				ctx.g.P("\t\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t\t}")
			} else {
				// Прямой вызов UnmarshalJX
				ctx.g.P("\t\t\t\t\tif err := val.UnmarshalJX(d); err != nil {")
				ctx.g.P("\t\t\t\t\t\treturn err")
				ctx.g.P("\t\t\t\t\t}")
			}
			ctx.g.P("\t\t\t\t\t", fieldExpr, "[mapKey] = val")
			ctx.g.P("\t\t\t\t\treturn nil")
			ctx.g.P("\t\t\t\t})")
			return
		}
		method, _ := jxDecodeMethodAndBaseType(info.MapValKind)
		ctx.g.P("\t\t\t\t\tval, err := d.", method, "()")
		ctx.g.P("\t\t\t\t\tif err != nil {")
		ctx.g.P("\t\t\t\t\t\treturn err")
		ctx.g.P("\t\t\t\t\t}")
		ctx.g.P("\t\t\t\t\ttmp := ", valBase, "(val)")
		ctx.g.P("\t\t\t\t\t", fieldExpr, "[mapKey] = &tmp")
		ctx.g.P("\t\t\t\t\treturn nil")
		ctx.g.P("\t\t\t\t})")
		return
	}

	ctx.g.P("\t\t\t\t\tif d.Next() == ", jxNull, " {")
	ctx.g.P("\t\t\t\t\t\t_ = d.Null()")
	ctx.g.P("\t\t\t\t\t\t", fieldExpr, "[mapKey] = ", valType, "(", zeroValueForKind(info.MapValKind, false), ")")
	ctx.g.P("\t\t\t\t\t\treturn nil")
	ctx.g.P("\t\t\t\t\t}")
	if info.MapValKind == typepb.Field_TYPE_MESSAGE {
		ctx.g.P("\t\t\t\t\tvar val ", valType)
		if g.shouldUseProtoJSON(ctx, info.MapValTypeURL) {
			ctx.g.P("\t\t\t\t\traw, err := d.Raw()")
			ctx.g.P("\t\t\t\t\tif err != nil {")
			ctx.g.P("\t\t\t\t\t\treturn err")
			ctx.g.P("\t\t\t\t\t}")
			ctx.g.P("\t\t\t\t\tif err := ", protoJSONRefs.Unmarshal, "(raw, &val); err != nil {")
			ctx.g.P("\t\t\t\t\t\treturn err")
			ctx.g.P("\t\t\t\t\t}")
		} else {
			// Прямой вызов UnmarshalJX
			ctx.g.P("\t\t\t\t\tif err := (&val).UnmarshalJX(d); err != nil {")
			ctx.g.P("\t\t\t\t\t\treturn err")
			ctx.g.P("\t\t\t\t\t}")
		}
		ctx.g.P("\t\t\t\t\t", fieldExpr, "[mapKey] = val")
		ctx.g.P("\t\t\t\t\treturn nil")
		ctx.g.P("\t\t\t\t})")
		return
	}
	method, _ := jxDecodeMethodAndBaseType(info.MapValKind)
	ctx.g.P("\t\t\t\t\tval, err := d.", method, "()")
	ctx.g.P("\t\t\t\t\tif err != nil {")
	ctx.g.P("\t\t\t\t\t\treturn err")
	ctx.g.P("\t\t\t\t\t}")
	ctx.g.P("\t\t\t\t\t", fieldExpr, "[mapKey] = ", valType, "(val)")
	ctx.g.P("\t\t\t\t\treturn nil")
	ctx.g.P("\t\t\t\t})")
}

func jxDecodeMethodAndBaseType(kind typepb.Field_Kind) (string, string) {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "Str", "string"
	case typepb.Field_TYPE_BOOL:
		return "Bool", "bool"
	case typepb.Field_TYPE_FLOAT:
		return "Float32", "float32"
	case typepb.Field_TYPE_DOUBLE:
		return "Float64", "float64"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "Int64", "int64"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "UInt64", "uint64"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "Int32", "int32"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "UInt32", "uint32"
	case typepb.Field_TYPE_BYTES:
		return "Base64", "[]byte"
	case typepb.Field_TYPE_ENUM:
		return "Int32", "int32"
	default:
		return "", ""
	}
}

func (g *Generator) jxMapKeyStringExpr(kind typepb.Field_Kind, keyExpr string, refs *jxStrconvRefs) string {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "string(" + keyExpr + ")"
	case typepb.Field_TYPE_BOOL:
		return refs.FormatBool + "(bool(" + keyExpr + "))"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return refs.FormatInt + "(int64(" + keyExpr + "), 10)"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return refs.FormatUint + "(uint64(" + keyExpr + "), 10)"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return refs.FormatInt + "(int64(" + keyExpr + "), 10)"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return refs.FormatUint + "(uint64(" + keyExpr + "), 10)"
	default:
		return "string(" + keyExpr + ")"
	}
}

func (g *Generator) jxOmitCondition(info *jxFieldInfo) string {
	fieldExpr := "x." + info.GoName
	if info.IsMap || info.Cardinality == typepb.Field_CARDINALITY_REPEATED {
		return "len(" + fieldExpr + ") > 0"
	}
	if strings.HasPrefix(info.GoType, "*") {
		return fieldExpr + " != nil"
	}
	switch info.Kind {
	case typepb.Field_TYPE_STRING:
		return fieldExpr + " != \"\""
	case typepb.Field_TYPE_BOOL:
		return fieldExpr
	case typepb.Field_TYPE_BYTES:
		return "len(" + fieldExpr + ") > 0"
	default:
		return fieldExpr + " != 0"
	}
}

func (g *Generator) shouldUseProtoJSON(ctx *renderContext, typeURL string) bool {
	full := strings.TrimPrefix(typeURL, "type.googleapis.com/")
	if full == "" {
		return true
	}
	if strings.HasPrefix(full, "google.protobuf.") {
		return true
	}
	_, ok := ctx.builder.messagesByFullName[full]
	return !ok
}
