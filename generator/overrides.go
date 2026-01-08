package generator

import (
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/ir"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type overrideRegistry struct {
	rules []*goplain.OverwriteType
}

func newOverrideRegistry(overrides []*goplain.OverwriteType) *overrideRegistry {
	reg := &overrideRegistry{}
	for _, ov := range overrides {
		if ov == nil {
			continue
		}
		reg.rules = append(reg.rules, ov)
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

func (fg *fileGen) overrideForField(field *ir.Field) *goplain.OverwriteType {
	if field == nil {
		return nil
	}
	return field.Override
}

func (reg *overrideRegistry) matchField(protoType string, state selectorState) *goplain.OverwriteType {
	if reg == nil || len(reg.rules) == 0 {
		return nil
	}
	for _, ov := range reg.rules {
		if ov == nil {
			continue
		}
		if selectorMatches(ov.GetSelector(), protoType, state) {
			return ov
		}
	}
	return nil
}

func selectorMatches(selectors []*goplain.OverrideSelector, protoType string, state selectorState) bool {
	if len(selectors) == 0 {
		return false
	}
	protoType = normalizeProtoType(protoType)
	for _, sel := range selectors {
		if sel == nil {
			continue
		}
		if sel.GetProtoType() != "" && normalizeProtoType(sel.GetProtoType()) != protoType {
			continue
		}
		if sel.IsList != nil && state.isList != sel.GetIsList() {
			continue
		}
		if sel.IsMap != nil && state.isMap != sel.GetIsMap() {
			continue
		}
		if sel.IsOptional != nil && state.isOptional != sel.GetIsOptional() {
			continue
		}
		if sel.IsOneof != nil && state.isOneof != sel.GetIsOneof() {
			continue
		}
		if sel.IsEmbedded != nil && state.isEmbedded != sel.GetIsEmbedded() {
			continue
		}
		if sel.IsSerialized != nil && state.isSerialized != sel.GetIsSerialized() {
			continue
		}
		if sel.IsVirtual != nil && state.isVirtual != sel.GetIsVirtual() {
			continue
		}
		return true
	}
	return false
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

func (fg *fileGen) pbType(field *ir.Field) string {
	if field.Kind == ir.KindEnum && field.EnumType != nil {
		return fg.qualifiedGoIdent(field.EnumType.GoIdent)
	}
	if field.Kind == ir.KindMessage && field.MessageType != nil {
		return "*" + fg.qualifiedGoIdent(field.MessageType.GoIdent)
	}
	base := kindToGoType(field.Kind)
	if isIRFieldNullable(field) {
		return "*" + base
	}
	return base
}

func (fg *fileGen) overrideFromPBExpr(field *ir.Field, src string, ptr bool, ov *goplain.OverwriteType) string {
	expr := fg.overrideToPlainCall(field, src, ov)
	if !ptr {
		return expr
	}
	base := fg.overrideBaseType(ov)
	nilable := field.Kind == ir.KindMessage || isIRFieldNullable(field)
	if nilable {
		return "func() *" + base + " { if " + src + " == nil { return nil }; val := " + expr + "; return &val }()"
	}
	return "func() *" + base + " { val := " + expr + "; return &val }()"
}

func (fg *fileGen) overrideToPBExpr(field *ir.Field, src string, ptr bool, ov *goplain.OverwriteType) string {
	if !ptr {
		return fg.overrideToPBCall(field, src, ov)
	}
	pbType := fg.pbType(field)
	if strings.HasPrefix(pbType, "*") {
		return "func() " + pbType + " { if " + src + " == nil { return nil }; return " + fg.overrideToPBCall(field, "*"+src, ov) + " }()"
	}
	return "func() " + pbType + " { if " + src + " == nil { var zero " + pbType + "; return zero }; return " + fg.overrideToPBCall(field, "*"+src, ov) + " }()"
}

func (fg *fileGen) overrideToPlainCall(field *ir.Field, src string, ov *goplain.OverwriteType) string {
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

func (fg *fileGen) overrideToPBCall(field *ir.Field, src string, ov *goplain.OverwriteType) string {
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
