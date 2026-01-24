package generator

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// generatePbJXMethods generates MarshalJX/UnmarshalJX for original protobuf message
func (g *Generator) generatePbJXMethods(gf *protogen.GeneratedFile, msg *protogen.Message, f *protogen.File) {
	g.generatePbMarshalJX(gf, msg, f)
	g.generatePbUnmarshalJX(gf, msg, f)
}

// generatePbMarshalJX generates MarshalJX for protobuf message
func (g *Generator) generatePbMarshalJX(gf *protogen.GeneratedFile, msg *protogen.Message, f *protogen.File) {
	typeName := msg.GoIdent.GoName

	gf.P("// MarshalJX encodes ", typeName, " to JSON using jx.Encoder")
	gf.P("func (p *", typeName, ") MarshalJX(e *", gf.QualifiedGoIdent(jxPkg.Ident("Encoder")), ") {")
	gf.P("\tif p == nil {")
	gf.P("\t\te.Null()")
	gf.P("\t\treturn")
	gf.P("\t}")
	gf.P()
	gf.P("\te.ObjStart()")

	// Generate field encodings
	for _, field := range msg.Fields {
		// Skip oneof wrapper fields - they're handled via GetXxx methods
		if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
			continue
		}
		g.generatePbMarshalJXField(gf, field, f)
	}

	// Generate oneof field encodings
	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue // synthetic oneofs are proto3 optional
		}
		g.generatePbMarshalJXOneof(gf, oneof, f)
	}

	gf.P("\te.ObjEnd()")
	gf.P("}")
	gf.P()
}

// generatePbMarshalJXField generates encoding for a single protobuf field
func (g *Generator) generatePbMarshalJXField(gf *protogen.GeneratedFile, field *protogen.Field, f *protogen.File) {
	fieldAccess := "p.Get" + field.GoName + "()"
	jsonName := string(field.Desc.JSONName())

	// Handle different field types
	if field.Desc.IsMap() {
		gf.P("\tif len(", fieldAccess, ") > 0 {")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		g.generatePbMarshalJXMapValue(gf, field, fieldAccess, f, "\t\t")
		gf.P("\t}")
		return
	}

	if field.Desc.IsList() {
		gf.P("\tif len(", fieldAccess, ") > 0 {")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		gf.P("\t\te.ArrStart()")
		gf.P("\t\tfor _, v := range ", fieldAccess, " {")
		g.generatePbMarshalJXSingleValue(gf, field, "v", f, "\t\t\t")
		gf.P("\t\t}")
		gf.P("\t\te.ArrEnd()")
		gf.P("\t}")
		return
	}

	// Scalar/message fields
	if field.Message != nil {
		gf.P("\tif ", fieldAccess, " != nil {")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		g.generatePbMarshalJXSingleValue(gf, field, fieldAccess, f, "\t\t")
		gf.P("\t}")
	} else if field.Desc.HasOptionalKeyword() {
		// Optional scalar - check pointer field directly
		gf.P("\tif p.", field.GoName, " != nil {")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		if field.Enum != nil {
			gf.P("\t\te.Int32(int32(*p.", field.GoName, "))")
		} else {
			g.generatePbMarshalJXScalarValue(gf, field.Desc.Kind(), "*p."+field.GoName, "\t\t")
		}
		gf.P("\t}")
	} else {
		// Required scalar - check for zero value
		zeroCheck := g.getZeroCheck(field, fieldAccess)
		if zeroCheck != "" {
			gf.P("\tif ", zeroCheck, " {")
			gf.P("\t\te.FieldStart(\"", jsonName, "\")")
			g.generatePbMarshalJXSingleValue(gf, field, fieldAccess, f, "\t\t")
			gf.P("\t}")
		} else {
			gf.P("\te.FieldStart(\"", jsonName, "\")")
			g.generatePbMarshalJXSingleValue(gf, field, fieldAccess, f, "\t")
		}
	}
}

