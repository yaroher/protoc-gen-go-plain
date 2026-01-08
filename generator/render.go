package generator

import (
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/ir"
	"google.golang.org/protobuf/compiler/protogen"
)

type fieldContext int

const (
	ctxField fieldContext = iota
	ctxListElem
	ctxMapValue
	ctxOneofField
)

type typeModel struct {
	basePlain    func(*fileGen) string
	pointerField bool
	fromPB       func(*fileGen, *ir.Field, string, bool) string
	toPB         func(*fileGen, *ir.Field, string, bool) string
}

var typeModels = func() map[string]typeModel {
	m := map[string]typeModel{}

	for name, w := range ir.WKTCasts {
		cast := w
		m[name] = typeModel{
			basePlain: func(*fileGen) string {
				return cast.BaseType
			},
			pointerField: true,
			fromPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
				if ptr {
					return fg.castIdent(cast.ToPtr) + "(" + src + ")"
				}
				return fg.castIdent(cast.ToVal) + "(" + src + ")"
			},
			toPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
				if ptr {
					return fg.castIdent(cast.FromPtr) + "(" + src + ")"
				}
				return fg.castIdent(cast.FromVal) + "(" + src + ")"
			},
		}
	}

	m["google.protobuf.Timestamp"] = typeModel{
		basePlain: func(fg *fileGen) string {
			return fg.timeIdent("Time")
		},
		pointerField: true,
		fromPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("TimestampToPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("TimestampToTime") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("TimestampFromPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("TimestampFromTime") + "(" + src + ")"
		},
	}
	m["google.protobuf.Duration"] = typeModel{
		basePlain: func(fg *fileGen) string {
			return fg.timeIdent("Duration")
		},
		pointerField: true,
		fromPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("DurationToPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("DurationToTime") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("DurationFromPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("DurationFromTime") + "(" + src + ")"
		},
	}
	m["google.protobuf.Struct"] = typeModel{
		basePlain: func(*fileGen) string { return "map[string]any" },
		fromPB: func(fg *fileGen, _ *ir.Field, src string, _ bool) string {
			return fg.castIdent("StructToMap") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *ir.Field, src string, _ bool) string {
			return fg.castIdent("StructFromMap") + "(" + src + ")"
		},
	}
	m["google.protobuf.Value"] = serializedModel()
	m["google.protobuf.ListValue"] = serializedModel()
	m["google.protobuf.Any"] = serializedModel()
	m["google.protobuf.Empty"] = typeModel{
		basePlain:    func(*fileGen) string { return "struct{}" },
		pointerField: true,
		fromPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("EmptyToPtrStruct") + "(" + src + ")"
			}
			return fg.castIdent("EmptyToStruct") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *ir.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("EmptyFromPtrStruct") + "(" + src + ")"
			}
			return fg.castIdent("EmptyFromStruct") + "(" + src + ")"
		},
	}

	return m
}()

func serializedModel() typeModel {
	return typeModel{
		basePlain: func(*fileGen) string { return "[]byte" },
		fromPB: func(fg *fileGen, _ *ir.Field, src string, _ bool) string {
			return fg.castIdent("MessageToSliceByte") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, field *ir.Field, src string, ptr bool) string {
			if ptr {
				src = "*" + src
			}
			return fg.castIdent("MessageFromSliceByte") + "[" + fg.pbMessagePointerType(field.MessageType) + "](" + src + ")"
		},
	}
}

type fileGen struct {
	g    *Generator
	file *protogen.File
	out  *protogen.GeneratedFile

	irFile          *ir.File
	messagesByFull  map[string]*ir.Message
	virtualMessages []*goplain.VirtualMessage
}

func newFileGen(g *Generator, f *protogen.File, model *ir.File) *fileGen {
	out := g.Plugin.NewGeneratedFile(f.GeneratedFilenamePrefix+".pb.plain.go", f.GoImportPath)
	fg := &fileGen{g: g, file: f, out: out, irFile: model}
	if model != nil {
		fg.virtualMessages = model.VirtualMessages
		fg.messagesByFull = make(map[string]*ir.Message, len(model.Messages))
		for _, msg := range model.Messages {
			if msg == nil {
				continue
			}
			fg.messagesByFull[msg.ProtoFullName] = msg
		}
	}
	return fg
}

