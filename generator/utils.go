package generator

import (
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/samber/lo"
	"github.com/yaroher/protoc-gen-go-plain/generator/empath"
	"google.golang.org/protobuf/types/known/typepb"
)

func stringOrDefault(s string, def string) string {
	if s == "" {
		return def
	}
	return s
}

func removeOneoffs(msg *typepb.Type, names []string) {
	oldOneofs := msg.Oneofs
	newOneofs := lo.Filter(oldOneofs, func(oneof string, _ int) bool {
		return !lo.Contains(names, oneof)
	})
	msg.Oneofs = newOneofs
}

func pickOneFromMarkers(maps map[string]string, keys ...string) map[string]string {
	for ks := range maps {
		for _, key := range keys {
			if value, ok := maps[ks]; ok {
				return map[string]string{key: value}
			}
		}
	}
	return nil
}

func decodeEmpath(s string) string {
	repl := strings.NewReplacer(
		"%3F", "?",
		"%3B", ";",
		"%3D", "=",
		"|", "/",
	)
	return repl.Replace(s)
}

func encodeMarkerValue(s string) string {
	repl := strings.NewReplacer(
		"/", "|",
		"?", "%3F",
		";", "%3B",
		"=", "%3D",
	)
	return repl.Replace(s)
}

func goFieldNameFromPlain(name string) string {
	if strings.HasSuffix(name, "CRF") {
		base := strings.TrimSuffix(name, "CRF")
		return strcase.ToCamel(base) + "CRF"
	}
	return strcase.ToCamel(name)
}

func jsonTagFromPlain(name string) string {
	if strings.HasSuffix(name, "CRF") {
		base := strings.TrimSuffix(name, "CRF")
		return strcase.ToLowerCamel(base) + "CRF"
	}
	return strcase.ToLowerCamel(name)
}

type mapFieldInfo struct {
	keyKind   typepb.Field_Kind
	valueKind typepb.Field_Kind
	valueType string
}

func mapFieldInfoFor(field *typepb.Field) (mapFieldInfo, bool) {
	if field == nil {
		return mapFieldInfo{}, false
	}
	if !hasMarker(field.TypeUrl, mapMarker) {
		return mapFieldInfo{}, false
	}
	info := mapFieldInfo{}
	for _, segment := range empath.Parse(field.TypeUrl) {
		if v := segment.GetMarker(mapKeyKind); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				info.keyKind = typepb.Field_Kind(n)
			}
		}
		if v := segment.GetMarker(mapValueKind); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				info.valueKind = typepb.Field_Kind(n)
			}
		}
		if v := segment.GetMarker(mapValueTypeURL); v != "" {
			info.valueType = decodeEmpath(v)
		}
	}
	if info.keyKind == 0 || info.valueKind == 0 {
		return mapFieldInfo{}, false
	}
	return info, true
}

func mapScalarGoType(kind typepb.Field_Kind) string {
	switch kind {
	case typepb.Field_TYPE_STRING:
		return "string"
	case typepb.Field_TYPE_BOOL:
		return "bool"
	case typepb.Field_TYPE_INT32, typepb.Field_TYPE_SINT32, typepb.Field_TYPE_SFIXED32:
		return "int32"
	case typepb.Field_TYPE_UINT32, typepb.Field_TYPE_FIXED32:
		return "uint32"
	case typepb.Field_TYPE_INT64, typepb.Field_TYPE_SINT64, typepb.Field_TYPE_SFIXED64:
		return "int64"
	case typepb.Field_TYPE_UINT64, typepb.Field_TYPE_FIXED64:
		return "uint64"
	case typepb.Field_TYPE_ENUM:
		return "int32"
	default:
		return "any"
	}
}

func wktGoType(typeURL string) (string, string, bool) {
	target := empath.Parse(typeURL).Last().Value()
	if !strings.HasPrefix(target, "google.protobuf.") {
		return "", "", false
	}
	switch getShortName(target) {
	case "Timestamp":
		return "timestamppb.Timestamp", "google.golang.org/protobuf/types/known/timestamppb", true
	case "Duration":
		return "durationpb.Duration", "google.golang.org/protobuf/types/known/durationpb", true
	case "Any":
		return "anypb.Any", "google.golang.org/protobuf/types/known/anypb", true
	case "Struct":
		return "structpb.Struct", "google.golang.org/protobuf/types/known/structpb", true
	case "Value":
		return "structpb.Value", "google.golang.org/protobuf/types/known/structpb", true
	case "ListValue":
		return "structpb.ListValue", "google.golang.org/protobuf/types/known/structpb", true
	case "Empty":
		return "emptypb.Empty", "google.golang.org/protobuf/types/known/emptypb", true
	case "FieldMask":
		return "fieldmaskpb.FieldMask", "google.golang.org/protobuf/types/known/fieldmaskpb", true
	case "BoolValue":
		return "wrapperspb.BoolValue", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "BytesValue":
		return "wrapperspb.BytesValue", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "DoubleValue":
		return "wrapperspb.DoubleValue", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "FloatValue":
		return "wrapperspb.FloatValue", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "Int32Value":
		return "wrapperspb.Int32Value", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "Int64Value":
		return "wrapperspb.Int64Value", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "StringValue":
		return "wrapperspb.StringValue", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "UInt32Value":
		return "wrapperspb.UInt32Value", "google.golang.org/protobuf/types/known/wrapperspb", true
	case "UInt64Value":
		return "wrapperspb.UInt64Value", "google.golang.org/protobuf/types/known/wrapperspb", true
	default:
		return "", "", false
	}
}
