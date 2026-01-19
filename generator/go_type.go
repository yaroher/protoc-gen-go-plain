package generator

import (
	"fmt"
	"go/token"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// GoSanitized converts a string to a valid Go identifier.
func goSanitized(s string) string {
	// Sanitize the input to the set of valid characters,
	// which must be '_' or be in the Unicode L or N categories.
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, s)

	// Prepend '_' in the event of a Go keyword conflict or if
	// the identifier is invalid (does not start in the Unicode L category).
	r, _ := utf8.DecodeRuneInString(s)
	if token.Lookup(s).IsKeyword() || !unicode.IsLetter(r) {
		return "_" + s
	}
	return s
}

func cleanPackageName(name string) protogen.GoPackageName {
	return protogen.GoPackageName(goSanitized(name))
}

func qualifiedGoIdent(ident protogen.GoIdent) string {
	if ident.GoImportPath == "" {
		return ident.GoName
	}
	packageName := cleanPackageName(path.Base(string(ident.GoImportPath)))
	return string(packageName) + "." + ident.GoName
}

func getFieldGoType(field *protogen.Field) string {
	goType := ""
	isScalar := true

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		goType = "bool"
	case protoreflect.EnumKind:
		goType = qualifiedGoIdent(field.Enum.GoIdent)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		goType = "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		goType = "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		goType = "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		goType = "uint64"
	case protoreflect.FloatKind:
		goType = "float32"
	case protoreflect.DoubleKind:
		goType = "float64"
	case protoreflect.StringKind:
		goType = "string"
	case protoreflect.BytesKind:
		goType = "[]byte"
		isScalar = false
	case protoreflect.MessageKind, protoreflect.GroupKind:
		goType = "*" + qualifiedGoIdent(field.Message.GoIdent)
		isScalar = false
	}

	switch {
	case field.Desc.IsList():
		return "[]" + goType
	case field.Desc.IsMap():
		keyType := getFieldGoType(field.Message.Fields[0])
		valType := getFieldGoType(field.Message.Fields[1])
		return fmt.Sprintf("map[%v]%v", keyType, valType)
	}

	// Для optional полей скалярные типы становятся указателями
	if isScalar && field.Desc.HasPresence() && !field.Desc.IsMap() && !field.Desc.IsList() {
		return "*" + goType
	}

	return goType
}

// findTypeAliasOverride ищет override из type alias в оригинальном Plugin
// getFieldGoTypeWithFile использует GeneratedFile для правильной работы с импортами
// typeAliasOverrides - мапа overrides из type alias полей (собранная ДО конверсии)
func getFieldGoTypeWithFile(g *protogen.GeneratedFile, field *protogen.Field, typeAliasOverrides map[string]*goplain.GoIdent) string {
	// 1. Сначала проверяем прямые опции у поля
	fieldOpts := field.Desc.Options().(*descriptorpb.FieldOptions)
	if proto.HasExtension(fieldOpts, goplain.E_Field) {
		fieldExtOpts := proto.GetExtension(fieldOpts, goplain.E_Field).(*goplain.FieldOptions)
		if overrideType := fieldExtOpts.GetOverrideType(); overrideType != nil {
			goIdent := protogen.GoIdent{
				GoName:       overrideType.GetName(),
				GoImportPath: protogen.GoImportPath(overrideType.GetImportPath()),
			}
			return g.QualifiedGoIdent(goIdent)
		}
	}

	// 2. Проверяем, был ли оригинальный тип поля type alias с override
	// Преобразуем имя поля обратно к оригинальному (убираем Plain)
	plainFieldName := string(field.Desc.FullName())
	origFieldName := strings.Replace(plainFieldName, "Plain.", ".", 1)

	if origOverride, found := typeAliasOverrides[origFieldName]; found {
		goIdent := protogen.GoIdent{
			GoName:       origOverride.GetName(),
			GoImportPath: protogen.GoImportPath(origOverride.GetImportPath()),
		}
		return g.QualifiedGoIdent(goIdent)
	}

	goType := ""
	isScalar := true

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		goType = "bool"
	case protoreflect.EnumKind:
		goType = g.QualifiedGoIdent(field.Enum.GoIdent)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		goType = "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		goType = "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		goType = "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		goType = "uint64"
	case protoreflect.FloatKind:
		goType = "float32"
	case protoreflect.DoubleKind:
		goType = "float64"
	case protoreflect.StringKind:
		goType = "string"
	case protoreflect.BytesKind:
		goType = "[]byte"
		isScalar = false
	case protoreflect.MessageKind, protoreflect.GroupKind:
		goType = "*" + g.QualifiedGoIdent(field.Message.GoIdent)
		isScalar = false
	}

	switch {
	case field.Desc.IsList():
		return "[]" + goType
	case field.Desc.IsMap():
		// Для map полей рекурсивно обрабатываем ключ и значение
		keyType := getFieldGoTypeWithFile(g, field.Message.Fields[0], typeAliasOverrides)
		valType := getFieldGoTypeWithFile(g, field.Message.Fields[1], typeAliasOverrides)
		return fmt.Sprintf("map[%v]%v", keyType, valType)
	}

	// Для optional полей скалярные типы становятся указателями
	if isScalar && field.Desc.HasPresence() && !field.Desc.IsMap() && !field.Desc.IsList() {
		return "*" + goType
	}

	return goType
}
