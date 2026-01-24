package generator

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// jx package path
var jxPkg = protogen.GoImportPath("github.com/go-faster/jx")

// generateJSONMethods generates MarshalJX and UnmarshalJX methods
func (g *Generator) generateJSONMethods(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	g.generateMarshalJX(gf, msg, f)
	g.generateUnmarshalJX(gf, msg, f)
}

// generateMarshalJX generates method for JSON encoding with jx
// Uses Src_ for sparse serialization - only encodes fields listed in Src_
func (g *Generator) generateMarshalJX(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	plainType := msg.GoName

	gf.P("// MarshalJX encodes ", plainType, " to JSON using jx.Encoder")
	gf.P("// If Src_ is set, only fields in Src_ are encoded (sparse mode)")
	gf.P("func (p *", plainType, ") MarshalJX(e *", gf.QualifiedGoIdent(jxPkg.Ident("Encoder")), ") {")
	gf.P("\tif p == nil {")
	gf.P("\t\te.Null()")
	gf.P("\t\treturn")
	gf.P("\t}")
	gf.P()

	// Build src lookup set for sparse mode
	gf.P("\t// Build lookup set from Src_ for sparse serialization")
	gf.P("\tsrcSet := make(map[uint16]struct{}, len(p.Src_))")
	gf.P("\tfor _, idx := range p.Src_ {")
	gf.P("\t\tsrcSet[idx] = struct{}{}")
	gf.P("\t}")
	gf.P("\tsparse := len(p.Src_) > 0")
	gf.P()

	gf.P("\te.ObjStart()")
	gf.P()

	// Always encode _src if present
	gf.P("\tif sparse {")
	gf.P("\t\te.FieldStart(\"_src\")")
	gf.P("\t\te.ArrStart()")
	gf.P("\t\tfor _, idx := range p.Src_ {")
	gf.P("\t\t\te.UInt16(idx)")
	gf.P("\t\t}")
	gf.P("\t\te.ArrEnd()")
	gf.P("\t}")

	// Generate oneof case field encodings
	for _, eo := range msg.EmbeddedOneofs {
		gf.P("\tif p.", eo.CaseFieldName, " != \"\" {")
		gf.P("\t\te.FieldStart(\"", eo.JSONName, "\")")
		gf.P("\t\te.Str(p.", eo.CaseFieldName, ")")
		gf.P("\t}")
	}

	// Generate field encodings with sparse check
	for _, field := range msg.Fields {
		g.generateMarshalJXFieldSparse(gf, field, f)
	}

	gf.P("\te.ObjEnd()")
	gf.P("}")
	gf.P()

	// Generate helper method that returns bytes
	gf.P("// MarshalJSON implements json.Marshaler using jx")
	gf.P("func (p *", plainType, ") MarshalJSON() ([]byte, error) {")
	gf.P("\te := ", gf.QualifiedGoIdent(jxPkg.Ident("GetEncoder")), "()")
	gf.P("\tdefer ", gf.QualifiedGoIdent(jxPkg.Ident("PutEncoder")), "(e)")
	gf.P("\tp.MarshalJX(e)")
	gf.P("\treturn e.Bytes(), nil")
	gf.P("}")
	gf.P()
}

// generateMarshalJXFieldSparse generates encoding for a single field with sparse check
func (g *Generator) generateMarshalJXFieldSparse(gf *protogen.GeneratedFile, field *IRField, f *protogen.File) {
	fieldAccess := "p." + field.GoName
	jsonName := field.JSONName

	// Check if field is in srcSet (sparse mode) or not sparse
	gf.P("\tif _, inSrc := srcSet[", field.Index, "]; !sparse || inSrc {")

	// Handle optional/pointer fields with omitempty
	if field.GoType.IsPointer {
		gf.P("\t\tif ", fieldAccess, " != nil {")
		gf.P("\t\t\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t\t\t")
		gf.P("\t\t}")
	} else if field.IsRepeated {
		gf.P("\t\tif len(", fieldAccess, ") > 0 {")
		gf.P("\t\t\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t\t\t")
		gf.P("\t\t}")
	} else if field.Kind == KindMessage {
		gf.P("\t\tif ", fieldAccess, " != nil {")
		gf.P("\t\t\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t\t\t")
		gf.P("\t\t}")
	} else {
		// Scalar - always encode (no omitempty for simplicity)
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t\t")
	}

	gf.P("\t}")
}

