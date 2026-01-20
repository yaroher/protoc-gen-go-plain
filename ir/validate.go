package ir

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/typepb"
)

// Validation rules summary (derived from goplain/goplain.proto):
// - MessageOptions.generate toggles transformation for that message.
// - MessageOptions.type_alias requires exactly one field named "value" with scalar (non-message/enum) type and non-repeated.
// - MessageOptions.virtual_fields entries must have name, kind; number (if set) must be >0 and unique within message; for MESSAGE/ENUM kind, type_url is required.
// - FieldOptions.override_type requires name.
// - FieldOptions.serialize is allowed on any field; serialize conflicts with embed.
// - FieldOptions.embed/embed_with_prefix require MESSAGE type; embed_with_prefix implies embed.
// - FieldOptions.with_enums is required for fields inside oneof.
// - OneofOptions.embed/embed_with_prefix flatten oneof fields; embed_with_prefix implies embed.
// - OneofOptions.enum_dispatched/enum_dispatched_with_prefix generate enum dispatch field; enum_dispatched_with_prefix implies enum_dispatched.

func validateFileOptions(file *descriptorpb.FileDescriptorProto, diags *[]Diagnostic) {
	if file.GetOptions() == nil {
		return
	}
	ext := proto.GetExtension(file.GetOptions(), goplain.E_File)
	if ext == nil {
		return
	}
	opts, ok := ext.(*goplain.FileOptions)
	if !ok || opts == nil {
		return
	}
	for i, ov := range opts.GetGoTypesOverrides() {
		if ov == nil {
			continue
		}
		selector := ov.GetSelector()
		if selector == nil {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "type override selector is required", Subject: fmt.Sprintf("%s:go_types_overrides[%d]", file.GetName(), i)})
			continue
		}
		if selector.GetTargetFullPath() == "" && selector.FieldKind == nil && selector.FieldCardinality == nil && selector.GetFieldTypeUrl() == "" {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "type override selector must set at least one filter", Subject: fmt.Sprintf("%s:go_types_overrides[%d]", file.GetName(), i)})
		}
		if ov.GetTargetGoType() == nil || ov.GetTargetGoType().GetName() == "" {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "type override target_go_type.name is required", Subject: fmt.Sprintf("%s:go_types_overrides[%d]", file.GetName(), i)})
		}
	}
}

func validateMessageOptions(msg *descriptorpb.DescriptorProto, fullName string, diags *[]Diagnostic) {
	if msg.GetOptions() == nil {
		return
	}
	ext := proto.GetExtension(msg.GetOptions(), goplain.E_Message)
	if ext == nil {
		return
	}
	opts, ok := ext.(*goplain.MessageOptions)
	if !ok || opts == nil {
		return
	}
	if opts.GetTypeAlias() {
		if msg.GetOptions().GetMapEntry() {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "type_alias not allowed on map entry", Subject: fullName})
		} else if len(msg.GetField()) != 1 || msg.GetField()[0].GetName() != "value" {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "type_alias requires exactly one field named 'value'", Subject: fullName})
		} else {
			f := msg.GetField()[0]
			if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
				*diags = append(*diags, Diagnostic{Level: DiagError, Message: "type_alias field 'value' must not be repeated", Subject: fullName})
			}
			if f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE || f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM || f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP {
				*diags = append(*diags, Diagnostic{Level: DiagError, Message: "type_alias field 'value' must be scalar", Subject: fullName})
			}
		}
	}

	if len(opts.GetVirtualFields()) > 0 {
		seen := make(map[int32]struct{})
		for _, f := range msg.GetField() {
			if f.GetNumber() > 0 {
				seen[f.GetNumber()] = struct{}{}
			}
		}
		for i, vf := range opts.GetVirtualFields() {
			if vf == nil {
				continue
			}
			subject := fmt.Sprintf("%s:virtual_fields[%d]", fullName, i)
			if strings.TrimSpace(vf.GetName()) == "" {
				*diags = append(*diags, Diagnostic{Level: DiagError, Message: "virtual field name is required", Subject: subject})
			}
			if vf.GetKind() == typepb.Field_TYPE_UNKNOWN {
				*diags = append(*diags, Diagnostic{Level: DiagError, Message: "virtual field kind is required", Subject: subject})
			}
			if vf.GetNumber() < 0 {
				*diags = append(*diags, Diagnostic{Level: DiagError, Message: "virtual field number must be positive", Subject: subject})
			}
			if vf.GetNumber() > 0 {
				if _, ok := seen[vf.GetNumber()]; ok {
					*diags = append(*diags, Diagnostic{Level: DiagError, Message: "virtual field number conflicts with existing field", Subject: subject})
				} else {
					seen[vf.GetNumber()] = struct{}{}
				}
			}
			if (vf.GetKind() == typepb.Field_TYPE_MESSAGE || vf.GetKind() == typepb.Field_TYPE_ENUM) && strings.TrimSpace(vf.GetTypeUrl()) == "" {
				*diags = append(*diags, Diagnostic{Level: DiagError, Message: "virtual field type_url required for message/enum", Subject: subject})
			}
		}
	}
}

