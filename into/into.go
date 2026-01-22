package into

import (
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func SetMessage(msg proto.Message, path []string, val proto.Message) bool {
	if msg == nil || val == nil {
		return false
	}
	m := msg.ProtoReflect()
	for i, name := range path {
		fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			return false
		}
		if i == len(path)-1 {
			if fd.Kind() != protoreflect.MessageKind {
				return false
			}
			m.Set(fd, protoreflect.ValueOfMessage(val.ProtoReflect()))
			return true
		}
		if fd.Kind() != protoreflect.MessageKind {
			return false
		}
		m = m.Mutable(fd).Message()
	}
	return false
}

func GetMessage(msg proto.Message, path []string) (proto.Message, bool) {
	if msg == nil {
		return nil, false
	}
	m := msg.ProtoReflect()
	for i, name := range path {
		fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			return nil, false
		}
		if i == len(path)-1 {
			if fd.Kind() != protoreflect.MessageKind {
				return nil, false
			}
			if !m.Has(fd) {
				return nil, false
			}
			return m.Get(fd).Message().Interface(), true
		}
		if fd.Kind() != protoreflect.MessageKind {
			return nil, false
		}
		if !m.Has(fd) {
			return nil, false
		}
		m = m.Get(fd).Message()
	}
	return nil, false
}

func SetScalar(msg proto.Message, path []string, kind protoreflect.Kind, val protoreflect.Value) bool {
	if msg == nil {
		return false
	}
	m := msg.ProtoReflect()
	for i, name := range path {
		fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			return false
		}
		if i == len(path)-1 {
			if fd.Kind() != kind {
				return false
			}
			m.Set(fd, val)
			return true
		}
		if fd.Kind() != protoreflect.MessageKind {
			return false
		}
		m = m.Mutable(fd).Message()
	}
	return false
}

func GetScalar(msg proto.Message, path []string, kind protoreflect.Kind) (protoreflect.Value, bool) {
	if msg == nil {
		return protoreflect.Value{}, false
	}
	m := msg.ProtoReflect()
	for i, name := range path {
		fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			return protoreflect.Value{}, false
		}
		if i == len(path)-1 {
			if fd.Kind() != kind {
				return protoreflect.Value{}, false
			}
			if !m.Has(fd) {
				return protoreflect.Value{}, false
			}
			return m.Get(fd), true
		}
		if fd.Kind() != protoreflect.MessageKind {
			return protoreflect.Value{}, false
		}
		if !m.Has(fd) {
			return protoreflect.Value{}, false
		}
		m = m.Get(fd).Message()
	}
	return protoreflect.Value{}, false
}

func SetScalarList(msg proto.Message, path []string, kind protoreflect.Kind, vals any) bool {
	if msg == nil {
		return false
	}
	m := msg.ProtoReflect()
	for i, name := range path {
		fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			return false
		}
		if i == len(path)-1 {
			if fd.Kind() != kind || fd.Cardinality() != protoreflect.Repeated {
				return false
			}
			list := m.Mutable(fd).List()
			list.Truncate(0)
			switch v := vals.(type) {
			case []string:
				for _, el := range v {
					list.Append(protoreflect.ValueOfString(el))
				}
			case []bool:
				for _, el := range v {
					list.Append(protoreflect.ValueOfBool(el))
				}
			case []int32:
				for _, el := range v {
					list.Append(protoreflect.ValueOfInt32(el))
				}
			case []uint32:
				for _, el := range v {
					list.Append(protoreflect.ValueOfUint32(el))
				}
			case []int64:
				for _, el := range v {
					list.Append(protoreflect.ValueOfInt64(el))
				}
			case []uint64:
				for _, el := range v {
					list.Append(protoreflect.ValueOfUint64(el))
				}
			case []float32:
				for _, el := range v {
					list.Append(protoreflect.ValueOfFloat32(el))
				}
			case []float64:
				for _, el := range v {
					list.Append(protoreflect.ValueOfFloat64(el))
				}
			case [][]byte:
				for _, el := range v {
					list.Append(protoreflect.ValueOfBytes(el))
				}
			case []protoreflect.EnumNumber:
				for _, el := range v {
					list.Append(protoreflect.ValueOfEnum(el))
				}
			default:
				return false
			}
			return true
		}
		if fd.Kind() != protoreflect.MessageKind {
			return false
		}
		m = m.Mutable(fd).Message()
	}
	return false
}

