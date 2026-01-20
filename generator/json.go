package generator

import (
	"strconv"

	"github.com/yaroher/protoc-gen-go-plain/ir"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func generateJSONMethods(g *protogen.GeneratedFile, plainMsg *protogen.Message, msgIR *ir.MessageIR) {
	if g == nil || plainMsg == nil || msgIR == nil {
		return
	}
	fieldPlans := map[string]*ir.FieldPlan{}
	for _, fp := range msgIR.FieldPlan {
		if fp == nil {
			continue
		}
		fieldPlans[fp.NewField.Name] = fp
	}
	generateMarshalJSON(g, plainMsg, msgIR, fieldPlans)
	generateUnmarshalJSON(g, plainMsg, msgIR, fieldPlans)
}

func generateMarshalJSON(g *protogen.GeneratedFile, plainMsg *protogen.Message, msgIR *ir.MessageIR, fieldPlans map[string]*ir.FieldPlan) {
	jxEnc := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Encoder", GoImportPath: "github.com/go-faster/jx"})
	protojsonMarshal := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Marshal", GoImportPath: "google.golang.org/protobuf/encoding/protojson"})
	jsonMarshal := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Marshal", GoImportPath: "encoding/json"})

	g.P("func (m *", plainMsg.GoIdent.GoName, ") MarshalJSON() ([]byte, error) {")
	g.P("\tif m == nil { return []byte(\"null\"), nil }")
	g.P("\t_ = ", protojsonMarshal)
	g.P("\t_ = ", jsonMarshal)
	g.P("\tvar e ", jxEnc)
	g.P("\te.ObjStart()")
	for _, f := range plainMsg.Fields {
		jsonName := string(f.Desc.JSONName())
		g.P("\te.FieldStart(", strconv.Quote(jsonName), ")")
		fp := fieldPlans[string(f.Desc.Name())]
		emitJSONEncodeField(g, f, fp, protojsonMarshal, jsonMarshal)
	}
	g.P("\te.ObjEnd()")
	g.P("\treturn e.Bytes(), nil")
	g.P("}")
	g.P("")
}

func generateUnmarshalJSON(g *protogen.GeneratedFile, plainMsg *protogen.Message, msgIR *ir.MessageIR, fieldPlans map[string]*ir.FieldPlan) {
	jxDecode := g.QualifiedGoIdent(protogen.GoIdent{GoName: "DecodeBytes", GoImportPath: "github.com/go-faster/jx"})
	jxDecoder := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Decoder", GoImportPath: "github.com/go-faster/jx"})
	protojsonUnmarshal := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Unmarshal", GoImportPath: "google.golang.org/protobuf/encoding/protojson"})
	jsonUnmarshal := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Unmarshal", GoImportPath: "encoding/json"})

	g.P("func (m *", plainMsg.GoIdent.GoName, ") UnmarshalJSON(data []byte) error {")
	g.P("\tif m == nil { return nil }")
	g.P("\t_ = ", protojsonUnmarshal)
	g.P("\t_ = ", jsonUnmarshal)
	g.P("\td := ", jxDecode, "(data)")
	g.P("\treturn d.Obj(func(d *", jxDecoder, ", key string) error {")
	g.P("\t\tswitch key {")
	for _, f := range plainMsg.Fields {
		jsonName := string(f.Desc.JSONName())
		g.P("\t\tcase ", strconv.Quote(jsonName), ":")
		fp := fieldPlans[string(f.Desc.Name())]
		emitJSONDecodeField(g, f, fp, protojsonUnmarshal, jsonUnmarshal)
	}
	g.P("\t\tdefault:")
	g.P("\t\t\treturn d.Skip()")
	g.P("\t\t}")
	g.P("\t})")
	g.P("}")
	g.P("")
}

func emitJSONEncodeField(g *protogen.GeneratedFile, f *protogen.Field, fp *ir.FieldPlan, protojsonMarshal, jsonMarshal string) {
	if fp != nil && hasOverride(fp) {
		overrideName := getOverrideName(fp)
		if overrideName == "EnumDiscriminator" {
			g.P("\te.Str(string(m.", f.GoName, "))")
			return
		}
		if overrideName == "any" {
			g.P("\tif m.", f.GoName, " == nil { e.Null() } else {")
			g.P("\t\tif b, err := ", jsonMarshal, "(m.", f.GoName, "); err != nil { return nil, err } else { e.Raw(b) }")
			g.P("\t}")
			return
		}
		g.P("\t\tif b, err := ", jsonMarshal, "(m.", f.GoName, "); err != nil { return nil, err } else { e.Raw(b) }")
		return
	}
	switch {
	case f.Desc.IsMap():
		emitJSONEncodeMap(g, f, protojsonMarshal, jsonMarshal)
		return
	case f.Desc.Cardinality() == protoreflect.Repeated:
		emitJSONEncodeSlice(g, f, protojsonMarshal, jsonMarshal)
		return
	}

	switch f.Desc.Kind() {
	case protoreflect.BoolKind:
		g.P("\te.Bool(m.", f.GoName, ")")
	case protoreflect.StringKind:
		g.P("\te.Str(m.", f.GoName, ")")
	case protoreflect.BytesKind:
		g.P("\tif m.", f.GoName, " == nil { e.Null() } else { e.Base64(m.", f.GoName, ") }")
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		g.P("\te.Int32(m.", f.GoName, ")")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		g.P("\te.Int64(m.", f.GoName, ")")
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		g.P("\te.UInt32(m.", f.GoName, ")")
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		g.P("\te.UInt64(m.", f.GoName, ")")
	case protoreflect.FloatKind:
		g.P("\te.Float32(m.", f.GoName, ")")
	case protoreflect.DoubleKind:
		g.P("\te.Float64(m.", f.GoName, ")")
	case protoreflect.EnumKind:
		g.P("\te.Int32(int32(m.", f.GoName, "))")
	case protoreflect.MessageKind:
		g.P("\tif m.", f.GoName, " == nil { e.Null() } else {")
		g.P("\t\tif b, err := ", protojsonMarshal, "(m.", f.GoName, "); err != nil { return nil, err } else { e.Raw(b) }")
		g.P("\t}")
	default:
		g.P("\tif m.", f.GoName, " == nil { e.Null() } else {")
		g.P("\t\tif b, err := ", jsonMarshal, "(m.", f.GoName, "); err != nil { return nil, err } else { e.Raw(b) }")
		g.P("\t}")
	}
}

func emitJSONEncodeSlice(g *protogen.GeneratedFile, f *protogen.Field, protojsonMarshal, jsonMarshal string) {
	g.P("\tif m.", f.GoName, " == nil { e.Null(); } else {")
	g.P("\t\te.ArrStart()")
	elemKind := f.Desc.Kind()
	if elemKind == protoreflect.MessageKind && f.Message != nil {
		g.P("\t\tfor _, v := range m.", f.GoName, " {")
		g.P("\t\t\tif v == nil { e.Null(); continue }")
		g.P("\t\t\tb, err := ", protojsonMarshal, "(v)")
		g.P("\t\t\tif err != nil { return nil, err }")
		g.P("\t\t\te.Raw(b)")
		g.P("\t\t}")
		g.P("\t\te.ArrEnd()")
		g.P("\t}")
		return
	}
	g.P("\t\tfor _, v := range m.", f.GoName, " {")
	switch elemKind {
	case protoreflect.BoolKind:
		g.P("\t\t\te.Bool(v)")
	case protoreflect.StringKind:
		g.P("\t\t\te.Str(v)")
	case protoreflect.BytesKind:
		g.P("\t\t\te.Base64(v)")
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		g.P("\t\t\te.Int32(v)")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		g.P("\t\t\te.Int64(v)")
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		g.P("\t\t\te.UInt32(v)")
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		g.P("\t\t\te.UInt64(v)")
	case protoreflect.FloatKind:
		g.P("\t\t\te.Float32(v)")
	case protoreflect.DoubleKind:
		g.P("\t\t\te.Float64(v)")
	case protoreflect.EnumKind:
		g.P("\t\t\te.Int32(int32(v))")
	default:
		g.P("\t\t\tb, err := ", jsonMarshal, "(v)")
		g.P("\t\t\tif err != nil { return nil, err }")
		g.P("\t\t\te.Raw(b)")
	}
	g.P("\t\t}")
	g.P("\t\te.ArrEnd()")
	g.P("\t}")
}

func emitJSONEncodeMap(g *protogen.GeneratedFile, f *protogen.Field, protojsonMarshal, jsonMarshal string) {
	keyField := f.Message.Fields[0]
	valField := f.Message.Fields[1]
	g.P("\tif m.", f.GoName, " == nil { e.Null(); } else {")
	g.P("\t\te.ObjStart()")
	g.P("\t\tfor k, v := range m.", f.GoName, " {")
	g.P("\t\t\te.FieldStart(", mapKeyToStringExpr(keyField, g), ")")
	if valField.Desc.Kind() == protoreflect.MessageKind && valField.Message != nil {
		g.P("\t\t\tif v == nil { e.Null(); } else {")
		g.P("\t\t\t\tb, err := ", protojsonMarshal, "(v)")
		g.P("\t\t\t\tif err != nil { return nil, err }")
		g.P("\t\t\t\te.Raw(b)")
		g.P("\t\t\t}")
	} else {
		switch valField.Desc.Kind() {
		case protoreflect.BoolKind:
			g.P("\t\t\te.Bool(v)")
		case protoreflect.StringKind:
			g.P("\t\t\te.Str(v)")
		case protoreflect.BytesKind:
			g.P("\t\t\te.Base64(v)")
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
			g.P("\t\t\te.Int32(v)")
		case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			g.P("\t\t\te.Int64(v)")
		case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
			g.P("\t\t\te.UInt32(v)")
		case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
			g.P("\t\t\te.UInt64(v)")
		case protoreflect.FloatKind:
			g.P("\t\t\te.Float32(v)")
		case protoreflect.DoubleKind:
			g.P("\t\t\te.Float64(v)")
		case protoreflect.EnumKind:
			g.P("\t\t\te.Int32(int32(v))")
		default:
			g.P("\t\t\tb, err := ", jsonMarshal, "(v)")
			g.P("\t\t\tif err != nil { return nil, err }")
			g.P("\t\t\te.Raw(b)")
		}
	}
	g.P("\t\t}")
	g.P("\t\te.ObjEnd()")
	g.P("\t}")
}

func mapKeyToStringExpr(f *protogen.Field, g *protogen.GeneratedFile) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "k"
	case protoreflect.BoolKind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatBool", GoImportPath: "strconv"})
		return "strconv.FormatBool(k)"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatInt", GoImportPath: "strconv"})
		return "strconv.FormatInt(int64(k), 10)"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatInt", GoImportPath: "strconv"})
		return "strconv.FormatInt(k, 10)"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatUint", GoImportPath: "strconv"})
		return "strconv.FormatUint(uint64(k), 10)"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "FormatUint", GoImportPath: "strconv"})
		return "strconv.FormatUint(k, 10)"
	default:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "Sprint", GoImportPath: "fmt"})
		return "fmt.Sprint(k)"
	}
}

