package generator

import (
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
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
	basePlain    func(*FileGen) string
	pointerField bool
	fromPB       func(*FileGen, *protogen.Field, string, bool) string
	toPB         func(*FileGen, *protogen.Field, string, bool) string
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
			basePlain: func(*FileGen) string {
				return cast.baseType
			},
			pointerField: true,
			fromPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
				if ptr {
					return fg.castIdent(cast.toPtr) + "(" + src + ")"
				}
				return fg.castIdent(cast.toVal) + "(" + src + ")"
			},
			toPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
				if ptr {
					return fg.castIdent(cast.fromPtr) + "(" + src + ")"
				}
				return fg.castIdent(cast.fromVal) + "(" + src + ")"
			},
		}
	}

	m["google.protobuf.Timestamp"] = typeModel{
		basePlain: func(fg *FileGen) string {
			return fg.timeIdent("Time")
		},
		pointerField: true,
		fromPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("TimestampToPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("TimestampToTime") + "(" + src + ")"
		},
		toPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("TimestampFromPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("TimestampFromTime") + "(" + src + ")"
		},
	}
	m["google.protobuf.Duration"] = typeModel{
		basePlain: func(fg *FileGen) string {
			return fg.timeIdent("Duration")
		},
		pointerField: true,
		fromPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("DurationToPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("DurationToTime") + "(" + src + ")"
		},
		toPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("DurationFromPtrTime") + "(" + src + ")"
			}
			return fg.castIdent("DurationFromTime") + "(" + src + ")"
		},
	}
	m["google.protobuf.Struct"] = typeModel{
		basePlain: func(*FileGen) string { return "map[string]any" },
		fromPB: func(fg *FileGen, _ *protogen.Field, src string, _ bool) string {
			return fg.castIdent("StructToMap") + "(" + src + ")"
		},
		toPB: func(fg *FileGen, _ *protogen.Field, src string, _ bool) string {
			return fg.castIdent("StructFromMap") + "(" + src + ")"
		},
	}
	m["google.protobuf.Value"] = serializedModel()
	m["google.protobuf.ListValue"] = serializedModel()
	m["google.protobuf.Any"] = serializedModel()
	m["google.protobuf.Empty"] = typeModel{
		basePlain:    func(*FileGen) string { return "struct{}" },
		pointerField: true,
		fromPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
			if ptr {
				return fg.castIdent("EmptyToPtrStruct") + "(" + src + ")"
			}
			return fg.castIdent("EmptyToStruct") + "(" + src + ")"
		},
		toPB: func(fg *FileGen, _ *protogen.Field, src string, ptr bool) string {
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
		basePlain: func(*FileGen) string { return "[]byte" },
		fromPB: func(fg *FileGen, _ *protogen.Field, src string, _ bool) string {
			return fg.castIdent("MessageToSliceByte") + "(" + src + ")"
		},
		toPB: func(fg *FileGen, field *protogen.Field, src string, ptr bool) string {
			if ptr {
				src = "*" + src
			}
			return fg.castIdent("MessageFromSliceByte") + "[" + fg.pbMessagePointerType(field.Message) + "](" + src + ")"
		},
	}
}

type FileGen struct {
	g    *Generator
	file *protogen.File
	out  *protogen.GeneratedFile

	fileOverrides  map[string]*goplain.OverwriteType
	fieldOverrides map[*protogen.Field]*goplain.OverwriteType

	fileModel       *goplain.FileModel
	virtualFields   map[protoreflect.FullName][]*goplain.VirtualFieldSpec
	virtualMessages []*goplain.VirtualMessage
}

func (g *Generator) NewFileGen(f *protogen.File, model *goplain.FileModel) *FileGen {
	out := g.Plugin.NewGeneratedFile(f.GeneratedFilenamePrefix+".pb.plain.go", f.GoImportPath)
	fg := &FileGen{g: g, file: f, out: out, fileModel: model}
	fg.fileOverrides = newOverrideRegistry(getFileParams(f).GetOverwrite()).byProto
	fg.buildFieldOverrides(f.Messages)
	if model != nil {
		fg.virtualMessages = model.VirtualMessage
		fg.virtualFields = make(map[protoreflect.FullName][]*goplain.VirtualFieldSpec)
		for _, msg := range model.Messages {
			if len(msg.VirtualFields) == 0 {
				continue
			}
			fullName := protoreflect.FullName(msg.FullName)
			fg.virtualFields[fullName] = append([]*goplain.VirtualFieldSpec(nil), msg.VirtualFields...)
		}
	}
	return fg
}