func (fg *fileGen) P(v ...any) {
	fg.out.P(v...)
}

func (fg *fileGen) qualifiedGoIdent(id ir.GoIdent) string {
	if id.Name == "" {
		return ""
	}
	if id.ImportPath == "" {
		return id.Name
	}
	return fg.out.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: protogen.GoImportPath(id.ImportPath),
		GoName:       id.Name,
	})
}

func (fg *fileGen) castIdent(name string) string {
	return fg.out.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
		GoName:       name,
	})
}

func (fg *fileGen) timeIdent(name string) string {
	return fg.out.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: "time",
		GoName:       name,
	})
}

func (fg *fileGen) messageByFullName(name string) *ir.Message {
	if fg.messagesByFull == nil {
		return nil
	}
	return fg.messagesByFull[name]
}

func (fg *fileGen) virtualFields(msg *ir.Message) []*ir.Field {
	if msg == nil {
		return nil
	}
	out := make([]*ir.Field, 0)
	for _, field := range msg.Fields {
		if field != nil && field.IsVirtual {
			out = append(out, field)
		}
	}
	return out
}

func (fg *fileGen) virtualFieldType(field *ir.Field) string {
	if field == nil {
		return ""
	}
	if ov := fg.overrideForField(field); ov != nil {
		base := fg.overrideBaseType(ov)
		if fg.shouldPointer(field, ctxField, base) {
			return "*" + base
		}
		return base
	}
	if field.Kind == ir.KindEnum && field.EnumType != nil {
		return fg.qualifiedGoIdent(field.EnumType.GoIdent)
	}
	if field.Kind == ir.KindMessage && field.MessageType != nil {
		return fg.qualifiedGoIdent(field.MessageType.GoIdent)
	}
	base := kindToGoType(field.Kind)
	if fg.shouldPointer(field, ctxField, base) {
		return "*" + base
	}
	return base
}

func (fg *fileGen) genFile() {
	if fg.irFile == nil || !fg.hasGeneratedMessages(fg.irFile.Messages) {
		fg.out.Skip()
		return
	}

	fg.P("// Code generated by protoc-gen-go-plain. DO NOT EDIT.\n")
	fg.P("package ", fg.file.GoPackageName)
	fg.P()

	fg.emitVirtualMessages()

	for _, msg := range fg.irFile.Messages {
		fg.genMessage(msg)
	}
}

func (fg *fileGen) genMessage(msg *ir.Message) {
	if msg == nil {
		return
	}
	if msg.Generate {
		fg.genPlainStruct(msg)
		fg.genPlainOptions(msg)
		fg.genIntoPlain(msg, false)
		fg.genIntoPlain(msg, true)
		fg.genIntoPb(msg, false)
		fg.genIntoPb(msg, true)
	}
}

func (fg *fileGen) emitVirtualMessages() {
	if len(fg.virtualMessages) == 0 {
		return
	}
	for _, msg := range fg.virtualMessages {
		name := virtualMessageName(msg.Name)
		if name == "" {
			continue
		}
		fg.P("type ", name, " struct {")
		for _, field := range msg.Fields {
			fieldName := goSanitized(field.GetName())
			if fieldName == "" {
				continue
			}
			fg.P(fieldName, " ", virtualFieldType(fg.out, field))
		}
		fg.P("}")
		fg.P()
	}
}

func (fg *fileGen) hasGeneratedMessages(msgs []*ir.Message) bool {
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if msg.Generate {
			return true
		}
	}
	return false
}