// generatePbMarshalJXOneof generates encoding for oneof fields
func (g *Generator) generatePbMarshalJXOneof(gf *protogen.GeneratedFile, oneof *protogen.Oneof, f *protogen.File) {
	// Use type switch for oneof
	gf.P("\tswitch v := p.Get", oneof.GoName, "().(type) {")
	for _, field := range oneof.Fields {
		jsonName := string(field.Desc.JSONName())
		wrapperType := gf.QualifiedGoIdent(field.GoIdent)
		gf.P("\tcase *", wrapperType, ":")
		gf.P("\t\te.FieldStart(\"", jsonName, "\")")
		if field.Message != nil {
			gf.P("\t\tv.", field.GoName, ".MarshalJX(e)")
		} else {
			g.generatePbMarshalJXScalarValue(gf, field.Desc.Kind(), "v."+field.GoName, "\t\t")
		}
	}
	gf.P("\t}")
}

// generatePbMarshalJXSingleValue generates encoding for a single value
func (g *Generator) generatePbMarshalJXSingleValue(gf *protogen.GeneratedFile, field *protogen.Field, access string, f *protogen.File, indent string) {
	if field.Message != nil {
		// Check for well-known types
		if g.isWellKnownType(field.Message) {
			g.generatePbMarshalJXWellKnown(gf, field, access, f, indent)
			return
		}
		// Regular message - call MarshalJX
		gf.P(indent, access, ".MarshalJX(e)")
		return
	}

	if field.Enum != nil {
		gf.P(indent, "e.Int32(int32(", access, "))")
		return
	}

	// Scalar
	g.generatePbMarshalJXScalarValue(gf, field.Desc.Kind(), access, indent)
}

// generatePbMarshalJXScalarValue generates encoding for scalar by kind
func (g *Generator) generatePbMarshalJXScalarValue(gf *protogen.GeneratedFile, kind protoreflect.Kind, access string, indent string) {
	switch kind {
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
	case protoreflect.StringKind:
		gf.P(indent, "e.Str(", access, ")")
	case protoreflect.BytesKind:
		gf.P(indent, "e.Base64(", access, ")")
	default:
		gf.P(indent, "e.Null() // unsupported kind: ", kind)
	}
}

// generatePbMarshalJXMapValue generates encoding for map field
func (g *Generator) generatePbMarshalJXMapValue(gf *protogen.GeneratedFile, field *protogen.Field, access string, f *protogen.File, indent string) {
	keyField := field.Message.Fields[0]
	valueField := field.Message.Fields[1]

	gf.P(indent, "e.ObjStart()")
	gf.P(indent, "for k, v := range ", access, " {")

	// Key encoding
	switch keyField.Desc.Kind() {
	case protoreflect.StringKind:
		gf.P(indent, "\te.FieldStart(k)")
	default:
		fmtPkg := protogen.GoImportPath("fmt")
		gf.P(indent, "\te.FieldStart(", gf.QualifiedGoIdent(fmtPkg.Ident("Sprint")), "(k))")
	}

	// Value encoding
	g.generatePbMarshalJXSingleValue(gf, valueField, "v", f, indent+"\t")

	gf.P(indent, "}")
	gf.P(indent, "e.ObjEnd()")
}

// generatePbMarshalJXWellKnown generates encoding for well-known types
func (g *Generator) generatePbMarshalJXWellKnown(gf *protogen.GeneratedFile, field *protogen.Field, access string, f *protogen.File, indent string) {
	fullName := string(field.Message.Desc.FullName())

	switch fullName {
	case "google.protobuf.Timestamp":
		gf.P(indent, "if ", access, " != nil {")
		gf.P(indent, "\te.Str(", access, ".AsTime().Format(\"2006-01-02T15:04:05.999999999Z07:00\"))")
		gf.P(indent, "} else {")
		gf.P(indent, "\te.Null()")
		gf.P(indent, "}")
	case "google.protobuf.Duration":
		gf.P(indent, "if ", access, " != nil {")
		gf.P(indent, "\te.Str(", access, ".AsDuration().String())")
		gf.P(indent, "} else {")
		gf.P(indent, "\te.Null()")
		gf.P(indent, "}")
	case "google.protobuf.StringValue":
		gf.P(indent, "if ", access, " != nil {")
		gf.P(indent, "\te.Str(", access, ".GetValue())")
		gf.P(indent, "} else {")
		gf.P(indent, "\te.Null()")
		gf.P(indent, "}")
	case "google.protobuf.BoolValue":
		gf.P(indent, "if ", access, " != nil {")
		gf.P(indent, "\te.Bool(", access, ".GetValue())")
		gf.P(indent, "} else {")
		gf.P(indent, "\te.Null()")
		gf.P(indent, "}")
	case "google.protobuf.Int32Value", "google.protobuf.Int64Value",
		"google.protobuf.UInt32Value", "google.protobuf.UInt64Value",
		"google.protobuf.FloatValue", "google.protobuf.DoubleValue":
		// Use protojson for numeric wrappers
		protojsonPkg := protogen.GoImportPath("google.golang.org/protobuf/encoding/protojson")
		gf.P(indent, "if data, err := ", gf.QualifiedGoIdent(protojsonPkg.Ident("Marshal")), "(", access, "); err == nil {")
		gf.P(indent, "\te.Raw(data)")
		gf.P(indent, "} else {")
		gf.P(indent, "\te.Null()")
		gf.P(indent, "}")
	default:
		// Unknown well-known type, use MarshalJX
		gf.P(indent, access, ".MarshalJX(e)")
	}
}

