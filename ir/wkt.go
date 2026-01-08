package ir

type WKTCast struct {
	BaseType string
	ToVal    string
	ToPtr    string
	FromVal  string
	FromPtr  string
}

var WKTCasts = map[string]WKTCast{
	"google.protobuf.StringValue": {
		BaseType: "string",
		ToVal:    "StringValueToString",
		ToPtr:    "StringValueToPtrString",
		FromVal:  "StringValueFromString",
		FromPtr:  "StringValueFromPtrString",
	},
	"google.protobuf.BoolValue": {
		BaseType: "bool",
		ToVal:    "BoolValueToBool",
		ToPtr:    "BoolValueToPtrBool",
		FromVal:  "BoolValueFromBool",
		FromPtr:  "BoolValueFromPtrBool",
	},
	"google.protobuf.Int32Value": {
		BaseType: "int32",
		ToVal:    "Int32ValueToInt32",
		ToPtr:    "Int32ValueToPtrInt32",
		FromVal:  "Int32ValueFromInt32",
		FromPtr:  "Int32ValueFromPtrInt32",
	},
	"google.protobuf.Int64Value": {
		BaseType: "int64",
		ToVal:    "Int64ValueToInt64",
		ToPtr:    "Int64ValueToPtrInt64",
		FromVal:  "Int64ValueFromInt64",
		FromPtr:  "Int64ValueFromPtrInt64",
	},
	"google.protobuf.UInt32Value": {
		BaseType: "uint32",
		ToVal:    "UInt32ValueToUint32",
		ToPtr:    "UInt32ValueToPtrUint32",
		FromVal:  "UInt32ValueFromUint32",
		FromPtr:  "UInt32ValueFromPtrUint32",
	},
	"google.protobuf.UInt64Value": {
		BaseType: "uint64",
		ToVal:    "UInt64ValueToUint64",
		ToPtr:    "UInt64ValueToPtrUint64",
		FromVal:  "UInt64ValueFromUint64",
		FromPtr:  "UInt64ValueFromPtrUint64",
	},
	"google.protobuf.FloatValue": {
		BaseType: "float32",
		ToVal:    "FloatValueToFloat32",
		ToPtr:    "FloatValueToPtrFloat32",
		FromVal:  "FloatValueFromFloat32",
		FromPtr:  "FloatValueFromPtrFloat32",
	},
	"google.protobuf.DoubleValue": {
		BaseType: "float64",
		ToVal:    "DoubleValueToFloat64",
		ToPtr:    "DoubleValueToPtrFloat64",
		FromVal:  "DoubleValueFromFloat64",
		FromPtr:  "DoubleValueFromPtrFloat64",
	},
	"google.protobuf.BytesValue": {
		BaseType: "[]byte",
		ToVal:    "BytesValueToBytes",
		ToPtr:    "BytesValueToPtrBytes",
		FromVal:  "BytesValueFromBytes",
		FromPtr:  "BytesValueFromPtrBytes",
	},
}