func (fg *fileGen) genPlainStruct(msg *ir.Message) {
	plainName := msg.PlainName
	fg.P("type ", plainName, " struct {")

	for _, field := range msg.Fields {
		if field.IsVirtual {
			fg.P(field.GoName, " ", fg.virtualFieldType(field))
			continue
		}
		if isRealOneofField(field) {
			// oneof fields are emitted as standalone nullable fields
			fieldType := fg.plainType(field, ctxOneofField)
			fg.P(field.GoName, " ", fieldType)
			continue
		}

		if field.IsEmbedded {
			embedded := fg.messageByFullName(field.MessageType.ProtoFullName)
			if embedded != nil {
				fg.emitEmbeddedFields(embedded)
			}
			continue
		}

		fieldType := fg.plainType(field, ctxField)
		fg.P(field.GoName, " ", fieldType)
	}

	fg.P("}")
	fg.P()
}

func (fg *fileGen) genPlainOptions(msg *ir.Message) {
	extras := fg.virtualFields(msg)
	if len(extras) == 0 {
		return
	}
	plainName := msg.PlainName
	optionName := plainName + "Option"
	fg.P("type ", optionName, " func(*", plainName, ")")
	pbName := msg.GoIdent.Name
	for _, field := range extras {
		fieldName := field.GoName
		if fieldName == "" {
			continue
		}
		fg.P("func With", pbName, fieldName, "(v ", fg.virtualFieldType(field), ") ", optionName, " {")
		fg.P("return func(out *", plainName, ") { out.", fieldName, " = v }")
		fg.P("}")
	}
	fg.P()
}

func (fg *fileGen) emitEmbeddedFields(msg *ir.Message) {
	for _, field := range msg.Fields {
		if field.IsVirtual {
			continue
		}
		if isRealOneofField(field) {
			fieldType := fg.plainType(field, ctxOneofField)
			fg.P(field.GoName, " ", fieldType)
			continue
		}
		if field.IsEmbedded {
			embedded := fg.messageByFullName(field.MessageType.ProtoFullName)
			if embedded != nil {
				fg.emitEmbeddedFields(embedded)
			}
			continue
		}
		fieldType := fg.plainType(field, ctxField)
		fg.P(field.GoName, " ", fieldType)
	}
}

func (fg *fileGen) genIntoPlain(msg *ir.Message, deep bool) {
	plainName := msg.PlainName
	pbName := fg.qualifiedGoIdent(msg.GoIdent)
	methodName := "IntoPlain"
	if deep {
		methodName = "IntoPlainDeep"
	}
	extras := fg.virtualFields(msg)
	if len(extras) == 0 {
		fg.P("func (v *", pbName, ") ", methodName, "() *", plainName, " {")
	} else {
		fg.P("func (v *", pbName, ") ", methodName, "(opts ...", plainName, "Option) *", plainName, " {")
	}
	fg.P("if v == nil { return nil }")
	fg.P("out := &", plainName, "{}")

	for _, field := range msg.Fields {
		if field.IsVirtual {
			continue
		}
		if isRealOneofField(field) {
			continue
		}
		if field.IsEmbedded {
			fg.emitFromPBEmbedded("out", "v", field, deep)
			continue
		}
		fg.emitFromPBField("out", "v", field, deep)
	}

	for _, oneof := range msg.Oneofs {
		fg.emitFromPBOneof("out", "v", msg, oneof, deep)
	}

	if len(extras) > 0 {
		fg.P("for _, opt := range opts {")
		fg.P("if opt != nil { opt(out) }")
		fg.P("}")
	}
	fg.P("return out")
	fg.P("}")
	fg.P()
}

func (fg *fileGen) genIntoPb(msg *ir.Message, deep bool) {
	plainName := msg.PlainName
	pbName := fg.qualifiedGoIdent(msg.GoIdent)
	methodName := "IntoPb"
	if deep {
		methodName = "IntoPbDeep"
	}
	fg.P("func (v *", plainName, ") ", methodName, "() *", pbName, " {")
	fg.P("if v == nil { return nil }")
	fg.P("out := &", pbName, "{}")

	for _, field := range msg.Fields {
		if field.IsVirtual {
			continue
		}
		if isRealOneofField(field) {
			continue
		}
		if field.IsEmbedded {
			fg.emitToPBEmbedded("out", "v", field, deep)
			continue
		}
		fg.emitToPBField("out", "v", field, deep)
	}

	for _, oneof := range msg.Oneofs {
		fg.emitToPBOneof("out", "v", msg, oneof, deep)
	}

	fg.P("return out")
	fg.P("}")
	fg.P()
}

