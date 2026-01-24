package generator

import (
	"google.golang.org/protobuf/compiler/protogen"
)

// jx package path
var jxPkg = protogen.GoImportPath("github.com/go-faster/jx")

// generateJSONMethods generates MarshalJX and UnmarshalJX methods
func (g *Generator) generateJSONMethods(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	g.generateMarshalJX(gf, msg, f)
	g.generateUnmarshalJX(gf, msg, f)
}

// generateMarshalJX generates method for JSON encoding with jx
func (g *Generator) generateMarshalJX(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	plainType := msg.GoName

	gf.P("// MarshalJX encodes ", plainType, " to JSON using jx.Encoder")
	gf.P("func (p *", plainType, ") MarshalJX(e *", gf.QualifiedGoIdent(jxPkg.Ident("Encoder")), ") {")
	gf.P("\tif p == nil {")
	gf.P("\t\te.Null()")
	gf.P("\t\treturn")
	gf.P("\t}")
	gf.P()
	gf.P("\te.ObjStart()")
	gf.P()

	// Generate oneof case field encodings first
	for _, eo := range msg.EmbeddedOneofs {
		gf.P("\tif p.", eo.CaseFieldName, " != \"\" {")
		gf.P("\t\te.FieldStart(\"", eo.JSONName, "\")")
		gf.P("\t\te.Str(p.", eo.CaseFieldName, ")")
		gf.P("\t}")
	}

	// Generate field encodings
	for _, field := range msg.Fields {
		g.generateMarshalJXField(gf, field, f)
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

// generateMarshalJXField generates encoding for a single field
func (g *Generator) generateMarshalJXField(gf *protogen.GeneratedFile, field *IRField, f *protogen.File) {
	fieldAccess := "p." + field.GoName
	jsonName := field.JSONName

	// Handle optional/pointer fields with omitempty
	if field.GoType.IsPointer {
		gf.P("\tif ", fieldAccess, " != nil {")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t\t")
		gf.P("\t}")
	} else if field.IsRepeated {
		gf.P("\tif len(", fieldAccess, ") > 0 {")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t\t")
		gf.P("\t}")
	} else if field.IsOptional && field.GoType.Name == "string" {
		// Optional string - check for empty
		gf.P("\tif ", fieldAccess, " != \"\" {")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t\t")
		gf.P("\t}")
	} else {
		// Always include non-optional scalar fields
		gf.P("\te.FieldStart(\"", jsonName, "\")")
		g.generateMarshalJXValue(gf, field, fieldAccess, f, "\t")
	}
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
	switch field.Kind {
	case KindScalar:
		g.generateMarshalJXScalar(gf, field, access, indent)
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
	gf.P("func (p *", plainType, ") UnmarshalJX(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ") error {")
	gf.P("\tif p == nil {")
	gf.P("\t\treturn nil")
	gf.P("\t}")
	gf.P()
	gf.P("\treturn d.Obj(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ", key string) error {")
	gf.P("\t\tswitch key {")

	// Generate oneof case field decodings
	for _, eo := range msg.EmbeddedOneofs {
		gf.P("\t\tcase \"", eo.JSONName, "\":")
		gf.P("\t\t\tv, err := d.Str()")
		gf.P("\t\t\tif err != nil { return err }")
		gf.P("\t\t\tp.", eo.CaseFieldName, " = v")
	}

	// Generate field decodings
	for _, field := range msg.Fields {
		g.generateUnmarshalJXField(gf, field, f)
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

// generateUnmarshalJXField generates decoding for a single field
func (g *Generator) generateUnmarshalJXField(gf *protogen.GeneratedFile, field *IRField, f *protogen.File) {
	jsonName := field.JSONName
	fieldAccess := "p." + field.GoName

	gf.P("\t\tcase \"", jsonName, "\":")
	g.generateUnmarshalJXValue(gf, field, fieldAccess, f, "\t\t\t")
}

// generateUnmarshalJXValue generates value decoding
func (g *Generator) generateUnmarshalJXValue(gf *protogen.GeneratedFile, field *IRField, access string, f *protogen.File, indent string) {
	if field.IsRepeated && !field.IsMap {
		// Array
		gf.P(indent, "return d.Arr(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ") error {")
		g.generateUnmarshalJXSingleValue(gf, field, access, f, indent+"\t", true)
		gf.P(indent, "\treturn nil")
		gf.P(indent, "})")
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
		g.generateUnmarshalJXScalar(gf, field, access, indent, isArrayElem)
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
				gf.P(indent, "return ", access, ".UnmarshalJX(d)")
			} else {
				gf.P(indent, "return ", access, ".UnmarshalJX(d)")
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
func (g *Generator) generateUnmarshalJXScalar(gf *protogen.GeneratedFile, field *IRField, access string, indent string, isArrayElem bool) {
	var decodeCall, varType string

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