func (fg *FileGen) P(v ...any) {
	fg.out.P(v...)
}

func (fg *FileGen) castIdent(name string) string {
	return fg.out.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
		GoName:       name,
	})
}

func (fg *FileGen) timeIdent(name string) string {
	return fg.out.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: "time",
		GoName:       name,
	})
}

func (fg *FileGen) jsonTag(name string) string {
	return "`json:\"" + strcase.ToSnake(name) + "\"`"
}

func (fg *FileGen) GenFile() {
	if !fg.hasGeneratedMessages(fg.file.Messages) {
		fg.out.Skip()
		return
	}

	fg.P("// Code generated by protoc-gen-go-plain. DO NOT EDIT.\n")
	fg.P("package ", fg.file.GoPackageName)
	fg.P()

	fg.emitVirtualMessages()

	for _, msg := range fg.file.Messages {
		fg.genMessage(msg)
	}
}

func (fg *FileGen) genMessage(msg *protogen.Message) {
	if msg.Desc.IsMapEntry() {
		return
	}
	if shouldGenerateMessage(msg) {
		fg.genPlainStruct(msg)
		fg.genPlainOptions(msg)
		fg.genIntoPlain(msg, false)
		fg.genIntoPlain(msg, true)
		fg.genIntoPb(msg, false)
		fg.genIntoPb(msg, true)
		fg.genIntoMap(msg)
	}

	for _, child := range msg.Messages {
		fg.genMessage(child)
	}
}

func (fg *FileGen) emitVirtualMessages() {
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
			fg.P(fieldName, " ", virtualFieldType(fg.out, field), " ", fg.jsonTag(fieldName))
		}
		fg.P("}")
		fg.P()
	}
}

func (fg *FileGen) hasGeneratedMessages(msgs []*protogen.Message) bool {
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

func (fg *FileGen) genPlainStruct(msg *protogen.Message) {
	plainName := fg.plainMessageName(msg)
	fg.P("type ", plainName, " struct {")

	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			// oneof fields are emitted as standalone nullable fields
			fieldType := fg.plainType(field, ctxOneofField)
			fg.P(field.GoName, " ", fieldType, " ", fg.jsonTag(field.GoName))
			continue
		}

		if isEmbeddedMessage(field) {
			fg.emitEmbeddedFields(field.Message)
			continue
		}

		fieldType := fg.plainType(field, ctxField)
		fg.P(field.GoName, " ", fieldType, " ", fg.jsonTag(field.GoName))
	}

	if extras := fg.virtualFields[msg.Desc.FullName()]; len(extras) > 0 {
		for _, field := range extras {
			fieldName := field.GetName()
			fg.P(fieldName, " ", virtualFieldType(fg.out, field), " ", fg.jsonTag(fieldName))
		}
	}
	fg.P("}")
	fg.P()
}

func (fg *FileGen) genPlainOptions(msg *protogen.Message) {
	extras := fg.virtualFields[msg.Desc.FullName()]
	if len(extras) == 0 {
		return
	}
	plainName := fg.plainMessageName(msg)
	optionName := plainName + "Option"
	fg.P("type ", optionName, " func(*", plainName, ")")
	pbName := msg.GoIdent.GoName
	for _, field := range extras {
		fieldName := field.GetName()
		if fieldName == "" {
			continue
		}
		fg.P("func With", pbName, fieldName, "(v ", virtualFieldType(fg.out, field), ") ", optionName, " {")
		fg.P("return func(out *", plainName, ") { out.", fieldName, " = v }")
		fg.P("}")
	}
	fg.P()
}

func (fg *FileGen) emitEmbeddedFields(msg *protogen.Message) {
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			fieldType := fg.plainType(field, ctxOneofField)
			fg.P(field.GoName, " ", fieldType, " ", fg.jsonTag(field.GoName))
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitEmbeddedFields(field.Message)
			continue
		}
		fieldType := fg.plainType(field, ctxField)
		fg.P(field.GoName, " ", fieldType, " ", fg.jsonTag(field.GoName))
	}
}