func (fg *fileGen) emitFromPBEmbedded(outVar, srcVar string, field *ir.Field, deep bool) {
	if field.MessageType == nil {
		return
	}
	msg := fg.messageByFullName(field.MessageType.ProtoFullName)
	if msg == nil {
		return
	}
	fg.P("if ", srcVar, ".", field.GoName, " != nil {")
	fg.emitEmbeddedAssignFrom(srcVar+"."+field.GoName, outVar, msg, deep)
	fg.P("}")
}

func (fg *fileGen) emitEmbeddedAssignFrom(srcVar, outVar string, msg *ir.Message, deep bool) {
	for _, field := range msg.Fields {
		if field.IsVirtual {
			continue
		}
		if isRealOneofField(field) {
			fg.P(outVar, ".", field.GoName, " = ", fg.pbToPlainExpr(field, srcVar+"."+field.GoName, ctxField, deep))
			continue
		}
		if field.IsEmbedded {
			if field.MessageType != nil {
				if embedded := fg.messageByFullName(field.MessageType.ProtoFullName); embedded != nil {
					fg.emitEmbeddedAssignFrom(srcVar+"."+field.GoName, outVar, embedded, deep)
				}
			}
			continue
		}
		fg.P(outVar, ".", field.GoName, " = ", fg.pbToPlainExpr(field, srcVar+"."+field.GoName, ctxField, deep))
	}
}

func (fg *fileGen) emitToPBEmbedded(outVar, srcVar string, field *ir.Field, deep bool) {
	if field.MessageType == nil {
		return
	}
	msg := fg.messageByFullName(field.MessageType.ProtoFullName)
	if msg == nil {
		return
	}
	fg.P(outVar, ".", field.GoName, " = &", fg.qualifiedGoIdent(field.MessageType.GoIdent), "{}")
	fg.emitEmbeddedAssignTo(outVar+"."+field.GoName, srcVar, msg, deep)
}

func (fg *fileGen) emitEmbeddedAssignTo(tmpVar, srcVar string, msg *ir.Message, deep bool) {
	for _, field := range msg.Fields {
		if field.IsVirtual {
			continue
		}
		if isRealOneofField(field) {
			fg.P(tmpVar, ".", field.GoName, " = ", fg.plainToPBExpr(field, srcVar+"."+field.GoName, ctxField, deep))
			continue
		}
		if field.IsEmbedded {
			if field.MessageType != nil {
				if embedded := fg.messageByFullName(field.MessageType.ProtoFullName); embedded != nil {
					fg.emitEmbeddedAssignTo(tmpVar, srcVar, embedded, deep)
				}
			}
			continue
		}
		fg.P(tmpVar, ".", field.GoName, " = ", fg.plainToPBExpr(field, srcVar+"."+field.GoName, ctxField, deep))
	}
}

func isPointerType(typeName string) bool {
	return strings.HasPrefix(typeName, "*")
}

func cloneBytesExpr(src string) string {
	return "append([]byte(nil), " + src + "...)"
}

func (fg *fileGen) aliasValueField(msg *ir.Message) *ir.Field {
	if msg == nil || !msg.TypeAlias {
		return nil
	}
	return msg.AliasValueField
}

func (fg *fileGen) aliasMessageFromField(field *ir.Field) *ir.Message {
	if field == nil {
		return nil
	}
	if field.AliasFromFull != "" {
		msg := fg.messageByFullName(field.AliasFromFull)
		if msg != nil && msg.TypeAlias {
			return msg
		}
		return nil
	}
	if field.Kind == ir.KindMessage && field.MessageType != nil {
		msg := fg.messageByFullName(field.MessageType.ProtoFullName)
		if msg != nil && msg.TypeAlias {
			return msg
		}
	}
	return nil
}