// generatePbUnmarshalJX generates UnmarshalJX for protobuf message
func (g *Generator) generatePbUnmarshalJX(gf *protogen.GeneratedFile, msg *protogen.Message, f *protogen.File) {
	typeName := msg.GoIdent.GoName

	gf.P("// UnmarshalJX decodes ", typeName, " from JSON using jx.Decoder")
	gf.P("func (p *", typeName, ") UnmarshalJX(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ") error {")
	gf.P("\tif p == nil {")
	gf.P("\t\treturn nil")
	gf.P("\t}")
	gf.P()
	gf.P("\treturn d.Obj(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ", key string) error {")
	gf.P("\t\tswitch key {")

	// Generate field decodings
	for _, field := range msg.Fields {
		// Skip oneof wrapper fields
		if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
			continue
		}
		g.generatePbUnmarshalJXField(gf, field, f)
	}

	// Generate oneof field decodings
	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue
		}
		for _, field := range oneof.Fields {
			g.generatePbUnmarshalJXOneofField(gf, field, oneof, f)
		}
	}

	gf.P("\t\tdefault:")
	gf.P("\t\t\treturn d.Skip()")
	gf.P("\t\t}")
	gf.P("\t\treturn nil")
	gf.P("\t})")
	gf.P("}")
	gf.P()
}

// generatePbUnmarshalJXField generates decoding for a single field
func (g *Generator) generatePbUnmarshalJXField(gf *protogen.GeneratedFile, field *protogen.Field, f *protogen.File) {
	jsonName := string(field.Desc.JSONName())
	fieldAccess := "p." + field.GoName

	gf.P("\t\tcase \"", jsonName, "\":")

	if field.Desc.IsMap() {
		g.generatePbUnmarshalJXMap(gf, field, fieldAccess, f, "\t\t\t")
		return
	}

	if field.Desc.IsList() {
		gf.P("\t\t\treturn d.Arr(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ") error {")
		g.generatePbUnmarshalJXArrayElem(gf, field, fieldAccess, f, "\t\t\t\t")
		gf.P("\t\t\t\treturn nil")
		gf.P("\t\t\t})")
		return
	}

	g.generatePbUnmarshalJXSingleValue(gf, field, fieldAccess, f, "\t\t\t", false)
}

// generatePbUnmarshalJXOneofField generates decoding for oneof field
func (g *Generator) generatePbUnmarshalJXOneofField(gf *protogen.GeneratedFile, field *protogen.Field, oneof *protogen.Oneof, f *protogen.File) {
	jsonName := string(field.Desc.JSONName())

	gf.P("\t\tcase \"", jsonName, "\":")

	if field.Message != nil {
		msgType := gf.QualifiedGoIdent(field.Message.GoIdent)
		wrapperType := gf.QualifiedGoIdent(field.GoIdent)
		gf.P("\t\t\tv := &", msgType, "{}")
		gf.P("\t\t\tif err := v.UnmarshalJX(d); err != nil {")
		gf.P("\t\t\t\treturn err")
		gf.P("\t\t\t}")
		gf.P("\t\t\tp.", oneof.GoName, " = &", wrapperType, "{", field.GoName, ": v}")
	} else {
		// Scalar oneof field
		g.generatePbUnmarshalJXScalarOneof(gf, field, oneof, f, "\t\t\t")
	}
}

// generatePbUnmarshalJXScalarOneof generates decoding for scalar oneof field
func (g *Generator) generatePbUnmarshalJXScalarOneof(gf *protogen.GeneratedFile, field *protogen.Field, oneof *protogen.Oneof, f *protogen.File, indent string) {
	wrapperType := gf.QualifiedGoIdent(field.GoIdent)

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		gf.P(indent, "v, err := d.Bool()")
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		gf.P(indent, "v, err := d.Int32()")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		gf.P(indent, "v, err := d.Int64()")
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		gf.P(indent, "v, err := d.UInt32()")
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		gf.P(indent, "v, err := d.UInt64()")
	case protoreflect.FloatKind:
		gf.P(indent, "v, err := d.Float32()")
	case protoreflect.DoubleKind:
		gf.P(indent, "v, err := d.Float64()")
	case protoreflect.StringKind:
		gf.P(indent, "v, err := d.Str()")
	default:
		gf.P(indent, "return d.Skip()")
		return
	}

	gf.P(indent, "if err != nil { return err }")
	gf.P(indent, "p.", oneof.GoName, " = &", wrapperType, "{", field.GoName, ": v}")
}

