package generator

import (
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	fromPB       func(*fileGen, *protogen.Field, string, bool) string
	toPB         func(*fileGen, *protogen.Field, string, bool) string
}

type wrapperCast struct {
	baseType string
	toVal    string
	toPtr    string
	fromVal  string
	fromPtr  string
}

var wrapperCasts = map[protoreflect.FullName]wrapperCast{
	"google.protobuf.StringValue": {
		baseType: "string",
		toVal:    "StringValueToString",
		toPtr:    "StringValueToPtrString",
		fromVal:  "StringValueFromString",
		fromPtr:  "StringValueFromPtrString",
	},
	"google.protobuf.BoolValue": {
		baseType: "bool",
		toVal:    "BoolValueToBool",
		toPtr:    "BoolValueToPtrBool",
		fromVal:  "BoolValueFromBool",
		fromPtr:  "BoolValueFromPtrBool",
	},
	"google.protobuf.Int32Value": {
		baseType: "int32",
		toVal:    "Int32ValueToInt32",
		toPtr:    "Int32ValueToPtrInt32",
		fromVal:  "Int32ValueFromInt32",
		fromPtr:  "Int32ValueFromPtrInt32",
	},
	"google.protobuf.Int64Value": {
		baseType: "int64",
		toVal:    "Int64ValueToInt64",
		toPtr:    "Int64ValueToPtrInt64",
		fromVal:  "Int64ValueFromInt64",
		fromPtr:  "Int64ValueFromPtrInt64",
	},
	"google.protobuf.UInt32Value": {
		baseType: "uint32",
		toVal:    "UInt32ValueToUint32",
		toPtr:    "UInt32ValueToPtrUint32",
		fromVal:  "UInt32ValueFromUint32",
		fromPtr:  "UInt32ValueFromPtrUint32",
	},
	"google.protobuf.UInt64Value": {
		baseType: "uint64",
		toVal:    "UInt64ValueToUint64",
		toPtr:    "UInt64ValueToPtrUint64",
		fromVal:  "UInt64ValueFromUint64",
		fromPtr:  "UInt64ValueFromPtrUint64",
	},
	"google.protobuf.FloatValue": {
		baseType: "float32",
		toVal:    "FloatValueToFloat32",
		toPtr:    "FloatValueToPtrFloat32",
		fromVal:  "FloatValueFromFloat32",
		fromPtr:  "FloatValueFromPtrFloat32",
	},
	"google.protobuf.DoubleValue": {
		baseType: "float64",
		toVal:    "DoubleValueToFloat64",
		toPtr:    "DoubleValueToPtrFloat64",
		fromVal:  "DoubleValueFromFloat64",
		fromPtr:  "DoubleValueFromPtrFloat64",
	},
	"google.protobuf.BytesValue": {
		baseType: "[]byte",
		toVal:    "BytesValueToBytes",
		toPtr:    "BytesValueToPtrBytes",
		fromVal:  "BytesValueFromBytes",
		fromPtr:  "BytesValueFromPtrBytes",
	},
}