func (fg *fileGen) aliasValueFieldFromField(field *ir.Field) *ir.Field {
	msg := fg.aliasMessageFromField(field)
	return fg.aliasValueField(msg)
}

func (fg *fileGen) aliasPBToPlainExpr(field, aliasField *ir.Field, src string, ctx fieldContext, deep bool) string {
	plainType := fg.plainType(field, ctx)
	valExpr := fg.pbToPlainExpr(aliasField, src+".Value", ctx, deep)
	retExpr := "val"
	if ctx == ctxOneofField && aliasField.Kind != ir.KindMessage {
		retExpr = "&val"
	}
	return "func() " + plainType + " { if " + src + " == nil { var zero " + plainType + "; return zero }; val := " + valExpr + "; return " + retExpr + " }()"
}

func (fg *fileGen) aliasPlainToPBExpr(field, aliasField *ir.Field, src string, ctx fieldContext, deep bool) string {
	plainType := fg.plainType(field, ctx)
	valSrc := src
	if ctx == ctxOneofField && aliasField.Kind != ir.KindMessage {
		valSrc = "*" + src
	}
	valExpr := fg.plainToPBExpr(aliasField, valSrc, ctx, deep)
	msg := fg.aliasMessageFromField(field)
	if msg == nil {
		panic("alias message not found for field " + field.GoName)
	}
	msgType := fg.qualifiedGoIdent(msg.GoIdent)
	if isPointerType(plainType) {
		return "func() *" + msgType + " { if " + src + " == nil { return nil }; return &" + msgType + "{Value: " + valExpr + "} }()"
	}
	return "&" + msgType + "{Value: " + valExpr + "}"
}

func (fg *fileGen) emitFromPBField(outVar, srcVar string, field *ir.Field, deep bool) {
	embeddedVar := ""
	srcField := srcVar + "." + field.GoName
	if field.EmbeddedFrom != "" {
		embeddedVar = srcVar + "." + field.EmbeddedFrom
		srcField = embeddedVar + "." + field.GoName
		fg.P("if ", embeddedVar, " != nil {")
	}

	if field.IsMap {
		if field.MapValue == nil {
			if embeddedVar != "" {
				fg.P("}")
			}
			return
		}
		if !deep && !fg.requiresConversion(field.MapValue) {
			fg.P(outVar, ".", field.GoName, " = ", srcField)
			if embeddedVar != "" {
				fg.P("}")
			}
			return
		}
		keyField := field.MapKey
		valField := field.MapValue
		keyType := fg.mapKeyType(keyField)
		valType := fg.plainType(valField, ctxMapValue)
		fg.P("if ", srcField, " != nil {")
		fg.P(outVar, ".", field.GoName, " = make(map[", keyType, "]", valType, ", len(", srcField, "))")
		fg.P("for k, val := range ", srcField, " {")
		fg.P(outVar, ".", field.GoName, "[k] = ", fg.pbToPlainExpr(valField, "val", ctxMapValue, deep))
		fg.P("}")
		fg.P("}")
		if embeddedVar != "" {
			fg.P("}")
		}
		return
	}

	if field.IsList {
		if !deep && !fg.requiresConversion(field) {
			fg.P(outVar, ".", field.GoName, " = ", srcField)
			if embeddedVar != "" {
				fg.P("}")
			}
			return
		}
		fg.P("if ", srcField, " != nil {")
		fg.P("for _, el := range ", srcField, " {")
		fg.P(outVar, ".", field.GoName, " = append(", outVar, ".", field.GoName, ", ", fg.pbToPlainExpr(field, "el", ctxListElem, deep), ")")
		fg.P("}")
		fg.P("}")
		if embeddedVar != "" {
			fg.P("}")
		}
		return
	}

	expr := fg.pbToPlainExpr(field, srcField, ctxField, deep)
	fg.P(outVar, ".", field.GoName, " = ", expr)
	if embeddedVar != "" {
		fg.P("}")
	}
}