// generateMarshalJXValue generates the actual value encoding
func (g *Generator) generateMarshalJXValue(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string) {
	// Handle pointer dereference - but NOT for message types (protojson needs pointer)
	valueAccess := access
	if field.GoType.IsPointer && !field.IsRepeated && field.Kind != KindMessage {
		valueAccess = "*" + access
	}

	if field.IsRepeated && !field.IsMap {
		// Array
		gf.P(indent, "e.ArrStart()")
		gf.P(indent, "for _, v := range ", access, " {")
		g.generateMarshalJXSingleValue(gf, field, "v", f, indent+"\t")
		gf.P(indent, "}")
		gf.P(indent, "e.ArrEnd()")
		return
	}

	if field.IsMap {
		g.generateMarshalJXMap(gf, field, access, f, indent)
		return
	}

	g.generateMarshalJXSingleValue(gf, field, valueAccess, f, indent)
}

// generateMarshalJXSingleValue generates encoding for a single non-repeated value
// isArrayElem indicates if this is called from array iteration (v is value, not pointer)
func (g *Generator) generateMarshalJXSingleValue(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string) {
	// If field has type override with incompatible types, cast to source type for JSON
	valueAccess := access
	if field.NeedsCaster && field.SourceGoType.Name != "" {
		valueAccess = field.SourceGoType.Name + "(" + access + ")"
	}

	switch field.Kind {
	case KindScalar:
		g.generateMarshalJXScalar(gf, field, valueAccess, indent)
	case KindMessage:
		// Check if the message type has generate=true (has MarshalJX)
		hasMarshalJX := false
		if field.Source != nil && field.Source.Message != nil {
			msgOpts := g.getMessageOptions(field.Source.Message)
			hasMarshalJX = msgOpts != nil && msgOpts.Generate
		}

		if hasMarshalJX {
			// Plain type - has MarshalJX
			// If access is "v" from array iteration and type is not pointer, need &v
			if access == "v" && !field.GoType.IsPointer {
				gf.P(indent, "(&", access, ").MarshalJX(e)")
			} else {
				gf.P(indent, access, ".MarshalJX(e)")
			}
		} else {
			// Protobuf type - use protojson (needs pointer)
			protojsonPkg := protogen.GoImportPath("google.golang.org/protobuf/encoding/protojson")
			// If access is "v" from array iteration and type is value (not pointer), need &v
			marshalAccess := access
			if access == "v" && !field.GoType.IsPointer {
				marshalAccess = "&v"
			}
			gf.P(indent, "if data, err := ", gf.QualifiedGoIdent(protojsonPkg.Ident("Marshal")), "(", marshalAccess, "); err == nil {")
			gf.P(indent, "\te.Raw(data)")
			gf.P(indent, "} else {")
			gf.P(indent, "\te.Null()")
			gf.P(indent, "}")
		}
	case KindEnum:
		if field.EnumAsString {
			gf.P(indent, "e.Str(", access, ".String())")
		} else {
			gf.P(indent, "e.Int32(int32(", access, "))")
		}
	case KindBytes:
		gf.P(indent, "e.Base64(", access, ")")
	default:
		gf.P(indent, "// unsupported kind: ", field.Kind)
		gf.P(indent, "e.Null()")
	}
}

// generateMarshalJXScalar generates encoding for scalar types
func (g *Generator) generateMarshalJXScalar(gf *protogen.GeneratedFile, field *IRField, access string, indent string) {
	// If field has type override, use ScalarKind (original proto type) for encoding
	// Otherwise use GoType.Name
	if field.NeedsCaster {
		g.generateMarshalJXScalarByKind(gf, field.ScalarKind, access, indent)
		return
	}

	switch field.GoType.Name {
	case "string":
		gf.P(indent, "e.Str(", access, ")")
	case "bool":
		gf.P(indent, "e.Bool(", access, ")")
	case "int32":
		gf.P(indent, "e.Int32(", access, ")")
	case "int64":
		gf.P(indent, "e.Int64(", access, ")")
	case "uint32":
		gf.P(indent, "e.UInt32(", access, ")")
	case "uint64":
		gf.P(indent, "e.UInt64(", access, ")")
	case "float32":
		gf.P(indent, "e.Float32(", access, ")")
	case "float64":
		gf.P(indent, "e.Float64(", access, ")")
	case "byte":
		if field.GoType.IsSlice {
			gf.P(indent, "e.Base64(", access, ")")
		} else {
			gf.P(indent, "e.UInt32(uint32(", access, "))")
		}
	default:
		gf.P(indent, "// unknown scalar: ", field.GoType.Name)
		gf.P(indent, "e.Null()")
	}
}