func (fg *FileGen) genIntoPlain(msg *protogen.Message, deep bool) {
	plainName := fg.plainMessageName(msg)
	pbName := fg.out.QualifiedGoIdent(msg.GoIdent)
	methodName := "IntoPlain"
	if deep {
		methodName = "IntoPlainDeep"
	}
	extras := fg.virtualFields[msg.Desc.FullName()]
	if len(extras) == 0 {
		fg.P("func (src *", pbName, ") ", methodName, "() *", plainName, " {")
	} else {
		fg.P("func (src *", pbName, ") ", methodName, "(opts ...", plainName, "Option) *", plainName, " {")
	}
	fg.P("if src == nil { return nil }")
	fg.P("out := &", plainName, "{}")

	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitFromPBEmbedded("out", "src", field, deep)
			continue
		}
		fg.emitFromPBField("out", "src", field, deep)
	}

	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue
		}
		fg.emitFromPBOneof("out", "src", msg, oneof, deep)
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

func (fg *FileGen) genIntoPb(msg *protogen.Message, deep bool) {
	plainName := fg.plainMessageName(msg)
	pbName := fg.out.QualifiedGoIdent(msg.GoIdent)
	methodName := "IntoPb"
	if deep {
		methodName = "IntoPbDeep"
	}
	fg.P("func (src *", plainName, ") ", methodName, "() *", pbName, " {")
	fg.P("if src == nil { return nil }")
	fg.P("out := &", pbName, "{}")

	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitToPBEmbedded("out", "src", field, deep)
			continue
		}
		fg.emitToPBField("out", "src", field, deep)
	}

	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue
		}
		fg.emitToPBOneof("out", "src", msg, oneof, deep)
	}

	fg.P("return out")
	fg.P("}")
	fg.P()
}

func (fg *FileGen) genIntoMap(msg *protogen.Message) {
	plainName := fg.plainMessageName(msg)
	fg.P("func (src *", plainName, ") IntoMap() map[string]any {")
	fg.P("return src.intoMap(false, false)")
	fg.P("}")
	fg.P()
	fg.P("func (src *", plainName, ") IntoMapSkipZero() map[string]any {")
	fg.P("return src.intoMap(true, false)")
	fg.P("}")
	fg.P()
	fg.P("func (src *", plainName, ") IntoMapDeep() map[string]any {")
	fg.P("return src.intoMap(false, true)")
	fg.P("}")
	fg.P()
	fg.P("func (src *", plainName, ") IntoMapDeepSkipZero() map[string]any {")
	fg.P("return src.intoMap(true, true)")
	fg.P("}")
	fg.P()
	fg.P("func (src *", plainName, ") intoMap(skipZero, deep bool) map[string]any {")
	fg.P("if src == nil { return nil }")
	fieldCount := fg.countPlainFields(msg)
	if fieldCount > 0 {
		fg.P("out := make(map[string]any, ", fieldCount, ")")
	} else {
		fg.P("out := make(map[string]any)")
	}
	fg.emitIntoMapFields("out", "src", msg.Fields)

	if extras := fg.virtualFields[msg.Desc.FullName()]; len(extras) > 0 {
		for _, field := range extras {
			fieldName := field.GetName()
			if fieldName == "" {
				continue
			}
			fieldType := virtualFieldType(fg.out, field)
			srcExpr := "src." + fieldName
			cond := plainTypeNonZeroExpr(fieldType, srcExpr)
			fg.P("if !skipZero || ", cond, " {")
			fg.P("if deep {")
			fg.P("out[\"", strcase.ToSnake(fieldName), "\"] = ", fg.virtualMapValueExpr(fieldType, srcExpr, true))
			fg.P("} else {")
			fg.P("out[\"", strcase.ToSnake(fieldName), "\"] = ", srcExpr)
			fg.P("}")
			fg.P("}")
		}
	}
	fg.P("return out")
	fg.P("}")
	fg.P()
}