func (fg *fileGen) emitToPBField(outVar, srcVar string, field *ir.Field, deep bool) {
	dstField := outVar + "." + field.GoName
	if field.EmbeddedFrom != "" {
		embeddedVar := outVar + "." + field.EmbeddedFrom
		embeddedType := fg.embeddedFromType(field)
		if embeddedType == "" {
			panic("embedded_from type not found for " + field.EmbeddedFrom)
		}
		fg.P("if ", embeddedVar, " == nil { ", embeddedVar, " = &", embeddedType, "{} }")
		dstField = embeddedVar + "." + field.GoName
	}

	if field.IsMap {
		if field.MapValue == nil {
			return
		}
		if !deep && !fg.requiresConversion(field.MapValue) {
			fg.P(dstField, " = ", srcVar, ".", field.GoName)
			return
		}
		keyField := field.MapKey
		valField := field.MapValue
		keyType := fg.mapKeyType(keyField)
		valType := fg.pbValueType(valField)
		fg.P("if ", srcVar, ".", field.GoName, " != nil {")
		fg.P(dstField, " = make(map[", keyType, "]", valType, ", len(", srcVar, ".", field.GoName, "))")
		fg.P("for k, val := range ", srcVar, ".", field.GoName, " {")
		fg.P(dstField, "[k] = ", fg.plainToPBExpr(valField, "val", ctxMapValue, deep))
		fg.P("}")
		fg.P("}")
		return
	}

	if field.IsList {
		if !deep && !fg.requiresConversion(field) {
			fg.P(dstField, " = ", srcVar, ".", field.GoName)
			return
		}
		fg.P("if ", srcVar, ".", field.GoName, " != nil {")
		fg.P("for _, el := range ", srcVar, ".", field.GoName, " {")
		fg.P(dstField, " = append(", dstField, ", ", fg.plainToPBExpr(field, "el", ctxListElem, deep), ")")
		fg.P("}")
		fg.P("}")
		return
	}

	expr := fg.plainToPBExpr(field, srcVar+"."+field.GoName, ctxField, deep)
	fg.P(dstField, " = ", expr)
}

func (fg *fileGen) emitFromPBOneof(outVar, srcVar string, msg *ir.Message, oneof *ir.Oneof, deep bool) {
	fg.P("switch t := ", srcVar, ".", oneof.GoName, ".(type) {")
	for _, field := range oneof.Fields {
		pbWrapper := fg.pbOneofWrapperName(msg, field)
		plainField := field.GoName
		plainType := fg.plainType(field, ctxOneofField)
		fg.P("case *", pbWrapper, ":")
		expr := fg.pbToPlainExpr(field, "t."+field.GoName, ctxOneofField, deep)
		if strings.HasPrefix(plainType, "*") && field.Kind != ir.KindMessage {
			fg.P("val := ", expr)
			fg.P(outVar, ".", plainField, " = &val")
			continue
		}
		fg.P(outVar, ".", plainField, " = ", expr)
	}
	fg.P("}")
}

func (fg *fileGen) emitToPBOneof(outVar, srcVar string, msg *ir.Message, oneof *ir.Oneof, deep bool) {
	first := true
	for _, field := range oneof.Fields {
		cond := srcVar + "." + field.GoName + " != nil"
		if first {
			fg.P("if ", cond, " {")
			first = false
		} else {
			fg.P("} else if ", cond, " {")
		}

		pbWrapper := fg.pbOneofWrapperName(msg, field)
		plainSrc := srcVar + "." + field.GoName
		valueExpr := fg.oneofPlainToPBExpr(field, plainSrc, deep)
		fg.P(outVar, ".", oneof.GoName, " = &", pbWrapper, "{", field.GoName, ": ", valueExpr, "}")
	}
	if !first {
		fg.P("}")
	}
}

func (fg *fileGen) oneofPlainToPBExpr(field *ir.Field, src string, deep bool) string {
	if field.Kind == ir.KindMessage {
		return fg.plainToPBExpr(field, src, ctxOneofField, deep)
	}
	return fg.plainToPBExpr(field, "*"+src, ctxOneofField, deep)
}