// generateMarshalJXScalarByKind generates encoding based on protoreflect.Kind
func (g *Generator) generateMarshalJXScalarByKind(gf *protogen.GeneratedFile, kind protoreflect.Kind, access string, indent string) {
	switch kind {
	case protoreflect.StringKind:
		gf.P(indent, "e.Str(", access, ")")
	case protoreflect.BoolKind:
		gf.P(indent, "e.Bool(", access, ")")
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		gf.P(indent, "e.Int32(", access, ")")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		gf.P(indent, "e.Int64(", access, ")")
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		gf.P(indent, "e.UInt32(", access, ")")
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		gf.P(indent, "e.UInt64(", access, ")")
	case protoreflect.FloatKind:
		gf.P(indent, "e.Float32(", access, ")")
	case protoreflect.DoubleKind:
		gf.P(indent, "e.Float64(", access, ")")
	case protoreflect.BytesKind:
		gf.P(indent, "e.Base64(", access, ")")
	default:
		gf.P(indent, "// unknown kind: ", kind)
		gf.P(indent, "e.Null()")
	}
}

// generateMarshalJXMap generates encoding for map types
func (g *Generator) generateMarshalJXMap(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string) {
	gf.P(indent, "e.ObjStart()")
	gf.P(indent, "for k, v := range ", access, " {")

	// Map key must be string-like
	if field.MapKey != nil && field.MapKey.GoType.Name == "string" {
		gf.P(indent, "\te.FieldStart(k)")
	} else {
		// Convert key to string
		gf.P(indent, "\te.FieldStart(fmt.Sprint(k))")
	}

	// Encode value
	if field.MapValue != nil {
		g.generateMarshalJXSingleValue(gf, field.MapValue, "v", f, indent+"\t")
	} else {
		gf.P(indent, "\te.Null()")
	}

	gf.P(indent, "}")
	gf.P(indent, "e.ObjEnd()")
}