func (fg *FileGen) emitIntoMapFields(outVar, srcVar string, fields []*protogen.Field) {
	for _, field := range fields {
		if isRealOneofField(field) {
			fg.emitIntoMapAssign(outVar, srcVar, field, ctxOneofField)
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitIntoMapEmbedded(outVar, srcVar, field.Message)
			continue
		}
		fg.emitIntoMapAssign(outVar, srcVar, field, ctxField)
	}
}

func (fg *FileGen) emitIntoMapEmbedded(outVar, srcVar string, msg *protogen.Message) {
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			fg.emitIntoMapAssign(outVar, srcVar, field, ctxOneofField)
			continue
		}
		if isEmbeddedMessage(field) {
			fg.emitIntoMapEmbedded(outVar, srcVar, field.Message)
			continue
		}
		fg.emitIntoMapAssign(outVar, srcVar, field, ctxField)
	}
}

func (fg *FileGen) emitIntoMapAssign(outVar, srcVar string, field *protogen.Field, ctx fieldContext) {
	key := strcase.ToSnake(field.GoName)
	srcExpr := srcVar + "." + field.GoName
	cond := fg.plainFieldNonZeroExprWithCtx(field, srcExpr, ctx)
	fg.P("if !skipZero || ", cond, " {")
	fg.P("if !deep {")
	fg.P(outVar, "[\"", key, "\"] = ", srcExpr)
	fg.P("} else {")

	if ctx == ctxField && field.Desc.IsMap() {
		keyField := field.Message.Fields[0]
		valField := field.Message.Fields[1]
		keyType := fg.mapKeyType(keyField)
		valType := fg.plainType(valField, ctxMapValue)
		fg.P("var v map[", keyType, "]", valType)
		fg.P("if ", srcExpr, " != nil {")
		fg.P("v = make(map[", keyType, "]", valType, ", len(", srcExpr, "))")
		fg.P("for k, val := range ", srcExpr, " {")
		if fg.isBytesPlainType(valField) {
			fg.P("v[k] = ", fg.deepCopyBytesExpr(valField, "val", ctxMapValue))
		} else {
			fg.P("v[k] = val")
		}
		fg.P("}")
		fg.P("}")
		fg.P(outVar, "[\"", key, "\"] = v")
	} else if ctx == ctxField && field.Desc.IsList() {
		elemType := fg.plainType(field, ctxListElem)
		fg.P("var v []", elemType)
		fg.P("if ", srcExpr, " != nil {")
		fg.P("v = make([]", elemType, ", 0, len(", srcExpr, "))")
		fg.P("for _, val := range ", srcExpr, " {")
		if fg.isBytesPlainType(field) {
			fg.P("v = append(v, ", fg.deepCopyBytesExpr(field, "val", ctxListElem), ")")
		} else {
			fg.P("v = append(v, val)")
		}
		fg.P("}")
		fg.P("}")
		fg.P(outVar, "[\"", key, "\"] = v")
	} else if fg.isBytesPlainType(field) {
		fg.P(outVar, "[\"", key, "\"] = ", fg.deepCopyBytesExpr(field, srcExpr, ctx))
	} else {
		fg.P(outVar, "[\"", key, "\"] = ", srcExpr)
	}
	fg.P("}")
	fg.P("}")
}

func (fg *FileGen) countPlainFields(msg *protogen.Message) int {
	count := 0
	for _, field := range msg.Fields {
		if isEmbeddedMessage(field) {
			count += fg.countEmbeddedFields(field.Message)
			continue
		}
		count++
	}
	count += len(fg.virtualFields[msg.Desc.FullName()])
	return count
}

func (fg *FileGen) countEmbeddedFields(msg *protogen.Message) int {
	count := 0
	for _, field := range msg.Fields {
		if isEmbeddedMessage(field) {
			count += fg.countEmbeddedFields(field.Message)
			continue
		}
		count++
	}
	return count
}

func (fg *FileGen) emitFromPBEmbedded(outVar, srcVar string, field *protogen.Field, deep bool) {
	fg.P("if ", srcVar, ".", field.GoName, " != nil {")
	fg.emitEmbeddedAssignFrom(srcVar+"."+field.GoName, outVar, field.Message, deep)
	fg.P("}")
}

