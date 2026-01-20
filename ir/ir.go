package ir

import (
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/typepb"
)

// IR is an immutable transformation plan built from a protogen.Plugin.
type IR struct {
	Files    map[string]*FileIR    // key: file name (.proto)
	Messages map[string]*MessageIR // key: full name (.pkg.Msg)
	Enums    map[string]*EnumIR    // optional, populated if needed

	Renames         *RenameIndex
	TypeResolutions *TypeIndex
	Diagnostics     []Diagnostic
	Options         IRConfig
}

type IRConfig struct {
	PlainSuffix string
}

// FileIR holds per-file ordering and minimal metadata.
type FileIR struct {
	Name         string
	Package      string
	Generate     bool
	MessageOrder []string
	EnumOrder    []string
}

// MessageIR holds transformation plan for a message.
type MessageIR struct {
	FullName    string
	NewFullName string
	OrigName    string
	NewName     string
	Parent      string
	File        string
	Generate    bool
	IsMapEntry  bool

	FieldPlan      []*FieldPlan
	OneofPlan      []*OneofPlan
	VirtualPlan    []*FieldPlan
	GeneratedEnums []*EnumSpec
}

// EnumIR placeholder for future enum-specific transforms.
type EnumIR struct {
	FullName string
	OrigName string
	NewName  string
	File     string
}

type EnumSpec struct {
	Name   string
	Values []EnumValueSpec
}

type EnumValueSpec struct {
	Name   string
	Number int32
}

// FieldPlan describes how an input field becomes an output field.
type FieldPlan struct {
	OrigField *FieldRef
	NewField  FieldSpec
	Ops       []FieldOp
	Origin    FieldOrigin
}

type FieldRef struct {
	MessageFullName string
	FieldName       string
	FieldNumber     int32
}

// FieldSpec is a fully-specified FieldDescriptorProto payload.
type FieldSpec struct {
	Name           string
	Number         int32
	Label          descriptorpb.FieldDescriptorProto_Label
	Type           descriptorpb.FieldDescriptorProto_Type
	TypeName       string
	OneofIndex     *int32
	Proto3Optional bool
	Options        *descriptorpb.FieldOptions
}

type FieldOrigin struct {
	IsVirtual    bool
	IsEmbedded   bool
	EmbedSource  *FieldRef
	IsTypeAlias  bool
	IsSerialized bool
	IsOneof      bool
	OneofGroup   string
	OneofEnums   []string
	OriginalType string
}

type FieldOpKind string

const (
	OpRename           FieldOpKind = "rename"
	OpTypeAliasResolve FieldOpKind = "type_alias"
	OpSerialize        FieldOpKind = "serialize"
	OpEmbed            FieldOpKind = "embed"
	OpOverrideType     FieldOpKind = "override_type"
	OpDrop             FieldOpKind = "drop"
)

type FieldOp struct {
	Kind   FieldOpKind
	Reason string
	Data   map[string]string
}

// OneofPlan describes oneof transformations.
type OneofPlan struct {
	OrigName string
	NewName  string
	Fields   []string
	Ops      []OneofOp

	EnumDispatch    *EnumDispatchPlan
	Embed           bool
	EmbedWithPrefix bool
	Discriminator   bool
}

type OneofOpKind string

const (
	OneofOpEmbed        OneofOpKind = "embed"
	OneofOpEnumDispatch OneofOpKind = "enum_dispatch"
	OneofOpDrop         OneofOpKind = "drop"
)

type OneofOp struct {
	Kind   OneofOpKind
	Reason string
	Data   map[string]string
}

type EnumDispatchPlan struct {
	EnumFullName string
	WithPrefix   bool
	Generated    bool
}

// RenameIndex stores rename mappings for messages/enums.
type RenameIndex struct {
	Messages map[string]string
	Enums    map[string]string
}

// TypeIndex stores resolved alias and override info.
type TypeIndex struct {
	Alias     map[string]descriptorpb.FieldDescriptorProto_Type
	Overrides []TypeOverrideRule
}

type TypeOverrideRule struct {
	Selector TypeOverrideSelector
	Target   GoIdent
}

type TypeOverrideSelector struct {
	TargetFullPath   string
	FieldKind        *typepb.Field_Kind
	FieldCardinality *typepb.Field_Cardinality
	FieldTypeName    string
}

type GoIdent struct {
	Name       string
	ImportPath string
}

type DiagnosticLevel string

const (
	DiagInfo  DiagnosticLevel = "info"
	DiagWarn  DiagnosticLevel = "warn"
	DiagError DiagnosticLevel = "error"
)

type Diagnostic struct {
	Level   DiagnosticLevel
	Message string
	Subject string
}