func GetScalarList(msg proto.Message, path []string, kind protoreflect.Kind) (protoreflect.List, bool) {
	if msg == nil {
		return nil, false
	}
	m := msg.ProtoReflect()
	for i, name := range path {
		fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			return nil, false
		}
		if i == len(path)-1 {
			if fd.Kind() != kind || fd.Cardinality() != protoreflect.Repeated {
				return nil, false
			}
			list := m.Get(fd).List()
			if list.Len() == 0 {
				return nil, false
			}
			return list, true
		}
		if fd.Kind() != protoreflect.MessageKind {
			return nil, false
		}
		if !m.Has(fd) {
			return nil, false
		}
		m = m.Get(fd).Message()
	}
	return nil, false
}

func ParseCRFPath(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "/")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if idx := strings.Index(p, "?"); idx >= 0 {
			p = p[:idx]
		}
		result = append(result, p)
	}
	return result
}

func SetString(msg proto.Message, path []string, val string) bool {
	return SetScalar(msg, path, protoreflect.StringKind, protoreflect.ValueOfString(val))
}
func GetString(msg proto.Message, path []string) (string, bool) {
	v, ok := GetScalar(msg, path, protoreflect.StringKind)
	if !ok {
		return "", false
	}
	return v.String(), true
}
func SetStringList(msg proto.Message, path []string, vals []string) bool {
	return SetScalarList(msg, path, protoreflect.StringKind, vals)
}
func GetStringList(msg proto.Message, path []string) ([]string, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.StringKind)
	if !ok {
		return nil, false
	}
	result := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = list.Get(i).String()
	}
	return result, true
}

func SetBool(msg proto.Message, path []string, val bool) bool {
	return SetScalar(msg, path, protoreflect.BoolKind, protoreflect.ValueOfBool(val))
}
func GetBool(msg proto.Message, path []string) (bool, bool) {
	v, ok := GetScalar(msg, path, protoreflect.BoolKind)
	if !ok {
		return false, false
	}
	return v.Bool(), true
}
func SetBoolList(msg proto.Message, path []string, vals []bool) bool {
	return SetScalarList(msg, path, protoreflect.BoolKind, vals)
}
func GetBoolList(msg proto.Message, path []string) ([]bool, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.BoolKind)
	if !ok {
		return nil, false
	}
	result := make([]bool, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = list.Get(i).Bool()
	}
	return result, true
}

func SetInt32(msg proto.Message, path []string, val int32) bool {
	return SetScalar(msg, path, protoreflect.Int32Kind, protoreflect.ValueOfInt32(val))
}
func GetInt32(msg proto.Message, path []string) (int32, bool) {
	v, ok := GetScalar(msg, path, protoreflect.Int32Kind)
	if !ok {
		return 0, false
	}
	return int32(v.Int()), true
}
func SetInt32List(msg proto.Message, path []string, vals []int32) bool {
	return SetScalarList(msg, path, protoreflect.Int32Kind, vals)
}
func GetInt32List(msg proto.Message, path []string) ([]int32, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.Int32Kind)
	if !ok {
		return nil, false
	}
	result := make([]int32, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = int32(list.Get(i).Int())
	}
	return result, true
}

func SetUint32(msg proto.Message, path []string, val uint32) bool {
	return SetScalar(msg, path, protoreflect.Uint32Kind, protoreflect.ValueOfUint32(val))
}
func GetUint32(msg proto.Message, path []string) (uint32, bool) {
	v, ok := GetScalar(msg, path, protoreflect.Uint32Kind)
	if !ok {
		return 0, false
	}
	return uint32(v.Uint()), true
}
func SetUint32List(msg proto.Message, path []string, vals []uint32) bool {
	return SetScalarList(msg, path, protoreflect.Uint32Kind, vals)
}
func GetUint32List(msg proto.Message, path []string) ([]uint32, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.Uint32Kind)
	if !ok {
		return nil, false
	}
	result := make([]uint32, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = uint32(list.Get(i).Uint())
	}
	return result, true
}

func SetInt64(msg proto.Message, path []string, val int64) bool {
	return SetScalar(msg, path, protoreflect.Int64Kind, protoreflect.ValueOfInt64(val))
}
func GetInt64(msg proto.Message, path []string) (int64, bool) {
	v, ok := GetScalar(msg, path, protoreflect.Int64Kind)
	if !ok {
		return 0, false
	}
	return v.Int(), true
}
func SetInt64List(msg proto.Message, path []string, vals []int64) bool {
	return SetScalarList(msg, path, protoreflect.Int64Kind, vals)
}
func GetInt64List(msg proto.Message, path []string) ([]int64, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.Int64Kind)
	if !ok {
		return nil, false
	}
	result := make([]int64, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = list.Get(i).Int()
	}
	return result, true
}