func (fg *FileGen) emitEmbeddedAssignFrom(srcVar, outVar string, msg *protogen.Message, deep bool) {
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			fg.P(outVar, ".", field.GoName, " = ", fg.pbToPlainExpr(field, srcVar+"."+field.GoName, ctxField, deep))
			continue
		}
		if isEmbeddedMessage(field) {
			fg.P("if ", srcVar, ".", field.GoName, " != nil {")
			fg.emitEmbeddedAssignFrom(srcVar+"."+field.GoName, outVar, field.Message, deep)
			fg.P("}")
			continue
		}
		fg.P(outVar, ".", field.GoName, " = ", fg.pbToPlainExpr(field, srcVar+"."+field.GoName, ctxField, deep))
	}
}

func (fg *FileGen) emitToPBEmbedded(outVar, srcVar string, field *protogen.Field, deep bool) {
	fg.P(outVar, ".", field.GoName, " = &", fg.out.QualifiedGoIdent(field.Message.GoIdent), "{}")
	fg.emitEmbeddedAssignTo(outVar+"."+field.GoName, srcVar, field.Message, deep)
}

func (fg *FileGen) emitEmbeddedAssignTo(tmpVar, srcVar string, msg *protogen.Message, deep bool) {
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			fg.P(tmpVar, ".", field.GoName, " = ", fg.plainToPBExpr(field, srcVar+"."+field.GoName, ctxField, deep))
			continue
		}
		if isEmbeddedMessage(field) {
			if cond := fg.embeddedHasValueExpr(srcVar, field.Message); cond != "" {
				fg.P("if ", cond, " {")
				fg.P("if ", tmpVar, ".", field.GoName, " == nil { ", tmpVar, ".", field.GoName, " = &", fg.out.QualifiedGoIdent(field.Message.GoIdent), "{} }")
				fg.emitEmbeddedAssignTo(tmpVar+"."+field.GoName, srcVar, field.Message, deep)
				fg.P("}")
			}
			continue
		}
		fg.P(tmpVar, ".", field.GoName, " = ", fg.plainToPBExpr(field, srcVar+"."+field.GoName, ctxField, deep))
	}
}

func (fg *FileGen) embeddedHasValueExpr(srcVar string, msg *protogen.Message) string {
	conds := make([]string, 0, len(msg.Fields))
	for _, field := range msg.Fields {
		if isRealOneofField(field) {
			conds = append(conds, srcVar+"."+field.GoName+" != nil")
			continue
		}
		if isEmbeddedMessage(field) {
			if nested := fg.embeddedHasValueExpr(srcVar, field.Message); nested != "" {
				conds = append(conds, nested)
			}
			continue
		}
		if cond := fg.plainFieldNonZeroExpr(field, srcVar+"."+field.GoName); cond != "" {
			conds = append(conds, cond)
		}
	}
	if len(conds) == 0 {
		return ""
	}
	return strings.Join(conds, " || ")
}

func (fg *FileGen) plainFieldNonZeroExpr(field *protogen.Field, src string) string {
	return fg.plainFieldNonZeroExprWithCtx(field, src, ctxField)
}

func (fg *FileGen) plainFieldNonZeroExprWithCtx(field *protogen.Field, src string, ctx fieldContext) string {
	plainType := fg.plainType(field, ctx)
	if isPointerType(plainType) {
		return src + " != nil"
	}
	if strings.HasPrefix(plainType, "[]") {
		return "len(" + src + ") != 0"
	}
	if strings.HasPrefix(plainType, "map[") {
		return "len(" + src + ") != 0"
	}

	switch plainType {
	case "string":
		return src + " != \"\""
	case "bool":
		return src
	case "int", "int32", "int64", "uint", "uint32", "uint64", "float32", "float64":
		return src + " != 0"
	}

	if field.Desc.Kind() == protoreflect.BytesKind {
		return "len(" + src + ") != 0"
	}

	if isComparablePlainType(plainType) {
		return "func() bool { var zero " + plainType + "; return " + src + " != zero }()"
	}
	return "true"
}