var typeModels = func() map[protoreflect.FullName]typeModel {
	m := map[protoreflect.FullName]typeModel{}

	for name, w := range wrapperCasts {
		cast := w
		m[name] = typeModel{
			basePlain: func(*fileGen) string {
				return cast.baseType
			},
			pointerField: true,
			fromPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
				if ptr {
					return fg.castIdent(cast.toPtr) + "(" + src + ")"
				}
				return fg.castIdent(cast.toVal) + "(" + src + ")"
			},
			toPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
				if ptr {
					return fg.castIdent(cast.fromPtr) + "(" + src + ")"
				}
				return fg.castIdent(cast.fromVal) + "(" + src + ")"
			},
		}
	}

	m["google.protobuf.Timestamp"] = typeModel{
		basePlain: func(fg *fileGen) string {
			return fg.timeIdent("Time")
		},
		pointerField: true,
		fromPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("TimestampToPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("TimestampToTime") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
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
		fromPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("DurationToPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("DurationToTime") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("DurationFromPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("DurationFromTime") + "(" + src + ")"
		},
	}
	m["google.protobuf.Struct"] = typeModel{
		basePlain: func(*fileGen) string { return "map[string]any" },
		fromPB: func(fg *fileGen, _ *protogen.Field, src string, _ bool) string {
			return fg.castIdent("StructToMap") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *protogen.Field, src string, _ bool) string {
			return fg.castIdent("StructFromMap") + "(" + src + ")"
		},
	}
	m["google.protobuf.Value"] = serializedModel()
	m["google.protobuf.ListValue"] = serializedModel()
	m["google.protobuf.Any"] = serializedModel()
	m["google.protobuf.Empty"] = typeModel{
		basePlain:    func(*fileGen) string { return "struct{}" },
		pointerField: true,
		fromPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("EmptyToPtrStruct") + "(" + src + ")"
			}
			return fg.castIdent("EmptyToStruct") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, _ *protogen.Field, src string, ptr bool) string {
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
		fromPB: func(fg *fileGen, _ *protogen.Field, src string, _ bool) string {
			return fg.castIdent("MessageToSliceByte") + "(" + src + ")"
		},
		toPB: func(fg *fileGen, field *protogen.Field, src string, ptr bool) string {
			if ptr {
				src = "*" + src
			}
			return fg.castIdent("MessageFromSliceByte") + "[" + fg.pbMessagePointerType(field.Message) + "](" + src + ")"
		},
	}
}

type fileGen struct {
	g    *Generator
	file *protogen.File
	out  *protogen.GeneratedFile
}

func newFileGen(g *Generator, f *protogen.File) *fileGen {
	out := g.Plugin.NewGeneratedFile(f.GeneratedFilenamePrefix+".pb.plain.go", f.GoImportPath)
	return &fileGen{g: g, file: f, out: out}
}

func (fg *fileGen) P(v ...any) {
	fg.out.P(v...)
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

func (fg *fileGen) genFile() {
	if !fg.hasGeneratedMessages(fg.file.Messages) {
		fg.out.Skip()
		return
	}

	fg.P("// Code generated by protoc-gen-go-plain. DO NOT EDIT.\n")
	fg.P("package ", fg.file.GoPackageName)
	fg.P()

	for _, msg := range fg.file.Messages {
		fg.genMessage(msg)
	}
}

func (fg *fileGen) genMessage(msg *protogen.Message) {
	if msg.Desc.IsMapEntry() {
		return
	}
	if shouldGenerateMessage(msg) {
		fg.genPlainStruct(msg)
		fg.genIntoPlain(msg, false)
		fg.genIntoPlain(msg, true)
		fg.genIntoPb(msg, false)
		fg.genIntoPb(msg, true)
	}

	for _, child := range msg.Messages {
		fg.genMessage(child)
	}
}

func (fg *fileGen) hasGeneratedMessages(msgs []*protogen.Message) bool {
	for _, msg := range msgs {
		if msg.Desc.IsMapEntry() {
			continue
		}
		if shouldGenerateMessage(msg) {
			return true
		}
		if fg.hasGeneratedMessages(msg.Messages) {
			return true
		}
	}
	return false
}

func (fg *fileGen) genPlainStruct(msg *protogen.Message) {
	plainName := fg.plainMessageName(msg)
	fg.P("type ", plainName, " struct {")

	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			// oneof fields are emitted as standalone nullable fields
			fieldType := fg.plainType(field, ctxOneofField)
			fg.P(field.GoName, " ", fieldType)
			continue
		}

		if isEmbeddedMessage(field) {
			fg.emitEmbeddedFields(field.Message)
			continue
		}

		fieldType := fg.plainType(field, ctxField)
		fg.P(field.GoName, " ", fieldType)
	}
	fg.P("}")
	fg.P()
}

func (fg *fileGen) emitEmbeddedFields(msg *protogen.Message) {
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			fieldType := fg.plainType(field, ctxOneofField)
			fg.P(field.GoName, " ", fieldType)
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitEmbeddedFields(field.Message)
			continue
		}
		fieldType := fg.plainType(field, ctxField)
		fg.P(field.GoName, " ", fieldType)
	}
}