// generatePbUnmarshalJXSingleValue generates decoding for a single value
func (g *Generator) generatePbUnmarshalJXSingleValue(gf *protogen.GeneratedFile, field *protogen.Field, access string, f *protogen.File, indent string, isArrayElem bool) {
	if field.Message != nil {
		// Check for well-known types
		if g.isWellKnownType(field.Message) {
			g.generatePbUnmarshalJXWellKnown(gf, field, access, f, indent, isArrayElem)
			return
		}

		msgType := gf.QualifiedGoIdent(field.Message.GoIdent)
		if isArrayElem {
			gf.P(indent, "v := &", msgType, "{}")
			gf.P(indent, "if err := v.UnmarshalJX(d); err != nil {")
			gf.P(indent, "\treturn err")
			gf.P(indent, "}")
			gf.P(indent, access, " = append(", access, ", v)")
		} else {
			gf.P(indent, access, " = &", msgType, "{}")
			gf.P(indent, "if err := ", access, ".UnmarshalJX(d); err != nil {")
			gf.P(indent, "\treturn err")
			gf.P(indent, "}")
		}
		return
	}

	if field.Enum != nil {
		enumType := gf.QualifiedGoIdent(field.Enum.GoIdent)
		gf.P(indent, "v, err := d.Int32()")
		gf.P(indent, "if err != nil { return err }")
		if isArrayElem {
			gf.P(indent, access, " = append(", access, ", ", enumType, "(v))")
		} else if field.Desc.HasOptionalKeyword() {
			gf.P(indent, "_ev := ", enumType, "(v)")
			gf.P(indent, access, " = &_ev")
		} else {
			gf.P(indent, access, " = ", enumType, "(v)")
		}
		return
	}

	// Scalar
	g.generatePbUnmarshalJXScalarValue(gf, field, access, indent, isArrayElem)
}

// generatePbUnmarshalJXScalarValue generates decoding for scalar field
func (g *Generator) generatePbUnmarshalJXScalarValue(gf *protogen.GeneratedFile, field *protogen.Field, access string, indent string, isArrayElem bool) {
	var decodeCall string
	isOptional := field.Desc.HasOptionalKeyword()

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		decodeCall = "d.Bool()"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		decodeCall = "d.Int32()"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		decodeCall = "d.Int64()"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		decodeCall = "d.UInt32()"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		decodeCall = "d.UInt64()"
	case protoreflect.FloatKind:
		decodeCall = "d.Float32()"
	case protoreflect.DoubleKind:
		decodeCall = "d.Float64()"
	case protoreflect.StringKind:
		decodeCall = "d.Str()"
	case protoreflect.BytesKind:
		gf.P(indent, "v, err := d.Base64()")
		gf.P(indent, "if err != nil { return err }")
		if isArrayElem {
			gf.P(indent, access, " = append(", access, ", v)")
		} else {
			gf.P(indent, access, " = v")
		}
		return
	default:
		gf.P(indent, "return d.Skip()")
		return
	}

	gf.P(indent, "v, err := ", decodeCall)
	gf.P(indent, "if err != nil { return err }")

	if isArrayElem {
		gf.P(indent, access, " = append(", access, ", v)")
	} else if isOptional {
		gf.P(indent, access, " = &v")
	} else {
		gf.P(indent, access, " = v")
	}
}