// generateUnmarshalJX generates method for JSON decoding with jx
func (g *Generator) generateUnmarshalJX(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	plainType := msg.GoName

	gf.P("// UnmarshalJX decodes ", plainType, " from JSON using jx.Decoder")
	gf.P("// Populates Src_ with indices of decoded fields")
	gf.P("func (p *", plainType, ") UnmarshalJX(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ") error {")
	gf.P("\tif p == nil {")
	gf.P("\t\treturn nil")
	gf.P("\t}")
	gf.P()
	gf.P("\treturn d.Obj(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ", key string) error {")
	gf.P("\t\tswitch key {")

	// Handle _src field
	gf.P("\t\tcase \"_src\":")
	gf.P("\t\t\treturn d.Arr(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ") error {")
	gf.P("\t\t\t\tv, err := d.UInt16()")
	gf.P("\t\t\t\tif err != nil { return err }")
	gf.P("\t\t\t\tp.Src_ = append(p.Src_, v)")
	gf.P("\t\t\t\treturn nil")
	gf.P("\t\t\t})")

	// Generate oneof case field decodings
	for _, eo := range msg.EmbeddedOneofs {
		gf.P("\t\tcase \"", eo.JSONName, "\":")
		gf.P("\t\t\tv, err := d.Str()")
		gf.P("\t\t\tif err != nil { return err }")
		gf.P("\t\t\tp.", eo.CaseFieldName, " = v")
	}

	// Generate field decodings
	for _, field := range msg.Fields {
		g.generateUnmarshalJXFieldWithSrc(gf, field, f)
	}

	gf.P("\t\tdefault:")
	gf.P("\t\t\treturn d.Skip()")
	gf.P("\t\t}")
	gf.P("\t\treturn nil")
	gf.P("\t})")
	gf.P("}")
	gf.P()

	// Generate helper method
	gf.P("// UnmarshalJSON implements json.Unmarshaler using jx")
	gf.P("func (p *", plainType, ") UnmarshalJSON(data []byte) error {")
	gf.P("\td := ", gf.QualifiedGoIdent(jxPkg.Ident("DecodeBytes")), "(data)")
	gf.P("\treturn p.UnmarshalJX(d)")
	gf.P("}")
	gf.P()
}

// generateUnmarshalJXFieldWithSrc generates decoding for a single field and adds to Src_
func (g *Generator) generateUnmarshalJXFieldWithSrc(gf *protogen.GeneratedFile, field *IRField, f *protogen.File) {
	jsonName := field.JSONName
	fieldAccess := "p." + field.GoName

	gf.P("\t\tcase \"", jsonName, "\":")

	// Generate field-specific decoding
	g.generateUnmarshalJXValue(gf, field, fieldAccess, f, "\t\t\t")

	// Add field index to Src_ after successful decode
	gf.P("\t\t\tp.Src_ = append(p.Src_, ", field.Index, ")")
}

// generateUnmarshalJXValue generates value decoding
func (g *Generator) generateUnmarshalJXValue(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string) {
	if field.IsRepeated && !field.IsMap {
		// Array - use err pattern to allow Src_ append after
		gf.P(indent, "if err := d.Arr(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ") error {")
		g.generateUnmarshalJXSingleValue(gf, field, access, f, indent+"\t", true)
		gf.P(indent, "\treturn nil")
		gf.P(indent, "}); err != nil {")
		gf.P(indent, "\treturn err")
		gf.P(indent, "}")
		return
	}

	if field.IsMap {
		g.generateUnmarshalJXMap(gf, field, access, f, indent)
		return
	}

	g.generateUnmarshalJXSingleValue(gf, field, access, f, indent, false)
}

// generateUnmarshalJXSingleValue generates decoding for a single value
func (g *Generator) generateUnmarshalJXSingleValue(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string, isArrayElem bool) {
	switch field.Kind {
	case KindScalar:
		g.generateUnmarshalJXScalar(gf, field, access, f, indent, isArrayElem)
	case KindMessage:
		// Check if the message type has generate=true (has UnmarshalJX)
		hasUnmarshalJX := false
		if field.Source != nil && field.Source.Message != nil {
			msgOpts := g.getMessageOptions(field.Source.Message)
			hasUnmarshalJX = msgOpts != nil && msgOpts.Generate
		}

		if hasUnmarshalJX {
			// Plain type - has UnmarshalJX
			if isArrayElem {
				gf.P(indent, "var v ", field.GoType.Name)
				gf.P(indent, "if err := v.UnmarshalJX(d); err != nil {")
				gf.P(indent, "\treturn err")
				gf.P(indent, "}")
				if field.GoType.IsPointer {
					gf.P(indent, access, " = append(", access, ", &v)")
				} else {
					gf.P(indent, access, " = append(", access, ", v)")
				}
			} else if field.GoType.IsPointer {
				gf.P(indent, access, " = &", field.GoType.Name, "{}")
				gf.P(indent, "if err := ", access, ".UnmarshalJX(d); err != nil {")
				gf.P(indent, "\treturn err")
				gf.P(indent, "}")
			} else {
				gf.P(indent, "if err := ", access, ".UnmarshalJX(d); err != nil {")
				gf.P(indent, "\treturn err")
				gf.P(indent, "}")
			}
		} else {
			// Protobuf type - use protojson
			protojsonPkg := protogen.GoImportPath("google.golang.org/protobuf/encoding/protojson")
			gf.P(indent, "raw, err := d.Raw()")
			gf.P(indent, "if err != nil { return err }")
			typeName := g.qualifyType(gf, field.GoType, f)
			if isArrayElem {
				gf.P(indent, "var v ", typeName)
				gf.P(indent, "if err := ", gf.QualifiedGoIdent(protojsonPkg.Ident("Unmarshal")), "(raw, &v); err != nil {")
				gf.P(indent, "\treturn err")
				gf.P(indent, "}")
				// If slice is []T (not []*T), append value; otherwise append pointer
				if field.GoType.IsPointer {
					gf.P(indent, access, " = append(", access, ", &v)")
				} else {
					gf.P(indent, access, " = append(", access, ", v)")
				}
			} else {
				gf.P(indent, access, " = &", typeName, "{}")
				gf.P(indent, "if err := ", gf.QualifiedGoIdent(protojsonPkg.Ident("Unmarshal")), "(raw, ", access, "); err != nil {")
				gf.P(indent, "\treturn err")
				gf.P(indent, "}")
			}
		}
	case KindEnum:
		enumType := g.qualifyType(gf, field.GoType, f)
		if field.EnumAsString {
			gf.P(indent, "s, err := d.Str()")
			gf.P(indent, "if err != nil { return err }")
			gf.P(indent, "// TODO: parse enum from string")
			gf.P(indent, "_ = s")
		} else {
			gf.P(indent, "v, err := d.Int32()")
			gf.P(indent, "if err != nil { return err }")
			if isArrayElem {
				gf.P(indent, access, " = append(", access, ", ", enumType, "(v))")
			} else {
				gf.P(indent, access, " = ", enumType, "(v)")
			}
		}
	case KindBytes:
		gf.P(indent, "v, err := d.Base64()")
		gf.P(indent, "if err != nil { return err }")
		if isArrayElem {
			gf.P(indent, access, " = append(", access, ", v)")
		} else {
			gf.P(indent, access, " = v")
		}
	default:
		gf.P(indent, "return d.Skip()")
	}
}

// generateUnmarshalJXScalar generates decoding for scalar types
func (g *Generator) generateUnmarshalJXScalar(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string, isArrayElem bool) {
	var decodeCall, varType string

	// For fields with NeedsCaster, decode using original ScalarKind and cast to GoType
	if field.NeedsCaster {
		decodeCall, varType = g.getDecodeCallByKind(field.ScalarKind)
		if decodeCall == "" {
			gf.P(indent, "return d.Skip()")
			return
		}

		gf.P(indent, "v, err := ", decodeCall)
		gf.P(indent, "if err != nil { return err }")

		// Simple type cast for JSON (no external casters) - use qualified type name
		qualifiedType := g.qualifyType(gf, field.GoType, f)
		castExpr := qualifiedType + "(v)"
		if isArrayElem {
			gf.P(indent, access, " = append(", access, ", ", castExpr, ")")
		} else if field.GoType.IsPointer {
			gf.P(indent, "_tmp := ", castExpr)
			gf.P(indent, access, " = &_tmp")
		} else {
			gf.P(indent, access, " = ", castExpr)
		}
		return
	}

	switch field.GoType.Name {
	case "string":
		decodeCall = "d.Str()"
		varType = "string"
	case "bool":
		decodeCall = "d.Bool()"
		varType = "bool"
	case "int32":
		decodeCall = "d.Int32()"
		varType = "int32"
	case "int64":
		decodeCall = "d.Int64()"
		varType = "int64"
	case "uint32":
		decodeCall = "d.UInt32()"
		varType = "uint32"
	case "uint64":
		decodeCall = "d.UInt64()"
		varType = "uint64"
	case "float32":
		decodeCall = "d.Float32()"
		varType = "float32"
	case "float64":
		decodeCall = "d.Float64()"
		varType = "float64"
	case "byte":
		if field.GoType.IsSlice {
			gf.P(indent, "v, err := d.Base64()")
			gf.P(indent, "if err != nil { return err }")
			gf.P(indent, access, " = v")
			return
		}
		decodeCall = "d.UInt32()"
		varType = "byte"
	default:
		gf.P(indent, "return d.Skip()")
		return
	}

	gf.P(indent, "v, err := ", decodeCall)
	gf.P(indent, "if err != nil { return err }")

	if isArrayElem {
		if varType == "byte" {
			gf.P(indent, access, " = append(", access, ", byte(v))")
		} else {
			gf.P(indent, access, " = append(", access, ", v)")
		}
	} else if field.GoType.IsPointer {
		gf.P(indent, access, " = &v")
	} else {
		gf.P(indent, access, " = v")
	}
}

// getDecodeCallByKind returns decoder call and variable type for protoreflect.Kind
func (g *Generator) getDecodeCallByKind(kind protoreflect.Kind) (string, string) {
	switch kind {
	case protoreflect.StringKind:
		return "d.Str()", "string"
	case protoreflect.BoolKind:
		return "d.Bool()", "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "d.Int32()", "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "d.Int64()", "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "d.UInt32()", "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "d.UInt64()", "uint64"
	case protoreflect.FloatKind:
		return "d.Float32()", "float32"
	case protoreflect.DoubleKind:
		return "d.Float64()", "float64"
	default:
		return "", ""
	}
}

// generateUnmarshalJXMap generates decoding for map types
func (g *Generator) generateUnmarshalJXMap(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string) {
	keyType := "string"
	if field.MapKey != nil {
		keyType = field.MapKey.GoType.Name
	}
	valueType := "any"
	if field.MapValue != nil {
		valueType = field.MapValue.GoType.Name
		if field.MapValue.GoType.IsPointer {
			valueType = "*" + valueType
		}
	}

	gf.P(indent, "if ", access, " == nil {")
	gf.P(indent, "\t", access, " = make(map[", keyType, "]", valueType, ")")
	gf.P(indent, "}")
	gf.P(indent, "return d.Obj(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ", key string) error {")

	if field.MapValue != nil {
		g.generateUnmarshalJXSingleValue(gf, field.MapValue, access+"[key]", f, indent+"\t", false)
	} else {
		gf.P(indent, "\treturn d.Skip()")
	}

	gf.P(indent, "\treturn nil")
	gf.P(indent, "})")
}