func emitJSONDecodeField(g *protogen.GeneratedFile, f *protogen.Field, fp *ir.FieldPlan, protojsonUnmarshal, jsonUnmarshal string) {
	if fp != nil && hasOverride(fp) {
		overrideName := getOverrideName(fp)
		if overrideName == "EnumDiscriminator" {
			discType := getOverrideGoType(g, f, fp)
			g.P("\t\t\tv, err := d.Str()")
			g.P("\t\t\tif err != nil { return err }")
			g.P("\t\t\tm.", f.GoName, " = ", discType, "(v)")
			g.P("\t\t\treturn nil")
			return
		}
		if overrideName == "any" {
			g.P("\t\t\traw, err := d.Raw()")
			g.P("\t\t\tif err != nil { return err }")
			g.P("\t\t\tif string(raw) == \"null\" { m.", f.GoName, " = nil; return nil }")
			g.P("\t\t\tvar v any")
			g.P("\t\t\tif err := ", jsonUnmarshal, "(raw, &v); err != nil { return err }")
			g.P("\t\t\tm.", f.GoName, " = v")
			g.P("\t\t\treturn nil")
			return
		}
		g.P("\t\t\traw, err := d.Raw()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\treturn ", jsonUnmarshal, "(raw, &m.", f.GoName, ")")
		return
	}
	switch {
	case f.Desc.IsMap():
		emitJSONDecodeMap(g, f, protojsonUnmarshal, jsonUnmarshal)
		return
	case f.Desc.Cardinality() == protoreflect.Repeated:
		emitJSONDecodeSlice(g, f, protojsonUnmarshal, jsonUnmarshal)
		return
	}

	switch f.Desc.Kind() {
	case protoreflect.BoolKind:
		g.P("\t\t\tv, err := d.Bool()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.StringKind:
		g.P("\t\t\tv, err := d.Str()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.BytesKind:
		g.P("\t\t\tv, err := d.Base64()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		g.P("\t\t\tv, err := d.Int32()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		g.P("\t\t\tv, err := d.Int64()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		g.P("\t\t\tv, err := d.UInt32()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		g.P("\t\t\tv, err := d.UInt64()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.FloatKind:
		g.P("\t\t\tv, err := d.Float32()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.DoubleKind:
		g.P("\t\t\tv, err := d.Float64()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	case protoreflect.EnumKind:
		g.P("\t\t\tv, err := d.Int32()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = ", f.Enum.GoIdent.GoName, "(v)")
	case protoreflect.MessageKind:
		msgType := g.QualifiedGoIdent(f.Message.GoIdent)
		g.P("\t\t\traw, err := d.Raw()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tif string(raw) == \"null\" { m.", f.GoName, " = nil; return nil }")
		g.P("\t\t\tv := &", msgType, "{}")
		g.P("\t\t\tif err := ", protojsonUnmarshal, "(raw, v); err != nil { return err }")
		g.P("\t\t\tm.", f.GoName, " = v")
	default:
		g.P("\t\t\traw, err := d.Raw()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\treturn ", jsonUnmarshal, "(raw, &m.", f.GoName, ")")
	}
	g.P("\t\t\treturn nil")
}

func emitJSONDecodeSlice(g *protogen.GeneratedFile, f *protogen.Field, protojsonUnmarshal, jsonUnmarshal string) {
	elemKind := f.Desc.Kind()
	if elemKind == protoreflect.MessageKind && f.Message != nil {
		msgType := g.QualifiedGoIdent(f.Message.GoIdent)
		g.P("\t\t\traw, err := d.Raw()")
		g.P("\t\t\tif err != nil { return err }")
		g.P("\t\t\tif string(raw) == \"null\" { m.", f.GoName, " = nil; return nil }")
		g.P("\t\t\tvar items []json.RawMessage")
		g.P("\t\t\tif err := ", jsonUnmarshal, "(raw, &items); err != nil { return err }")
		g.P("\t\t\tout := make([]*", msgType, ", 0, len(items))")
		g.P("\t\t\tfor _, b := range items {")
		g.P("\t\t\t\tif string(b) == \"null\" { out = append(out, nil); continue }")
		g.P("\t\t\t\tv := &", msgType, "{}")
		g.P("\t\t\t\tif err := ", protojsonUnmarshal, "(b, v); err != nil { return err }")
		g.P("\t\t\t\tout = append(out, v)")
		g.P("\t\t\t}")
		g.P("\t\t\tm.", f.GoName, " = out")
		g.P("\t\t\treturn nil")
		return
	}
	g.P("\t\t\traw, err := d.Raw()")
	g.P("\t\t\tif err != nil { return err }")
	g.P("\t\t\treturn ", jsonUnmarshal, "(raw, &m.", f.GoName, ")")
}

func emitJSONDecodeMap(g *protogen.GeneratedFile, f *protogen.Field, protojsonUnmarshal, jsonUnmarshal string) {
	keyField := f.Message.Fields[0]
	valField := f.Message.Fields[1]
	g.P("\t\t\traw, err := d.Raw()")
	g.P("\t\t\tif err != nil { return err }")
	g.P("\t\t\tif string(raw) == \"null\" { m.", f.GoName, " = nil; return nil }")
	if valField.Desc.Kind() == protoreflect.MessageKind && valField.Message != nil {
		msgType := g.QualifiedGoIdent(valField.Message.GoIdent)
		g.P("\t\t\tvar items map[string]json.RawMessage")
		g.P("\t\t\tif err := ", jsonUnmarshal, "(raw, &items); err != nil { return err }")
		g.P("\t\t\tout := make(map[", mapKeyGoType(keyField), "]*", msgType, ", len(items))")
		g.P("\t\t\tfor k, b := range items {")
		g.P("\t\t\t\tkey, err := ", mapKeyParseExpr(keyField, g), "(k)")
		g.P("\t\t\t\tif err != nil { return err }")
		g.P("\t\t\t\tif string(b) == \"null\" { out[key] = nil; continue }")
		g.P("\t\t\t\tv := &", msgType, "{}")
		g.P("\t\t\t\tif err := ", protojsonUnmarshal, "(b, v); err != nil { return err }")
		g.P("\t\t\t\tout[key] = v")
		g.P("\t\t\t}")
		g.P("\t\t\tm.", f.GoName, " = out")
		g.P("\t\t\treturn nil")
		return
	}
	g.P("\t\t\tvar items map[string]json.RawMessage")
	g.P("\t\t\tif err := ", jsonUnmarshal, "(raw, &items); err != nil { return err }")
	g.P("\t\t\tout := make(map[", mapKeyGoType(keyField), "]", mapValueGoType(valField), ", len(items))")
	g.P("\t\t\tfor k, b := range items {")
	g.P("\t\t\t\tkey, err := ", mapKeyParseExpr(keyField, g), "(k)")
	g.P("\t\t\t\tif err != nil { return err }")
	g.P("\t\t\t\tvar v ", mapValueGoType(valField))
	g.P("\t\t\t\tif err := ", jsonUnmarshal, "(b, &v); err != nil { return err }")
	g.P("\t\t\t\tout[key] = v")
	g.P("\t\t\t}")
	g.P("\t\t\tm.", f.GoName, " = out")
	g.P("\t\t\treturn nil")
}

func mapKeyGoType(f *protogen.Field) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	default:
		return "string"
	}
}

func mapValueGoType(f *protogen.Field) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.BytesKind:
		return "[]byte"
	case protoreflect.EnumKind:
		return f.Enum.GoIdent.GoName
	default:
		return "any"
	}
}

func mapKeyParseExpr(f *protogen.Field, g *protogen.GeneratedFile) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "func(s string) (string, error) { return s, nil }"
	case protoreflect.BoolKind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseBool", GoImportPath: "strconv"})
		return "func(s string) (bool, error) { return strconv.ParseBool(s) }"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseInt", GoImportPath: "strconv"})
		return "func(s string) (int32, error) { v, err := strconv.ParseInt(s, 10, 32); return int32(v), err }"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseInt", GoImportPath: "strconv"})
		return "func(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseUint", GoImportPath: "strconv"})
		return "func(s string) (uint32, error) { v, err := strconv.ParseUint(s, 10, 32); return uint32(v), err }"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		_ = g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseUint", GoImportPath: "strconv"})
		return "func(s string) (uint64, error) { return strconv.ParseUint(s, 10, 64) }"
	default:
		return "func(s string) (string, error) { return s, nil }"
	}
}

func getOverrideName(fp *ir.FieldPlan) string {
	if fp == nil {
		return ""
	}
	for _, op := range fp.Ops {
		if op.Kind != ir.OpOverrideType {
			continue
		}
		return op.Data["name"]
	}
	return ""
}