func validateFieldOptions(field *descriptorpb.FieldDescriptorProto, fullName string, diags *[]Diagnostic) {
	if field.GetOptions() == nil {
		return
	}
	ext := proto.GetExtension(field.GetOptions(), goplain.E_Field)
	if ext == nil {
		return
	}
	opts, ok := ext.(*goplain.FieldOptions)
	if !ok || opts == nil {
		return
	}
	subject := fmt.Sprintf("%s.%s", fullName, field.GetName())

	if opts.GetOverrideType() != nil && strings.TrimSpace(opts.GetOverrideType().GetName()) == "" {
		*diags = append(*diags, Diagnostic{Level: DiagError, Message: "override_type.name is required", Subject: subject})
	}
	if (opts.GetEmbed() || opts.GetEmbedWithPrefix()) && field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		*diags = append(*diags, Diagnostic{Level: DiagError, Message: "embed requires message field", Subject: subject})
	}
	if (opts.GetEmbed() || opts.GetEmbedWithPrefix()) && opts.GetSerialize() {
		*diags = append(*diags, Diagnostic{Level: DiagError, Message: "serialize and embed are mutually exclusive", Subject: subject})
	}
	if opts.GetEnumAsString() && opts.GetEnumAsInt() {
		*diags = append(*diags, Diagnostic{Level: DiagError, Message: "enum_as_string and enum_as_int are mutually exclusive", Subject: subject})
	}
	if (opts.GetEnumAsString() || opts.GetEnumAsInt()) && field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		*diags = append(*diags, Diagnostic{Level: DiagError, Message: "enum_as_* requires enum field", Subject: subject})
	}
}

func validateOneofOptions(oneof *descriptorpb.OneofDescriptorProto, fullName string, fields []*descriptorpb.FieldDescriptorProto, allFields []*descriptorpb.FieldDescriptorProto, diags *[]Diagnostic) {
	var opts *goplain.OneofOptions
	if oneof.GetOptions() != nil {
		ext := proto.GetExtension(oneof.GetOptions(), goplain.E_Oneof)
		if ext != nil {
			if v, ok := ext.(*goplain.OneofOptions); ok {
				opts = v
			}
		}
	}

	enumDispatch := false
	embed := false
	if opts != nil {
		enumDispatch = opts.GetEnumDispatched() || opts.GetEnumDispatchedWithPrefix()
		embed = opts.GetEmbed() || opts.GetEmbedWithPrefix()
	}
	enumFull := ""
	hasAnyEnums := false
	for _, f := range fields {
		if f == nil {
			continue
		}
		if f.GetOptions() == nil {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "with_enums required for oneof field", Subject: fmt.Sprintf("%s.%s", fullName, f.GetName())})
			continue
		}
		fopts := proto.GetExtension(f.GetOptions(), goplain.E_Field)
		fieldOpts, _ := fopts.(*goplain.FieldOptions)
		if fieldOpts == nil || len(fieldOpts.GetWithEnums()) == 0 {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "with_enums required for oneof field", Subject: fmt.Sprintf("%s.%s", fullName, f.GetName())})
			continue
		}
		for _, v := range fieldOpts.GetWithEnums() {
			ef, _ := parseEnumValueFullName(v)
			if ef == "" {
				*diags = append(*diags, Diagnostic{Level: DiagError, Message: "with_enums must be full enum value name", Subject: fmt.Sprintf("%s.%s", fullName, f.GetName())})
			}
			if enumFull == "" && ef != "" {
				enumFull = ef
			}
			hasAnyEnums = true
		}
	}
	if !hasAnyEnums {
		return
	}
	for _, f := range fields {
		fopts := getFieldOptions(f)
		if fopts == nil || len(fopts.GetWithEnums()) == 0 {
			*diags = append(*diags, Diagnostic{Level: DiagError, Message: "with_enums required for oneof field when using enum discriminator", Subject: fmt.Sprintf("%s.%s", fullName, f.GetName())})
		}
	}
	if enumDispatch || embed {
		return
	}
}

func parseEnumValueFullName(v string) (enumFull string, valueFull string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", ""
	}
	if !strings.HasPrefix(v, ".") {
		v = "." + v
	}
	lastDot := strings.LastIndex(v, ".")
	if lastDot <= 0 || lastDot == len(v)-1 {
		return "", ""
	}
	return v[:lastDot], v
}

func hasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Level == DiagError {
			return true
		}
	}
	return false
}