func plainTypeNonZeroExpr(plainType, src string) string {
	if strings.HasPrefix(plainType, "*") {
		return src + " != nil"
	}
	if strings.HasPrefix(plainType, "[]") {
		return "len(" + src + ") != 0"
	}
	if strings.HasPrefix(plainType, "map[") {
		return "len(" + src + ") != 0"
	}

	switch plainType {
	case "string":
		return src + " != \"\""
	case "bool":
		return src
	case "int", "int32", "int64", "uint", "uint32", "uint64", "float32", "float64":
		return src + " != 0"
	}

	if isComparablePlainType(plainType) {
		return "func() bool { var zero " + plainType + "; return " + src + " != zero }()"
	}
	return "true"
}

func isPointerType(typeName string) bool {
	return strings.HasPrefix(typeName, "*")
}

func isComparablePlainType(typeName string) bool {
	if typeName == "struct{}" {
		return true
	}
	if strings.HasPrefix(typeName, "[]") || strings.HasPrefix(typeName, "map[") {
		return false
	}
	if strings.HasPrefix(typeName, "func(") || strings.HasPrefix(typeName, "interface{") {
		return false
	}
	if strings.HasPrefix(typeName, "chan ") {
		return false
	}
	if strings.HasPrefix(typeName, "struct{") {
		return false
	}
	return true
}

func cloneBytesExpr(src string) string {
	return "append([]byte(nil), " + src + "...)"
}

func (fg *FileGen) isBytesPlainType(field *protogen.Field) bool {
	return fg.plainBaseType(field) == "[]byte"
}

func (fg *FileGen) deepCopyBytesExpr(field *protogen.Field, src string, ctx fieldContext) string {
	if isPointerType(fg.plainType(field, ctx)) {
		return "func() *[]byte { if " + src + " == nil { return nil }; v := " + cloneBytesExpr("(*"+src+")") + "; return &v }()"
	}
	return cloneBytesExpr(src)
}

func (fg *FileGen) virtualMapValueExpr(typeName, src string, deep bool) string {
	if !deep {
		return src
	}
	if typeName == "[]byte" {
		return cloneBytesExpr(src)
	}
	if typeName == "*[]byte" {
		return "func() *[]byte { if " + src + " == nil { return nil }; v := " + cloneBytesExpr("(*"+src+")") + "; return &v }()"
	}
	return src
}

func (fg *FileGen) aliasValueField(msg *protogen.Message) *protogen.Field {
	if msg == nil || !isTypeAliasMessage(msg) {
		return nil
	}
	if len(msg.Fields) != 1 {
		panic("type_alias message " + string(msg.Desc.FullName()) + " must have exactly one field named value")
	}
	field := msg.Fields[0]
	if field.Desc.Name() != "value" {
		panic("type_alias message " + string(msg.Desc.FullName()) + " must have a single field named value")
	}
	if field.Desc.IsList() || field.Desc.IsMap() || isRealOneofField(field) {
		panic("type_alias message " + string(msg.Desc.FullName()) + " value field must be a singular non-oneof field")
	}
	return field
}

func (fg *FileGen) aliasValueFieldFromField(field *protogen.Field) *protogen.Field {
	if field == nil || field.Desc.Kind() != protoreflect.MessageKind {
		return nil
	}
	return fg.aliasValueField(field.Message)
}

func (fg *FileGen) aliasPBToPlainExpr(field, aliasField *protogen.Field, src string, ctx fieldContext, deep bool) string {
	plainType := fg.plainType(field, ctx)
	valExpr := fg.pbToPlainExpr(aliasField, src+".Value", ctx, deep)
	retExpr := "val"
	if isPointerType(plainType) && !fg.pbToPlainExprReturnsPointer(aliasField, ctx) {
		retExpr = "&val"
	}
	return "func() " + plainType + " { if " + src + " == nil { var zero " + plainType + "; return zero }; val := " + valExpr + "; return " + retExpr + " }()"
}

func (fg *FileGen) aliasPlainToPBExpr(field, aliasField *protogen.Field, src string, ctx fieldContext, deep bool) string {
	plainType := fg.plainType(field, ctx)
	valSrc := src
	if isPointerType(plainType) && !fg.plainToPBExprWantsPointer(aliasField, ctx) {
		valSrc = "*" + src
	}
	valExpr := fg.plainToPBExpr(aliasField, valSrc, ctx, deep)
	msgType := fg.out.QualifiedGoIdent(field.Message.GoIdent)
	if isPointerType(plainType) {
		return "func() *" + msgType + " { if " + src + " == nil { return nil }; return &" + msgType + "{Value: " + valExpr + "} }()"
	}
	return "&" + msgType + "{Value: " + valExpr + "}"
}

