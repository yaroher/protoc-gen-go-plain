package generator

import (
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type overrideRegistry struct {
	byProto map[string]*goplain.OverwriteType
}

func newOverrideRegistry(overrides []*goplain.OverwriteType) *overrideRegistry {
	reg := &overrideRegistry{byProto: make(map[string]*goplain.OverwriteType)}
	for _, ov := range overrides {
		if ov == nil {
			continue
		}
		protoType := normalizeProtoType(ov.GetProtoType())
		if protoType == "" {
			panic("override proto_type is required for file/global overrides")
		}
		reg.byProto[protoType] = ov
	}
	return reg
}

func normalizeProtoType(t string) string {
	return strings.TrimPrefix(t, ".")
}

func protoTypeKey(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.MessageKind:
		return string(field.Message.Desc.FullName())
	case protoreflect.EnumKind:
		return string(field.Enum.Desc.FullName())
	default:
		return field.Desc.Kind().String()
	}
}

func (fg *fileGen) buildFieldOverrides(msgs []*protogen.Message) {
	for _, msg := range msgs {
		if msg.Desc.IsMapEntry() {
			continue
		}
		for _, field := range msg.Fields {
			if ov := getFieldOverwrite(field); ov != nil {
				if fg.fieldOverrides == nil {
					fg.fieldOverrides = make(map[*protogen.Field]*goplain.OverwriteType)
				}
				fg.fieldOverrides[field] = ov
				if field.Desc.IsMap() && len(field.Message.Fields) > 1 {
					fg.fieldOverrides[field.Message.Fields[1]] = ov
				}
			}
		}
		fg.buildFieldOverrides(msg.Messages)
	}
}

func (fg *fileGen) fieldOverride(field *protogen.Field) *goplain.OverwriteType {
	if fg.fieldOverrides == nil {
		return nil
	}
	return fg.fieldOverrides[field]
}

func (fg *fileGen) fileOverride(field *protogen.Field) *goplain.OverwriteType {
	if fg.fileOverrides == nil {
		return nil
	}
	return fg.fileOverrides[protoTypeKey(field)]
}

func (fg *fileGen) globalOverride(field *protogen.Field) *goplain.OverwriteType {
	if fg.g == nil || fg.g.overrides == nil {
		return nil
	}
	return fg.g.overrides.byProto[protoTypeKey(field)]
}

func (fg *fileGen) overrideForField(field *protogen.Field) *goplain.OverwriteType {
	if ov := fg.fieldOverride(field); ov != nil {
		return ov
	}
	if ov := fg.fileOverride(field); ov != nil {
		return ov
	}
	return fg.globalOverride(field)
}

func (fg *fileGen) overrideBaseType(ov *goplain.OverwriteType) string {
	if ov == nil || ov.GoType == nil || ov.GoType.GetName() == "" {
		panic("overwrite go_type is required")
	}
	if strings.HasPrefix(ov.GoType.GetName(), "*") {
		panic("overwrite go_type must be a base type (without pointer)")
	}
	return fg.qualifyGoIdent(ov.GoType)
}

func (fg *fileGen) qualifyGoIdent(id *goplain.GoIdent) string {
	if id == nil || id.GetName() == "" {
		return ""
	}
	name := id.GetName()
	if id.GetImportPath() == "" {
		if strings.Contains(name, ".") {
			panic("overwrite GoIdent name must be unqualified when import_path is empty")
		}
		return name
	}
	if strings.Contains(name, ".") {
		panic("overwrite GoIdent name must be unqualified when import_path is set")
	}
	return fg.out.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: protogen.GoImportPath(id.GetImportPath()),
		GoName:       name,
	})
}

func (fg *fileGen) overrideFuncIdent(id *goplain.GoIdent) string {
	if id == nil || id.GetName() == "" {
		return ""
	}
	return fg.qualifyGoIdent(id)
}

func (fg *fileGen) pbType(field *protogen.Field) string {
	if field.Desc.Kind() == protoreflect.EnumKind {
		return fg.out.QualifiedGoIdent(field.Enum.GoIdent)
	}
	if field.Desc.Kind() == protoreflect.MessageKind {
		return "*" + fg.out.QualifiedGoIdent(field.Message.GoIdent)
	}
	base := kindToGoType(field.Desc.Kind())
	if isFieldNullable(field) {
		return "*" + base
	}
	return base
}

func (fg *fileGen) overrideFromPBExpr(field *protogen.Field, src string, ptr bool, ov *goplain.OverwriteType) string {
	expr := fg.overrideToPlainCall(field, src, ov)
	if !ptr {
		return expr
	}
	base := fg.overrideBaseType(ov)
	nilable := field.Desc.Kind() == protoreflect.MessageKind || isFieldNullable(field)
	if nilable {
		return "func() *" + base + " { if " + src + " == nil { return nil }; val := " + expr + "; return &val }()"
	}
	return "func() *" + base + " { val := " + expr + "; return &val }()"
}

func (fg *fileGen) overrideToPBExpr(field *protogen.Field, src string, ptr bool, ov *goplain.OverwriteType) string {
	if !ptr {
		return fg.overrideToPBCall(field, src, ov)
	}
	pbType := fg.pbType(field)
	if strings.HasPrefix(pbType, "*") {
		return "func() " + pbType + " { if " + src + " == nil { return nil }; return " + fg.overrideToPBCall(field, "*"+src, ov) + " }()"
	}
	return "func() " + pbType + " { if " + src + " == nil { var zero " + pbType + "; return zero }; return " + fg.overrideToPBCall(field, "*"+src, ov) + " }()"
}

func (fg *fileGen) overrideToPlainCall(field *protogen.Field, src string, ov *goplain.OverwriteType) string {
	body := strings.TrimSpace(ov.GetToPlainBody())
	if body != "" {
		pbType := fg.pbType(field)
		base := fg.overrideBaseType(ov)
		return "func(v " + pbType + ") " + base + " { " + body + " }(" + src + ")"
	}
	fn := fg.overrideFuncIdent(ov.GetToPlain())
	if fn == "" {
		panic("overwrite to_plain or to_plain_body is required")
	}
	return fn + "(" + src + ")"
}

func (fg *fileGen) overrideToPBCall(field *protogen.Field, src string, ov *goplain.OverwriteType) string {
	body := strings.TrimSpace(ov.GetToPbBody())
	if body != "" {
		base := fg.overrideBaseType(ov)
		pbType := fg.pbType(field)
		return "func(v " + base + ") " + pbType + " { " + body + " }(" + src + ")"
	}
	fn := fg.overrideFuncIdent(ov.GetToPb())
	if fn == "" {
		panic("overwrite to_pb or to_pb_body is required")
	}
	return fn + "(" + src + ")"
}
