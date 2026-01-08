package ir

import (
	"encoding/json"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
)

type FileSnapshot struct {
	GoPackage       string                    `json:"go_package,omitempty"`
	GoImportPath    string                    `json:"go_import_path,omitempty"`
	PlainSuffix     string                    `json:"plain_suffix,omitempty"`
	Messages        []*MessageSnapshot        `json:"messages,omitempty"`
	VirtualMessages []*goplain.VirtualMessage `json:"virtual_messages,omitempty"`
}

type MessageSnapshot struct {
	ProtoFullName   string           `json:"proto_full_name,omitempty"`
	GoIdent         GoIdent          `json:"go_ident,omitempty"`
	PlainName       string           `json:"plain_name,omitempty"`
	Generate        bool             `json:"generate,omitempty"`
	TypeAlias       bool             `json:"type_alias,omitempty"`
	Fields          []*FieldSnapshot `json:"fields,omitempty"`
	Oneofs          []*OneofSnapshot `json:"oneofs,omitempty"`
	AliasValueField string           `json:"alias_value_field,omitempty"`
}

type FieldSnapshot struct {
	ProtoName    string  `json:"proto_name,omitempty"`
	GoName       string  `json:"go_name,omitempty"`
	Kind         Kind    `json:"kind,omitempty"`
	IsList       bool    `json:"is_list,omitempty"`
	IsMap        bool    `json:"is_map,omitempty"`
	IsOptional   bool    `json:"is_optional,omitempty"`
	IsEmbedded   bool    `json:"is_embedded,omitempty"`
	IsSerialized bool    `json:"is_serialized,omitempty"`
	IsVirtual    bool    `json:"is_virtual,omitempty"`
	GoType       GoIdent `json:"go_type,omitempty"`

	Oneof       string                 `json:"oneof,omitempty"`
	MessageType *TypeRefSnapshot       `json:"message_type,omitempty"`
	EnumType    *TypeRefSnapshot       `json:"enum_type,omitempty"`
	MapKey      *FieldSnapshot         `json:"map_key,omitempty"`
	MapValue    *FieldSnapshot         `json:"map_value,omitempty"`
	Override    *goplain.OverwriteType `json:"override,omitempty"`
}

type OneofSnapshot struct {
	Name   string   `json:"name,omitempty"`
	GoName string   `json:"go_name,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

type TypeRefSnapshot struct {
	Kind          Kind    `json:"kind,omitempty"`
	ProtoFullName string  `json:"proto_full_name,omitempty"`
	GoIdent       GoIdent `json:"go_ident,omitempty"`
}

func (f *File) Snapshot() *FileSnapshot {
	if f == nil {
		return nil
	}
	out := &FileSnapshot{
		GoPackage:       f.GoPackage,
		GoImportPath:    f.GoImportPath,
		PlainSuffix:     f.PlainSuffix,
		VirtualMessages: f.VirtualMessages,
	}
	if len(f.Messages) > 0 {
		out.Messages = make([]*MessageSnapshot, 0, len(f.Messages))
		for _, msg := range f.Messages {
			out.Messages = append(out.Messages, msgSnapshot(msg))
		}
	}
	return out
}

func msgSnapshot(msg *Message) *MessageSnapshot {
	if msg == nil {
		return nil
	}
	out := &MessageSnapshot{
		ProtoFullName: msg.ProtoFullName,
		GoIdent:       msg.GoIdent,
		PlainName:     msg.PlainName,
		Generate:      msg.Generate,
		TypeAlias:     msg.TypeAlias,
	}
	if msg.AliasValueField != nil {
		out.AliasValueField = msg.AliasValueField.GoName
	}
	if len(msg.Fields) > 0 {
		out.Fields = make([]*FieldSnapshot, 0, len(msg.Fields))
		for _, field := range msg.Fields {
			out.Fields = append(out.Fields, fieldSnapshot(field))
		}
	}
	if len(msg.Oneofs) > 0 {
		out.Oneofs = make([]*OneofSnapshot, 0, len(msg.Oneofs))
		for _, oneof := range msg.Oneofs {
			out.Oneofs = append(out.Oneofs, oneofSnapshot(oneof))
		}
	}
	return out
}

func fieldSnapshot(field *Field) *FieldSnapshot {
	if field == nil {
		return nil
	}
	out := &FieldSnapshot{
		ProtoName:    field.ProtoName,
		GoName:       field.GoName,
		Kind:         field.Kind,
		IsList:       field.IsList,
		IsMap:        field.IsMap,
		IsOptional:   field.IsOptional,
		IsEmbedded:   field.IsEmbedded,
		IsSerialized: field.IsSerialized,
		IsVirtual:    field.IsVirtual,
		GoType:       field.GoType,
		Override:     field.Override,
	}
	if field.Oneof != nil {
		out.Oneof = field.Oneof.GoName
	}
	if field.MessageType != nil {
		out.MessageType = &TypeRefSnapshot{
			Kind:          field.MessageType.Kind,
			ProtoFullName: field.MessageType.ProtoFullName,
			GoIdent:       field.MessageType.GoIdent,
		}
	}
	if field.EnumType != nil {
		out.EnumType = &TypeRefSnapshot{
			Kind:          field.EnumType.Kind,
			ProtoFullName: field.EnumType.ProtoFullName,
			GoIdent:       field.EnumType.GoIdent,
		}
	}
	if field.MapKey != nil {
		out.MapKey = fieldSnapshot(field.MapKey)
	}
	if field.MapValue != nil {
		out.MapValue = fieldSnapshot(field.MapValue)
	}
	return out
}

func oneofSnapshot(oneof *Oneof) *OneofSnapshot {
	if oneof == nil {
		return nil
	}
	out := &OneofSnapshot{
		Name:   oneof.Name,
		GoName: oneof.GoName,
	}
	if len(oneof.Fields) > 0 {
		out.Fields = make([]string, 0, len(oneof.Fields))
		for _, field := range oneof.Fields {
			if field == nil {
				continue
			}
			out.Fields = append(out.Fields, field.GoName)
		}
	}
	return out
}

func (f *File) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Snapshot())
}

func (m *Message) MarshalJSON() ([]byte, error) {
	return json.Marshal(msgSnapshot(m))
}

func (f *Field) MarshalJSON() ([]byte, error) {
	return json.Marshal(fieldSnapshot(f))
}

func (f *File) ToJSON() ([]byte, error) {
	return json.Marshal(f.Snapshot())
}

func (f *File) ToJSONIndent() ([]byte, error) {
	return json.MarshalIndent(f.Snapshot(), "", "  ")
}