func (fg *fileGen) genIntoPlain(msg *protogen.Message, deep bool) {
	plainName := fg.plainMessageName(msg)
	pbName := fg.out.QualifiedGoIdent(msg.GoIdent)
	methodName := "IntoPlain"
	if deep {
		methodName = "IntoPlainDeep"
	}
	fg.P("func (v *", pbName, ") ", methodName, "() *", plainName, " {")
	fg.P("if v == nil { return nil }")
	fg.P("out := &", plainName, "{}")

	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitFromPBEmbedded("out", "v", field, deep)
			continue
		}
		fg.emitFromPBField("out", "v", field, deep)
	}

	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue
		}
		fg.emitFromPBOneof("out", "v", msg, oneof, deep)
	}

	fg.P("return out")
	fg.P("}")
	fg.P()
}

func (fg *fileGen) genIntoPb(msg *protogen.Message, deep bool) {
	plainName := fg.plainMessageName(msg)
	pbName := fg.out.QualifiedGoIdent(msg.GoIdent)
	methodName := "IntoPb"
	if deep {
		methodName = "IntoPbDeep"
	}
	fg.P("func (v *", plainName, ") ", methodName, "() *", pbName, " {")
	fg.P("if v == nil { return nil }")
	fg.P("out := &", pbName, "{}")

	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitToPBEmbedded("out", "v", field, deep)
			continue
		}
		fg.emitToPBField("out", "v", field, deep)
	}

	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue
		}
		fg.emitToPBOneof("out", "v", msg, oneof, deep)
	}

	fg.P("return out")
	fg.P("}")
	fg.P()
}

func (fg *fileGen) emitFromPBEmbedded(outVar, srcVar string, field *protogen.Field, deep bool) {
	fg.P("if ", srcVar, ".", field.GoName, " != nil {")
	fg.emitEmbeddedAssignFrom(srcVar+"."+field.GoName, outVar, field.Message, deep)
	fg.P("}")
}

func (fg *fileGen) emitEmbeddedAssignFrom(srcVar, outVar string, msg *protogen.Message, deep bool) {
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			fg.P(outVar, ".", field.GoName, " = ", fg.pbToPlainExpr(field, srcVar+"."+field.GoName, ctxField, deep))
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitEmbeddedAssignFrom(srcVar+"."+field.GoName, outVar, field.Message, deep)
			continue
		}
		fg.P(outVar, ".", field.GoName, " = ", fg.pbToPlainExpr(field, srcVar+"."+field.GoName, ctxField, deep))
	}
}

func (fg *fileGen) emitToPBEmbedded(outVar, srcVar string, field *protogen.Field, deep bool) {
	fg.P(outVar, ".", field.GoName, " = &", fg.out.QualifiedGoIdent(field.Message.GoIdent), "{}")
	fg.emitEmbeddedAssignTo(outVar+"."+field.GoName, srcVar, field.Message, deep)
}