// generatePbUnmarshalJXArrayElem generates decoding for array element
func (g *Generator) generatePbUnmarshalJXArrayElem(gf *protogen.GeneratedFile, field *protogen.Field, access string, f *protogen.File, indent string) {
	g.generatePbUnmarshalJXSingleValue(gf, field, access, f, indent, true)
}

// generatePbUnmarshalJXMap generates decoding for map field
func (g *Generator) generatePbUnmarshalJXMap(gf *protogen.GeneratedFile, field *protogen.Field, access string, f *protogen.File, indent string) {
	keyField := field.Message.Fields[0]
	valueField := field.Message.Fields[1]

	keyType := g.pbGoTypeName(keyField)
	valueType := g.pbGoTypeName(valueField)
	if valueField.Message != nil {
		valueType = "*" + gf.QualifiedGoIdent(valueField.Message.GoIdent)
	}

	gf.P(indent, "if ", access, " == nil {")
	gf.P(indent, "\t", access, " = make(map[", keyType, "]", valueType, ")")
	gf.P(indent, "}")
	gf.P(indent, "return d.Obj(func(d *", gf.QualifiedGoIdent(jxPkg.Ident("Decoder")), ", key string) error {")

	// Convert key if needed
	keyAccess := "key"
	if keyType != "string" {
		strconvPkg := protogen.GoImportPath("strconv")
		switch keyField.Desc.Kind() {
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
			gf.P(indent, "\tkeyInt, err := ", gf.QualifiedGoIdent(strconvPkg.Ident("ParseInt")), "(key, 10, 32)")
			gf.P(indent, "\tif err != nil { return err }")
			keyAccess = "int32(keyInt)"
		case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			gf.P(indent, "\tkeyInt, err := ", gf.QualifiedGoIdent(strconvPkg.Ident("ParseInt")), "(key, 10, 64)")
			gf.P(indent, "\tif err != nil { return err }")
			keyAccess = "keyInt"
		case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
			gf.P(indent, "\tkeyUint, err := ", gf.QualifiedGoIdent(strconvPkg.Ident("ParseUint")), "(key, 10, 32)")
			gf.P(indent, "\tif err != nil { return err }")
			keyAccess = "uint32(keyUint)"
		case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
			gf.P(indent, "\tkeyUint, err := ", gf.QualifiedGoIdent(strconvPkg.Ident("ParseUint")), "(key, 10, 64)")
			gf.P(indent, "\tif err != nil { return err }")
			keyAccess = "keyUint"
		case protoreflect.BoolKind:
			gf.P(indent, "\tkeyBool, err := ", gf.QualifiedGoIdent(strconvPkg.Ident("ParseBool")), "(key)")
			gf.P(indent, "\tif err != nil { return err }")
			keyAccess = "keyBool"
		}
	}

	// Decode value
	if valueField.Message != nil {
		msgType := gf.QualifiedGoIdent(valueField.Message.GoIdent)
		gf.P(indent, "\tv := &", msgType, "{}")
		gf.P(indent, "\tif err := v.UnmarshalJX(d); err != nil {")
		gf.P(indent, "\t\treturn err")
		gf.P(indent, "\t}")
		gf.P(indent, "\t", access, "[", keyAccess, "] = v")
	} else {
		// Decode scalar value
		g.generatePbUnmarshalJXMapScalarValue(gf, valueField, access, keyAccess, f, indent+"\t")
	}

	gf.P(indent, "\treturn nil")
	gf.P(indent, "})")
}