func SetUint64(msg proto.Message, path []string, val uint64) bool {
	return SetScalar(msg, path, protoreflect.Uint64Kind, protoreflect.ValueOfUint64(val))
}
func GetUint64(msg proto.Message, path []string) (uint64, bool) {
	v, ok := GetScalar(msg, path, protoreflect.Uint64Kind)
	if !ok {
		return 0, false
	}
	return v.Uint(), true
}
func SetUint64List(msg proto.Message, path []string, vals []uint64) bool {
	return SetScalarList(msg, path, protoreflect.Uint64Kind, vals)
}
func GetUint64List(msg proto.Message, path []string) ([]uint64, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.Uint64Kind)
	if !ok {
		return nil, false
	}
	result := make([]uint64, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = list.Get(i).Uint()
	}
	return result, true
}

func SetFloat32(msg proto.Message, path []string, val float32) bool {
	return SetScalar(msg, path, protoreflect.FloatKind, protoreflect.ValueOfFloat32(val))
}
func GetFloat32(msg proto.Message, path []string) (float32, bool) {
	v, ok := GetScalar(msg, path, protoreflect.FloatKind)
	if !ok {
		return 0, false
	}
	return float32(v.Float()), true
}
func SetFloat32List(msg proto.Message, path []string, vals []float32) bool {
	return SetScalarList(msg, path, protoreflect.FloatKind, vals)
}
func GetFloat32List(msg proto.Message, path []string) ([]float32, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.FloatKind)
	if !ok {
		return nil, false
	}
	result := make([]float32, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = float32(list.Get(i).Float())
	}
	return result, true
}

func SetFloat64(msg proto.Message, path []string, val float64) bool {
	return SetScalar(msg, path, protoreflect.DoubleKind, protoreflect.ValueOfFloat64(val))
}
func GetFloat64(msg proto.Message, path []string) (float64, bool) {
	v, ok := GetScalar(msg, path, protoreflect.DoubleKind)
	if !ok {
		return 0, false
	}
	return v.Float(), true
}
func SetFloat64List(msg proto.Message, path []string, vals []float64) bool {
	return SetScalarList(msg, path, protoreflect.DoubleKind, vals)
}
func GetFloat64List(msg proto.Message, path []string) ([]float64, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.DoubleKind)
	if !ok {
		return nil, false
	}
	result := make([]float64, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = list.Get(i).Float()
	}
	return result, true
}

func SetBytes(msg proto.Message, path []string, val []byte) bool {
	return SetScalar(msg, path, protoreflect.BytesKind, protoreflect.ValueOfBytes(val))
}
func GetBytes(msg proto.Message, path []string) ([]byte, bool) {
	v, ok := GetScalar(msg, path, protoreflect.BytesKind)
	if !ok {
		return nil, false
	}
	return v.Bytes(), true
}
func SetBytesList(msg proto.Message, path []string, vals [][]byte) bool {
	return SetScalarList(msg, path, protoreflect.BytesKind, vals)
}
func GetBytesList(msg proto.Message, path []string) ([][]byte, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.BytesKind)
	if !ok {
		return nil, false
	}
	result := make([][]byte, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = list.Get(i).Bytes()
	}
	return result, true
}

func SetEnum(msg proto.Message, path []string, val protoreflect.EnumNumber) bool {
	return SetScalar(msg, path, protoreflect.EnumKind, protoreflect.ValueOfEnum(val))
}
func GetEnum(msg proto.Message, path []string) (protoreflect.EnumNumber, bool) {
	v, ok := GetScalar(msg, path, protoreflect.EnumKind)
	if !ok {
		return 0, false
	}
	return v.Enum(), true
}
func SetEnumList(msg proto.Message, path []string, vals []protoreflect.EnumNumber) bool {
	return SetScalarList(msg, path, protoreflect.EnumKind, vals)
}
func GetEnumList(msg proto.Message, path []string) ([]protoreflect.EnumNumber, bool) {
	list, ok := GetScalarList(msg, path, protoreflect.EnumKind)
	if !ok {
		return nil, false
	}
	result := make([]protoreflect.EnumNumber, list.Len())
	for i := 0; i < list.Len(); i++ {
		result[i] = list.Get(i).Enum()
	}
	return result, true
}