func (fg *fileGen) emitEmbeddedAssignTo(tmpVar, srcVar string, msg *protogen.Message, deep bool) {
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			fg.P(tmpVar, ".", field.GoName, " = ", fg.plainToPBExpr(field, srcVar+"."+field.GoName, ctxField, deep))
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitEmbeddedAssignTo(tmpVar, srcVar, field.Message, deep)
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

func (fg *fileGen) emitFromPBField(outVar, srcVar string, field *protogen.Field, deep bool) {
	if field.Desc.IsMap() {
		if !deep && !fg.requiresConversion(field.Message.Fields[1]) {
			fg.P(outVar, ".", field.GoName, " = ", srcVar, ".", field.GoName)
			return
		}
		keyField := field.Message.Fields[0]
		valField := field.Message.Fields[1]
		keyType := fg.mapKeyType(keyField)
		valType := fg.plainType(valField, ctxMapValue)
		fg.P("if ", srcVar, ".", field.GoName, " != nil {")
		fg.P(outVar, ".", field.GoName, " = make(map[", keyType, "]", valType, ", len(", srcVar, ".", field.GoName, "))")
		fg.P("for k, val := range ", srcVar, ".", field.GoName, " {")
		fg.P(outVar, ".", field.GoName, "[k] = ", fg.pbToPlainExpr(valField, "val", ctxMapValue, deep))
		fg.P("}")
		fg.P("}")
		return
	}

	if field.Desc.IsList() {
		if !deep && !fg.requiresConversion(field) {
			fg.P(outVar, ".", field.GoName, " = ", srcVar, ".", field.GoName)
			return
		}
		fg.P("if ", srcVar, ".", field.GoName, " != nil {")
		fg.P("for _, el := range ", srcVar, ".", field.GoName, " {")
		fg.P(outVar, ".", field.GoName, " = append(", outVar, ".", field.GoName, ", ", fg.pbToPlainExpr(field, "el", ctxListElem, deep), ")")
		fg.P("}")
		fg.P("}")
		return
	}

	expr := fg.pbToPlainExpr(field, srcVar+"."+field.GoName, ctxField, deep)
	fg.P(outVar, ".", field.GoName, " = ", expr)
}

func (fg *fileGen) emitToPBField(outVar, srcVar string, field *protogen.Field, deep bool) {
	if field.Desc.IsMap() {
		if !deep && !fg.requiresConversion(field.Message.Fields[1]) {
			fg.P(outVar, ".", field.GoName, " = ", srcVar, ".", field.GoName)
			return
		}
		keyField := field.Message.Fields[0]
		valField := field.Message.Fields[1]
		keyType := fg.mapKeyType(keyField)
		valType := fg.pbValueType(valField)
		fg.P("if ", srcVar, ".", field.GoName, " != nil {")
		fg.P(outVar, ".", field.GoName, " = make(map[", keyType, "]", valType, ", len(", srcVar, ".", field.GoName, "))")
		fg.P("for k, val := range ", srcVar, ".", field.GoName, " {")
		fg.P(outVar, ".", field.GoName, "[k] = ", fg.plainToPBExpr(valField, "val", ctxMapValue, deep))
		fg.P("}")
		fg.P("}")
		return
	}

	if field.Desc.IsList() {
		if !deep && !fg.requiresConversion(field) {
			fg.P(outVar, ".", field.GoName, " = ", srcVar, ".", field.GoName)
			return
		}
		fg.P("if ", srcVar, ".", field.GoName, " != nil {")
		fg.P("for _, el := range ", srcVar, ".", field.GoName, " {")
		fg.P(outVar, ".", field.GoName, " = append(", outVar, ".", field.GoName, ", ", fg.plainToPBExpr(field, "el", ctxListElem, deep), ")")
		fg.P("}")
		fg.P("}")
		return
	}

	expr := fg.plainToPBExpr(field, srcVar+"."+field.GoName, ctxField, deep)
	fg.P(outVar, ".", field.GoName, " = ", expr)
}

func (fg *fileGen) emitFromPBOneof(outVar, srcVar string, msg *protogen.Message, oneof *protogen.Oneof, deep bool) {
	fg.P("switch t := ", srcVar, ".", oneof.GoName, ".(type) {")
	for _, field := range oneof.Fields {
		pbWrapper := fg.pbOneofWrapperName(msg, field)
		plainField := field.GoName
		plainType := fg.plainType(field, ctxOneofField)
		fg.P("case *", pbWrapper, ":")
		expr := fg.pbToPlainExpr(field, "t."+field.GoName, ctxOneofField, deep)
		if strings.HasPrefix(plainType, "*") && field.Desc.Kind() != protoreflect.MessageKind {
			fg.P("val := ", expr)
			fg.P(outVar, ".", plainField, " = &val")
			continue
		}
		fg.P(outVar, ".", plainField, " = ", expr)
	}
	fg.P("}")
}

func (fg *fileGen) emitToPBOneof(outVar, srcVar string, msg *protogen.Message, oneof *protogen.Oneof, deep bool) {
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

func (fg *fileGen) oneofPlainToPBExpr(field *protogen.Field, src string, deep bool) string {
	if field.Desc.Kind() == protoreflect.MessageKind {
		return fg.plainToPBExpr(field, src, ctxOneofField, deep)
	}
	return fg.plainToPBExpr(field, "*"+src, ctxOneofField, deep)
}

func (fg *fileGen) pbToPlainExpr(field *protogen.Field, src string, ctx fieldContext, deep bool) string {
	if isSerializedMessage(field) {
		return fg.castIdent("MessageToSliceByte") + "(" + src + ")"
	}

	if model, ok := fg.model(field); ok {
		ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
		return model.fromPB(fg, field, src, ptr)
	}

	if field.Desc.Kind() == protoreflect.BytesKind && deep {
		return cloneBytesExpr(src)
	}

	if field.Desc.Kind() == protoreflect.MessageKind {
		return src
	}

	return src
}

func (fg *fileGen) plainToPBExpr(field *protogen.Field, src string, ctx fieldContext, deep bool) string {
	if isSerializedMessage(field) {
		if isPointerType(fg.plainType(field, ctx)) {
			src = "*" + src
		}
		return fg.castIdent("MessageFromSliceByte") + "[" + fg.pbMessagePointerType(field.Message) + "](" + src + ")"
	}

	if model, ok := fg.model(field); ok {
		ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
		return model.toPB(fg, field, src, ptr)
	}

	if field.Desc.Kind() == protoreflect.BytesKind && deep {
		return cloneBytesExpr(src)
	}

	if field.Desc.Kind() == protoreflect.MessageKind {
		return src
	}

	return src
}

func (fg *fileGen) model(field *protogen.Field) (typeModel, bool) {
	if field.Desc.Kind() != protoreflect.MessageKind {
		return typeModel{}, false
	}
	model, ok := typeModels[field.Message.Desc.FullName()]
	return model, ok
}

func (fg *fileGen) requiresConversion(field *protogen.Field) bool {
	if isSerializedMessage(field) {
		return true
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		if _, ok := fg.model(field); ok {
			return true
		}
	}
	return false
}

func (fg *fileGen) plainType(field *protogen.Field, ctx fieldContext) string {
	if ctx == ctxField {
		if field.Desc.IsMap() {
			keyType := fg.mapKeyType(field.Message.Fields[0])
			valType := fg.plainType(field.Message.Fields[1], ctxMapValue)
			return "map[" + keyType + "]" + valType
		}
		if field.Desc.IsList() {
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

func (fg *fileGen) plainBaseType(field *protogen.Field) string {
	if isSerializedMessage(field) {
		return "[]byte"
	}
	if model, ok := fg.model(field); ok {
		return model.basePlain(fg)
	}
	if field.Desc.Kind() == protoreflect.EnumKind {
		return fg.out.QualifiedGoIdent(field.Enum.GoIdent)
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return fg.out.QualifiedGoIdent(field.Message.GoIdent)
	}
	return kindToGoType(field.Desc.Kind())
}

func (fg *fileGen) shouldPointer(field *protogen.Field, ctx fieldContext, base string) bool {
	if strings.HasPrefix(base, "*") {
		return false
	}
	if isSerializedMessage(field) {
		return ctx == ctxOneofField
	}
	if ctx == ctxOneofField {
		return true
	}
	if model, ok := fg.model(field); ok {
		return model.pointerField && ctx == ctxField
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return true
	}
	if ctx == ctxField && isFieldNullable(field) {
		return true
	}
	return false
}

func (fg *fileGen) mapKeyType(field *protogen.Field) string {
	return kindToGoType(field.Desc.Kind())
}

func (fg *fileGen) pbValueType(field *protogen.Field) string {
	if field.Desc.Kind() == protoreflect.EnumKind {
		return fg.out.QualifiedGoIdent(field.Enum.GoIdent)
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return "*" + fg.out.QualifiedGoIdent(field.Message.GoIdent)
	}
	return kindToGoType(field.Desc.Kind())
}

func (fg *fileGen) plainMessageName(msg *protogen.Message) string {
	return msg.GoIdent.GoName + "Plain"
}

func (fg *fileGen) pbOneofWrapperName(msg *protogen.Message, field *protogen.Field) string {
	return msg.GoIdent.GoName + "_" + field.GoName
}

func (fg *fileGen) pbMessagePointerType(msg *protogen.Message) string {
	return "*" + fg.out.QualifiedGoIdent(msg.GoIdent)
}

func kindToGoType(kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "[]byte"
	default:
		return ""
	}
}

func isRealOneofField(field *protogen.Field) bool {
	return field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
}