// generatePbUnmarshalJXMapScalarValue generates scalar value decoding for map
func (g *Generator) generatePbUnmarshalJXMapScalarValue(gf *protogen.GeneratedFile, field *protogen.Field, mapAccess, keyAccess string, f *protogen.File, indent string) {
	var decodeCall string

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		decodeCall = "d.Bool()"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		decodeCall = "d.Int32()"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		decodeCall = "d.Int64()"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		decodeCall = "d.UInt32()"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		decodeCall = "d.UInt64()"
	case protoreflect.FloatKind:
		decodeCall = "d.Float32()"
	case protoreflect.DoubleKind:
		decodeCall = "d.Float64()"
	case protoreflect.StringKind:
		decodeCall = "d.Str()"
	case protoreflect.BytesKind:
		gf.P(indent, "v, err := d.Base64()")
		gf.P(indent, "if err != nil { return err }")
		gf.P(indent, mapAccess, "[", keyAccess, "] = v")
		return
	default:
		gf.P(indent, "return d.Skip()")
		return
	}

	gf.P(indent, "v, err := ", decodeCall)
	gf.P(indent, "if err != nil { return err }")
	gf.P(indent, mapAccess, "[", keyAccess, "] = v")
}

// generatePbUnmarshalJXWellKnown generates decoding for well-known types
func (g *Generator) generatePbUnmarshalJXWellKnown(gf *protogen.GeneratedFile, field *protogen.Field, access string, f *protogen.File, indent string, isArrayElem bool) {
	fullName := string(field.Message.Desc.FullName())

	// For now, use protojson for well-known types
	protojsonPkg := protogen.GoImportPath("google.golang.org/protobuf/encoding/protojson")
	msgType := gf.QualifiedGoIdent(field.Message.GoIdent)

	switch fullName {
	case "google.protobuf.Timestamp", "google.protobuf.Duration":
		// These have special JSON format, use protojson
		gf.P(indent, "raw, err := d.Raw()")
		gf.P(indent, "if err != nil { return err }")
		if isArrayElem {
			gf.P(indent, "v := &", msgType, "{}")
			gf.P(indent, "if err := ", gf.QualifiedGoIdent(protojsonPkg.Ident("Unmarshal")), "(raw, v); err != nil {")
			gf.P(indent, "\treturn err")
			gf.P(indent, "}")
			gf.P(indent, access, " = append(", access, ", v)")
		} else {
			gf.P(indent, access, " = &", msgType, "{}")
			gf.P(indent, "if err := ", gf.QualifiedGoIdent(protojsonPkg.Ident("Unmarshal")), "(raw, ", access, "); err != nil {")
			gf.P(indent, "\treturn err")
			gf.P(indent, "}")
		}
	default:
		// Use standard message decoding
		if isArrayElem {
			gf.P(indent, "v := &", msgType, "{}")
			gf.P(indent, "if err := v.UnmarshalJX(d); err != nil {")
			gf.P(indent, "\treturn err")
			gf.P(indent, "}")
			gf.P(indent, access, " = append(", access, ", v)")
		} else {
			gf.P(indent, access, " = &", msgType, "{}")
			gf.P(indent, "if err := ", access, ".UnmarshalJX(d); err != nil {")
			gf.P(indent, "\treturn err")
			gf.P(indent, "}")
		}
	}
}

// Helper functions

func (g *Generator) isWellKnownType(msg *protogen.Message) bool {
	fullName := string(msg.Desc.FullName())
	wellKnown := []string{
		"google.protobuf.Timestamp",
		"google.protobuf.Duration",
		"google.protobuf.StringValue",
		"google.protobuf.BoolValue",
		"google.protobuf.Int32Value",
		"google.protobuf.Int64Value",
		"google.protobuf.UInt32Value",
		"google.protobuf.UInt64Value",
		"google.protobuf.FloatValue",
		"google.protobuf.DoubleValue",
		"google.protobuf.BytesValue",
	}
	for _, wk := range wellKnown {
		if fullName == wk {
			return true
		}
	}
	return false
}

func (g *Generator) getZeroCheck(field *protogen.Field, access string) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return access
	case protoreflect.StringKind:
		return access + " != \"\""
	case protoreflect.BytesKind:
		return "len(" + access + ") > 0"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		return access + " != 0"
	default:
		return ""
	}
}

func (g *Generator) pbGoTypeName(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		return "[]byte"
	default:
		return "interface{}"
	}
}
