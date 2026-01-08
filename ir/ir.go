package ir

import (
	"github.com/yaroher/protoc-gen-go-plain/goplain"
)

type Kind string

const (
	KindBool     Kind = "bool"
	KindInt32    Kind = "int32"
	KindInt64    Kind = "int64"
	KindSint32   Kind = "sint32"
	KindSint64   Kind = "sint64"
	KindUint32   Kind = "uint32"
	KindUint64   Kind = "uint64"
	KindFixed32  Kind = "fixed32"
	KindFixed64  Kind = "fixed64"
	KindSfixed32 Kind = "sfixed32"
	KindSfixed64 Kind = "sfixed64"
	KindFloat    Kind = "float"
	KindDouble   Kind = "double"
	KindString   Kind = "string"
	KindBytes    Kind = "bytes"
	KindMessage  Kind = "message"
	KindEnum     Kind = "enum"
)

type GoIdent struct {
	Name       string
	ImportPath string
}

type TypeRef struct {
	Kind          Kind
	ProtoFullName string
	GoIdent       GoIdent
}

type Oneof struct {
	Name   string
	GoName string
	Fields []*Field
}

type Field struct {
	ProtoName    string
	GoName       string
	Kind         Kind
	IsList       bool
	IsMap        bool
	IsOptional   bool
	IsEmbedded   bool
	IsSerialized bool
	IsVirtual    bool

	GoType GoIdent

	Oneof *Oneof

	MessageType *TypeRef
	EnumType    *TypeRef

	MapKey   *Field
	MapValue *Field

	Override *goplain.OverwriteType
}

type Message struct {
	ProtoFullName string
	GoIdent       GoIdent
	PlainName     string
	Generate      bool
	TypeAlias     bool

	Fields []*Field
	Oneofs []*Oneof

	AliasValueField *Field
}

type File struct {
	GoPackage    string
	GoImportPath string
	PlainSuffix  string

	Messages        []*Message
	VirtualMessages []*goplain.VirtualMessage
}