func (fg *fileGen) pbToPlainExpr(field *ir.Field, src string, ctx fieldContext, deep bool) string {
	if fg.overrideForField(field) != nil {
		if model, ok := fg.model(field); ok {
			ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
			if ctx == ctxOneofField && field.Kind != ir.KindMessage {
				ptr = false
			}
			return model.fromPB(fg, field, src, ptr)
		}
	}

	if field.IsSerialized {
		return fg.castIdent("MessageToSliceByte") + "(" + src + ")"
	}

	if model, ok := fg.model(field); ok {
		ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
		if ctx == ctxOneofField && field.Kind != ir.KindMessage {
			ptr = false
		}
		return model.fromPB(fg, field, src, ptr)
	}

	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return fg.aliasPBToPlainExpr(field, aliasField, src, ctx, deep)
	}

	if field.Kind == ir.KindBytes && deep {
		return cloneBytesExpr(src)
	}

	if field.Kind == ir.KindMessage {
		return src
	}

	return src
}

func (fg *fileGen) plainToPBExpr(field *ir.Field, src string, ctx fieldContext, deep bool) string {
	if fg.overrideForField(field) != nil {
		if model, ok := fg.model(field); ok {
			ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
			if ctx == ctxOneofField && field.Kind != ir.KindMessage {
				ptr = false
			}
			return model.toPB(fg, field, src, ptr)
		}
	}

	if field.IsSerialized {
		if isPointerType(fg.plainType(field, ctx)) {
			src = "*" + src
		}
		return fg.castIdent("MessageFromSliceByte") + "[" + fg.serializedMessagePointerType(field) + "](" + src + ")"
	}

	if model, ok := fg.model(field); ok {
		ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
		if ctx == ctxOneofField && field.Kind != ir.KindMessage {
			ptr = false
		}
		return model.toPB(fg, field, src, ptr)
	}

	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return fg.aliasPlainToPBExpr(field, aliasField, src, ctx, deep)
	}

	if field.Kind == ir.KindBytes && deep {
		return cloneBytesExpr(src)
	}

	if field.Kind == ir.KindMessage {
		return src
	}

	return src
}

func (fg *fileGen) model(field *ir.Field) (typeModel, bool) {
	if ov := fg.overrideForField(field); ov != nil {
		base := fg.overrideBaseType(ov)
		model := typeModel{
			basePlain: func(*fileGen) string {
				return base
			},
			pointerField: ov.GetPointer(),
			fromPB: func(fg *fileGen, field *ir.Field, src string, ptr bool) string {
				return fg.overrideFromPBExpr(field, src, ptr, ov)
			},
			toPB: func(fg *fileGen, field *ir.Field, src string, ptr bool) string {
				return fg.overrideToPBExpr(field, src, ptr, ov)
			},
		}
		return model, true
	}
	if field.Kind != ir.KindMessage || field.MessageType == nil {
		return typeModel{}, false
	}
	model, ok := typeModels[field.MessageType.ProtoFullName]
	return model, ok
}

func (fg *fileGen) requiresConversion(field *ir.Field) bool {
	if fg.overrideForField(field) != nil {
		return true
	}
	if field.IsSerialized {
		return true
	}
	if fg.aliasValueFieldFromField(field) != nil {
		return true
	}
	if _, ok := fg.model(field); ok {
		return true
	}
	return false
}

func (fg *fileGen) plainType(field *ir.Field, ctx fieldContext) string {
	if ctx == ctxField {
		if field.IsMap {
			keyType := fg.mapKeyType(field.MapKey)
			valType := fg.plainType(field.MapValue, ctxMapValue)
			return "map[" + keyType + "]" + valType
		}
		if field.IsList {
			elemType := fg.plainType(field, ctxListElem)
			return "[]" + elemType
		}
	}

	base := fg.plainBaseType(field)
	if fg.shouldPointer(field, ctx, base) {
		return "*" + base
	}
	return base
}