func (fg *FileGen) pbToPlainExprReturnsPointer(field *protogen.Field, ctx fieldContext) bool {
	if fg.overrideForField(field) != nil {
		if model, ok := fg.model(field); ok {
			return fg.shouldPointer(field, ctx, model.basePlain(fg))
		}
	}
	if isSerializedMessage(field) {
		return false
	}
	if model, ok := fg.model(field); ok {
		return fg.shouldPointer(field, ctx, model.basePlain(fg))
	}
	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return isPointerType(fg.plainType(field, ctx))
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return true
	}
	return false
}

func (fg *FileGen) plainToPBExprWantsPointer(field *protogen.Field, ctx fieldContext) bool {
	if fg.overrideForField(field) != nil {
		if model, ok := fg.model(field); ok {
			return fg.shouldPointer(field, ctx, model.basePlain(fg))
		}
	}
	if isSerializedMessage(field) {
		return isPointerType(fg.plainType(field, ctx))
	}
	if model, ok := fg.model(field); ok {
		return fg.shouldPointer(field, ctx, model.basePlain(fg))
	}
	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return isPointerType(fg.plainType(field, ctx))
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return true
	}
	return false
}

func (fg *FileGen) emitFromPBField(outVar, srcVar string, field *protogen.Field, deep bool) {
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

func (fg *FileGen) emitToPBField(outVar, srcVar string, field *protogen.Field, deep bool) {
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

func (fg *FileGen) emitFromPBOneof(outVar, srcVar string, msg *protogen.Message, oneof *protogen.Oneof, deep bool) {
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

func (fg *FileGen) emitToPBOneof(outVar, srcVar string, msg *protogen.Message, oneof *protogen.Oneof, deep bool) {
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

func (fg *FileGen) oneofPlainToPBExpr(field *protogen.Field, src string, deep bool) string {
	if field.Desc.Kind() == protoreflect.MessageKind {
		return fg.plainToPBExpr(field, src, ctxOneofField, deep)
	}
	return fg.plainToPBExpr(field, "*"+src, ctxOneofField, deep)
}

func (fg *FileGen) pbToPlainExpr(field *protogen.Field, src string, ctx fieldContext, deep bool) string {
	if fg.overrideForField(field) != nil {
		if model, ok := fg.model(field); ok {
			ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
			if ctx == ctxOneofField && field.Desc.Kind() != protoreflect.MessageKind {
				ptr = false
			}
			return model.fromPB(fg, field, src, ptr)
		}
	}

	if isSerializedMessage(field) {
		return fg.castIdent("MessageToSliceByte") + "(" + src + ")"
	}

	if model, ok := fg.model(field); ok {
		ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
		if ctx == ctxOneofField && field.Desc.Kind() != protoreflect.MessageKind {
			ptr = false
		}
		return model.fromPB(fg, field, src, ptr)
	}

	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return fg.aliasPBToPlainExpr(field, aliasField, src, ctx, deep)
	}

	if field.Desc.Kind() == protoreflect.BytesKind && deep {
		return cloneBytesExpr(src)
	}

	if field.Desc.Kind() == protoreflect.MessageKind {
		return src
	}

	return src
}

func (fg *FileGen) plainToPBExpr(field *protogen.Field, src string, ctx fieldContext, deep bool) string {
	if fg.overrideForField(field) != nil {
		if model, ok := fg.model(field); ok {
			ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
			if ctx == ctxOneofField && field.Desc.Kind() != protoreflect.MessageKind {
				ptr = false
			}
			return model.toPB(fg, field, src, ptr)
		}
	}

	if isSerializedMessage(field) {
		if isPointerType(fg.plainType(field, ctx)) {
			src = "*" + src
		}
		return fg.castIdent("MessageFromSliceByte") + "[" + fg.pbMessagePointerType(field.Message) + "](" + src + ")"
	}

	if model, ok := fg.model(field); ok {
		ptr := fg.shouldPointer(field, ctx, model.basePlain(fg))
		if ctx == ctxOneofField && field.Desc.Kind() != protoreflect.MessageKind {
			ptr = false
		}
		return model.toPB(fg, field, src, ptr)
	}

	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return fg.aliasPlainToPBExpr(field, aliasField, src, ctx, deep)
	}

	if field.Desc.Kind() == protoreflect.BytesKind && deep {
		return cloneBytesExpr(src)
	}

	if field.Desc.Kind() == protoreflect.MessageKind {
		return src
	}

	return src
}

func (fg *FileGen) model(field *protogen.Field) (typeModel, bool) {
	if ov := fg.overrideForField(field); ov != nil {
		base := fg.overrideBaseType(ov)
		model := typeModel{
			basePlain: func(*FileGen) string {
				return base
			},
			pointerField: ov.GetPointer(),
			fromPB: func(fg *FileGen, field *protogen.Field, src string, ptr bool) string {
				return fg.overrideFromPBExpr(field, src, ptr, ov)
			},
			toPB: func(fg *FileGen, field *protogen.Field, src string, ptr bool) string {
				return fg.overrideToPBExpr(field, src, ptr, ov)
			},
		}
		return model, true
	}
	if field.Desc.Kind() != protoreflect.MessageKind {
		return typeModel{}, false
	}
	model, ok := typeModels[field.Message.Desc.FullName()]
	return model, ok
}

func (fg *FileGen) requiresConversion(field *protogen.Field) bool {
	if fg.overrideForField(field) != nil {
		return true
	}
	if isSerializedMessage(field) {
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

func (fg *FileGen) plainType(field *protogen.Field, ctx fieldContext) string {
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

func (fg *FileGen) plainBaseType(field *protogen.Field) string {
	if ov := fg.overrideForField(field); ov != nil {
		return fg.overrideBaseType(ov)
	}
	if isSerializedMessage(field) {
		return "[]byte"
	}
	if model, ok := fg.model(field); ok {
		return model.basePlain(fg)
	}
	if aliasField := fg.aliasValueFieldFromField(field); aliasField != nil {
		return fg.plainBaseType(aliasField)
	}
	if field.Desc.Kind() == protoreflect.EnumKind {
		return fg.out.QualifiedGoIdent(field.Enum.GoIdent)
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return fg.out.QualifiedGoIdent(field.Message.GoIdent)
	}
	return kindToGoType(field.Desc.Kind())
}

func (fg *FileGen) shouldPointer(field *protogen.Field, ctx fieldContext, base string) bool {
	if strings.HasPrefix(base, "*") {
		return false
	}
	if ov := fg.overrideForField(field); ov != nil {
		if ctx == ctxOneofField {
			return true
		}
		return ov.GetPointer() && ctx == ctxField
	}
	if isSerializedMessage(field) {
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
	if field.Desc.Kind() == protoreflect.MessageKind {
		return true
	}
	if ctx == ctxField && isFieldNullable(field) {
		return true
	}
	return false
}

func (fg *FileGen) mapKeyType(field *protogen.Field) string {
	return kindToGoType(field.Desc.Kind())
}

func (fg *FileGen) pbValueType(field *protogen.Field) string {
	if field.Desc.Kind() == protoreflect.EnumKind {
		return fg.out.QualifiedGoIdent(field.Enum.GoIdent)
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return "*" + fg.out.QualifiedGoIdent(field.Message.GoIdent)
	}
	return kindToGoType(field.Desc.Kind())
}

func (fg *FileGen) plainMessageName(msg *protogen.Message) string {
	suffix := "Plain"
	if fg.g != nil && fg.g.Settings != nil && fg.g.Settings.PlainSuffix != "" {
		suffix = fg.g.Settings.PlainSuffix
	}
	return msg.GoIdent.GoName + suffix
}

func (fg *FileGen) pbOneofWrapperName(msg *protogen.Message, field *protogen.Field) string {
	return msg.GoIdent.GoName + "_" + field.GoName
}

func (fg *FileGen) pbMessagePointerType(msg *protogen.Message) string {
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