func (fg *fileGen) plainBaseType(field *ir.Field) string {
	if ov := fg.overrideForField(field); ov != nil {
		return fg.overrideBaseType(ov)
	}
	if field.IsSerialized {
		return "[]byte"
	}
	if model, ok := fg.model(field); ok {
		return model.basePlain(fg)
	}
	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return fg.plainBaseType(aliasField)
	}
	if field.Kind == ir.KindEnum && field.EnumType != nil {
		return fg.qualifiedGoIdent(field.EnumType.GoIdent)
	}
	if field.Kind == ir.KindMessage && field.MessageType != nil {
		return fg.qualifiedGoIdent(field.MessageType.GoIdent)
	}
	return kindToGoType(field.Kind)
}

func (fg *fileGen) shouldPointer(field *ir.Field, ctx fieldContext, base string) bool {
	if strings.HasPrefix(base, "*") {
		return false
	}
	if ov := fg.overrideForField(field); ov != nil {
		if ctx == ctxOneofField {
			return true
		}
		return ov.GetPointer() && ctx == ctxField
	}
	if field.IsSerialized {
		return ctx == ctxOneofField
	}
	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		aliasBase := fg.plainBaseType(aliasField)
		return fg.shouldPointer(aliasField, ctx, aliasBase)
	}
	if ctx == ctxOneofField {
		return true
	}
	if model, ok := fg.model(field); ok {
		return model.pointerField && ctx == ctxField
	}
	if field.Kind == ir.KindMessage {
		return true
	}
	if ctx == ctxField && isIRFieldNullable(field) {
		return true
	}
	return false
}

func (fg *fileGen) mapKeyType(field *ir.Field) string {
	if field == nil {
		return ""
	}
	return kindToGoType(field.Kind)
}

func (fg *fileGen) embeddedFromType(field *ir.Field) string {
	if field == nil || field.EmbeddedFromFull == "" {
		return ""
	}
	if msg := fg.messageByFullName(field.EmbeddedFromFull); msg != nil {
		return fg.qualifiedGoIdent(msg.GoIdent)
	}
	return ""
}

func (fg *fileGen) pbValueType(field *ir.Field) string {
	if field.Kind == ir.KindEnum && field.EnumType != nil {
		return fg.qualifiedGoIdent(field.EnumType.GoIdent)
	}
	if field.Kind == ir.KindMessage && field.MessageType != nil {
		return "*" + fg.qualifiedGoIdent(field.MessageType.GoIdent)
	}
	return kindToGoType(field.Kind)
}

func (fg *fileGen) pbOneofWrapperName(msg *ir.Message, field *ir.Field) string {
	return msg.GoIdent.Name + "_" + field.GoName
}

func (fg *fileGen) pbMessagePointerType(msg *ir.TypeRef) string {
	if msg == nil {
		return ""
	}
	return "*" + fg.qualifiedGoIdent(msg.GoIdent)
}

func (fg *fileGen) serializedMessagePointerType(field *ir.Field) string {
	if field == nil {
		return ""
	}
	if field.MessageType != nil {
		return fg.pbMessagePointerType(field.MessageType)
	}
	if field.SerializedFromFull != "" {
		if msg := fg.messageByFullName(field.SerializedFromFull); msg != nil {
			return "*" + fg.qualifiedGoIdent(msg.GoIdent)
		}
	}
	panic("serialized field missing message type: " + field.GoName)
}

func kindToGoType(kind ir.Kind) string {
	switch kind {
	case ir.KindBool:
		return "bool"
	case ir.KindInt32, ir.KindSint32, ir.KindSfixed32:
		return "int32"
	case ir.KindUint32, ir.KindFixed32:
		return "uint32"
	case ir.KindInt64, ir.KindSint64, ir.KindSfixed64:
		return "int64"
	case ir.KindUint64, ir.KindFixed64:
		return "uint64"
	case ir.KindFloat:
		return "float32"
	case ir.KindDouble:
		return "float64"
	case ir.KindString:
		return "string"
	case ir.KindBytes:
		return "[]byte"
	default:
		return ""
	}
}

func isRealOneofField(field *ir.Field) bool {
	return field != nil && field.Oneof != nil
}

func isIRFieldNullable(field *ir.Field) bool {
	return field != nil && (field.IsOptional || field.Oneof != nil)
}
