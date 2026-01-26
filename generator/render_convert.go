package generator

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// generateConversionMethods generates IntoPb() and IntoPlain() methods
func (g *Generator) generateConversionMethods(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File, irFile *IRFile) {
	// Check if message has fields requiring casters
	casterFields := g.collectCasterFields(msg)
	hasCasters := len(casterFields) > 0

	// Generate Casters struct if needed (only when castersAsStruct=true)
	if hasCasters && g.castersAsStruct {
		g.generateCastersStruct(gf, msg, casterFields, f)
	}

	g.generateIntoPlain(gf, msg, f, casterFields, g.castersAsStruct)
	g.generateIntoPb(gf, msg, f, casterFields, g.castersAsStruct)

	// Generate IntoPlainReuse for pool usage (only when pool is enabled and no casters)
	if g.Settings.GeneratePool && !hasCasters {
		g.generateIntoPlainReuse(gf, msg, f)
	}
}

// collectCasterFields returns fields that require casters passed as parameters.
// Fields with existing casters (pre-defined and imported) are excluded.
func (g *Generator) collectCasterFields(msg *IRMessage) []*IRField {
	var result []*IRField
	for _, field := range msg.Fields {
		if field.NeedsCaster {
			// Check if there's an existing caster for this field
			// We need casters for both directions, so check both
			toPlainCaster := g.FindExistingCaster(field.SourceGoType, field.GoType)
			toPbCaster := g.FindExistingCaster(field.GoType, field.SourceGoType)

			// Only include if at least one direction doesn't have existing caster
			if toPlainCaster == nil || toPbCaster == nil {
				result = append(result, field)
			}
		}
	}
	return result
}

// hasExistingCaster checks if field has pre-defined casters for both directions
func (g *Generator) hasExistingCaster(field *IRField) bool {
	if !field.NeedsCaster {
		return false
	}
	toPlainCaster := g.FindExistingCaster(field.SourceGoType, field.GoType)
	toPbCaster := g.FindExistingCaster(field.GoType, field.SourceGoType)
	return toPlainCaster != nil && toPbCaster != nil
}

// generateCastersStruct generates struct with caster fields
func (g *Generator) generateCastersStruct(gf *protogen.GeneratedFile, msg *IRMessage, fields []*IRField, f *protogen.File) {
	castPkg := protogen.GoImportPath("github.com/yaroher/protoc-gen-go-plain/cast")

	gf.P("// ", msg.GoName, "Casters contains type casters for ", msg.GoName)
	gf.P("type ", msg.GoName, "Casters struct {")

	for _, field := range fields {
		srcType := g.qualifyType(gf, field.SourceGoType, f)
		dstType := g.qualifyType(gf, field.GoType, f)

		// ToPlain caster: SourceGoType -> GoType
		gf.P("\t", field.GoName, "ToPlain ", gf.QualifiedGoIdent(castPkg.Ident("Caster")), "[", srcType, ", ", dstType, "]")
		// ToPb caster: GoType -> SourceGoType
		gf.P("\t", field.GoName, "ToPb ", gf.QualifiedGoIdent(castPkg.Ident("Caster")), "[", dstType, ", ", srcType, "]")
	}

	gf.P("}")
	gf.P()
}

// generateCasterArgs generates caster arguments for IntoPlain/IntoPb
// toPlain=true: generate args for IntoPlain (SourceType -> TargetType)
// toPlain=false: generate args for IntoPb (TargetType -> SourceType)
func (g *Generator) generateCasterArgs(gf *protogen.GeneratedFile, fields []*IRField, f *protogen.File, toPlain bool) {
	castPkg := protogen.GoImportPath("github.com/yaroher/protoc-gen-go-plain/cast")

	for _, field := range fields {
		srcType := g.qualifyType(gf, field.SourceGoType, f)
		dstType := g.qualifyType(gf, field.GoType, f)

		var argName, fromType, toType string
		if toPlain {
			argName = g.lowerFirst(field.GoName) + "Caster"
			fromType = srcType
			toType = dstType
		} else {
			argName = g.lowerFirst(field.GoName) + "Caster"
			fromType = dstType
			toType = srcType
		}

		// Always add trailing comma for multi-line Go function params
		gf.P("\t", argName, " ", gf.QualifiedGoIdent(castPkg.Ident("Caster")), "[", fromType, ", ", toType, "],")
	}
}

// lowerFirst returns string with first letter lowercased
func (g *Generator) lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// casterCallWithImport generates caster call and ensures import is added
func (g *Generator) casterCallWithImport(gf *protogen.GeneratedFile, field *IRField, value string, toPlain bool) string {
	// Check for existing caster first
	var existingCaster *ExistingCaster
	if toPlain {
		existingCaster = g.FindExistingCaster(field.SourceGoType, field.GoType)
	} else {
		existingCaster = g.FindExistingCaster(field.GoType, field.SourceGoType)
	}

	if existingCaster != nil && existingCaster.CasterIdent.ImportPath != "" {
		// Use QualifiedGoIdent to ensure import is added
		ident := protogen.GoIdent{
			GoName:       existingCaster.CasterIdent.Name,
			GoImportPath: protogen.GoImportPath(existingCaster.CasterIdent.ImportPath),
		}
		qualifiedName := gf.QualifiedGoIdent(ident)
		if existingCaster.IsFunc {
			return qualifiedName + "(" + value + ")"
		}
		return qualifiedName + ".Cast(" + value + ")"
	}

	// Fall back to parameter-based casters
	if g.castersAsStruct {
		if toPlain {
			return "c." + field.GoName + "ToPlain.Cast(" + value + ")"
		}
		return "c." + field.GoName + "ToPb.Cast(" + value + ")"
	}
	// Separate arguments mode
	return g.lowerFirst(field.GoName) + "Caster.Cast(" + value + ")"
}

// generateIntoPlain generates method to convert protobuf message to plain struct
// func (pb *OriginalMessage) IntoPlain(c *MessageCasters) *MessagePlain (castersAsStruct=true)
// func (pb *OriginalMessage) IntoPlain(field1 cast.Caster[A,B], ...) *MessagePlain (castersAsStruct=false)
func (g *Generator) generateIntoPlain(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File, casterFields []*IRField, castersAsStruct bool) {
	if msg.Source == nil {
		return
	}

	pbType := msg.Source.GoIdent
	plainType := msg.GoName
	hasCasters := len(casterFields) > 0

	gf.P("// IntoPlain converts protobuf message to plain struct")
	if hasCasters {
		if castersAsStruct {
			gf.P("func (pb *", gf.QualifiedGoIdent(pbType), ") IntoPlain(c *", msg.GoName, "Casters) *", plainType, " {")
		} else {
			// Generate separate arguments
			gf.P("func (pb *", gf.QualifiedGoIdent(pbType), ") IntoPlain(")
			g.generateCasterArgs(gf, casterFields, f, true) // toPlain=true
			gf.P(") *", plainType, " {")
		}
	} else {
		gf.P("func (pb *", gf.QualifiedGoIdent(pbType), ") IntoPlain() *", plainType, " {")
	}
	gf.P("\tif pb == nil {")
	gf.P("\t\treturn nil")
	gf.P("\t}")
	gf.P("\tp := &", plainType, "{}")
	gf.P()

	// Generate oneof case detection first
	for _, eo := range msg.EmbeddedOneofs {
		g.generateOneofCaseDetection(gf, eo)
	}

	// Generate field assignments based on origin
	for _, field := range msg.Fields {
		g.generateIntoPlainField(gf, field, msg, f)
	}

	gf.P("\treturn p")
	gf.P("}")
	gf.P()
}

// generateIntoPlainReuse generates IntoPlainReuse method that fills existing Plain object
// This is used with sync.Pool for zero-allocation conversions
func (g *Generator) generateIntoPlainReuse(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	if msg.Source == nil {
		return
	}

	pbType := msg.Source.GoIdent
	plainType := msg.GoName

	gf.P("// IntoPlainReuse converts protobuf message to existing plain struct (for pool usage)")
	gf.P("func (pb *", gf.QualifiedGoIdent(pbType), ") IntoPlainReuse(p *", plainType, ") {")
	gf.P("\tif pb == nil || p == nil {")
	gf.P("\t\treturn")
	gf.P("\t}")
	gf.P("\t// Reset before filling")
	gf.P("\tp.Reset()")
	gf.P()

	// Generate oneof case detection first
	for _, eo := range msg.EmbeddedOneofs {
		g.generateOneofCaseDetection(gf, eo)
	}

	// Generate field conversions (same as IntoPlain but without allocation)
	for _, field := range msg.Fields {
		g.generateIntoPlainField(gf, field, msg, f)
	}

	gf.P("}")
	gf.P()
}

// generateOneofCaseDetection generates code to detect which oneof variant is set
func (g *Generator) generateOneofCaseDetection(gf *protogen.GeneratedFile, eo *EmbeddedOneof) {
	gf.P("\t// Detect ", eo.Name, " oneof case")

	// Build access path: pb.GoName or pb.AccessPath.GoName
	var oneofAccess string
	if eo.AccessPath != "" {
		oneofAccess = "pb." + eo.AccessPath + ".Get" + eo.GoName + "()"
	} else {
		oneofAccess = "pb." + eo.GoName
	}

	gf.P("\tswitch ", oneofAccess, ".(type) {")
	for _, variant := range eo.Variants {
		wrapperIdent := protogen.GoIdent{
			GoName:       eo.Source.Parent.GoIdent.GoName + "_" + variant.GoName,
			GoImportPath: eo.Source.Parent.GoIdent.GoImportPath,
		}
		gf.P("\tcase *", gf.QualifiedGoIdent(wrapperIdent), ":")
		gf.P("\t\tp.", eo.CaseFieldName, " = \"", variant.Name, "\"")
	}
	gf.P("\t}")
	gf.P()
}

// generateIntoPlainField generates code to copy one field from pb to plain
func (g *Generator) generateIntoPlainField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	switch field.Origin {
	case OriginDirect:
		g.generateIntoPlainDirectField(gf, field, msg, f)
	case OriginEmbed, OriginOneofEmbed:
		g.generateIntoPlainEmbedField(gf, field, msg, f)
	case OriginVirtual:
		// Virtual fields have no source in protobuf
		gf.P("\t// ", field.GoName, " is virtual, no source in protobuf")
	case OriginSerialized:
		g.generateIntoPlainSerializedField(gf, field, msg, f)
	case OriginTypeAlias:
		g.generateIntoPlainTypeAliasField(gf, field, msg, f)
	}
}

// generateIntoPlainDirectField handles direct field copy
func (g *Generator) generateIntoPlainDirectField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	if field.Source == nil {
		return
	}

	srcField := "pb." + field.Source.GoName
	dstField := "p." + field.GoName

	// Check if proto field is optional (pointer) but plain field is not
	// Note: bytes type in proto3 is never a pointer, even with optional keyword
	protoIsPointer := (field.Source.Desc.HasOptionalKeyword() || field.Source.Desc.HasPresence()) &&
		field.Source.Desc.Kind() != protoreflect.BytesKind
	// Plain is pointer if GoType.IsPointer OR field is optional (for nullable fields like optional Timestamp -> *time.Time)
	plainIsPointer := field.GoType.IsPointer || (field.IsOptional && !field.GoType.IsSlice && !field.IsRepeated)

	// Check if types match or need conversion
	if field.IsMap && field.MapValue != nil && field.MapValue.Kind == KindMessage {
		// Map with message value - check if value type has generate=true
		msgOpts := g.getMessageOptionsFromField(field.MapValue)
		if msgOpts != nil && msgOpts.Generate {
			// Need to convert each value via IntoPlain()
			keyType := "string"
			if field.MapKey != nil {
				keyType = field.MapKey.GoType.Name
			}
			// Plain map value type (already includes * if pointer)
			valueType := g.buildTypeStringPlain(field.MapValue, f)
			gf.P("\tif len(", srcField, ") > 0 {")
			gf.P("\t\t", dstField, " = make(map[", keyType, "]", valueType, ", len(", srcField, "))")
			gf.P("\t\tfor k, v := range ", srcField, " {")
			gf.P("\t\t\tif v != nil {")
			gf.P("\t\t\t\t", dstField, "[k] = v.IntoPlain()")
			gf.P("\t\t\t}")
			gf.P("\t\t}")
			gf.P("\t}")
		} else {
			// Use original protobuf type
			gf.P("\t", dstField, " = ", srcField)
		}
	} else if field.Kind == KindMessage {
		// Message fields need IntoPlain() call if the nested type has generate=true
		msgOpts := g.getMessageOptionsFromField(field)
		if msgOpts != nil && msgOpts.Generate {
			if field.IsRepeated {
				// Repeated plain: []PlainType (without pointer on element)
				// Source is []*ProtoMessage, IntoPlain() returns *PlainType
				// Need to dereference: *v.IntoPlain()
				gf.P("\tif len(", srcField, ") > 0 {")
				gf.P("\t\t", dstField, " = make([]", g.buildTypeStringPlain(field, f), ", len(", srcField, "))")
				gf.P("\t\tfor i, v := range ", srcField, " {")
				gf.P("\t\t\tif v != nil {")
				gf.P("\t\t\t\t", dstField, "[i] = *v.IntoPlain()")
				gf.P("\t\t\t}")
				gf.P("\t\t}")
				gf.P("\t}")
			} else {
				gf.P("\tif ", srcField, " != nil {")
				gf.P("\t\t", dstField, " = ", srcField, ".IntoPlain()")
				gf.P("\t}")
			}
		} else if field.NeedsCaster {
			// Message with type override (e.g., Timestamp -> time.Time)
			gf.P("\tif ", srcField, " != nil {")
			if plainIsPointer {
				// Plain is pointer (e.g., *time.Time) - need to take address of caster result
				gf.P("\t\t_tmp := ", g.casterCallWithImport(gf, field, srcField, true))
				gf.P("\t\t", dstField, " = &_tmp")
			} else {
				gf.P("\t\t", dstField, " = ", g.casterCallWithImport(gf, field, srcField, true))
			}
			gf.P("\t}")
		} else {
			// Use original protobuf type
			gf.P("\t", dstField, " = ", srcField)
		}
	} else if protoIsPointer && !plainIsPointer {
		// Proto has optional (pointer), plain has value - dereference with nil check
		gf.P("\tif ", srcField, " != nil {")
		if field.NeedsCaster {
			gf.P("\t\t", dstField, " = ", g.casterCallWithImport(gf, field, "*"+srcField, true))
		} else {
			gf.P("\t\t", dstField, " = *", srcField)
		}
		gf.P("\t}")
	} else if !protoIsPointer && plainIsPointer {
		// Proto has value, plain has pointer - take address
		if field.NeedsCaster {
			gf.P("\t_tmp := ", g.casterCallWithImport(gf, field, srcField, true))
			gf.P("\t", dstField, " = &_tmp")
		} else {
			gf.P("\t", dstField, " = &", srcField)
		}
	} else if field.EnumAsString {
		// Enum to string conversion
		gf.P("\t", dstField, " = ", srcField, ".String()")
	} else if field.EnumAsInt {
		// Enum to int32 conversion
		gf.P("\t", dstField, " = int32(", srcField, ")")
	} else {
		// Scalar, enum, bytes - direct copy (types match) or with cast
		if field.NeedsCaster {
			gf.P("\t", dstField, " = ", g.casterCallWithImport(gf, field, srcField, true))
		} else if g.needsTypeCast(field) && !field.GoType.IsSlice && field.Kind != KindBytes {
			// Type override without explicit caster - use simple cast
			// Skip cast for slice types and bytes - direct assignment works
			typeStr := g.qualifyType(gf, field.GoType, f)
			gf.P("\t", dstField, " = ", typeStr, "(", srcField, ")")
		} else {
			gf.P("\t", dstField, " = ", srcField)
		}
	}
}

// generateIntoPlainEmbedField handles embedded field extraction
func (g *Generator) generateIntoPlainEmbedField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	if len(field.PathNumbers) == 0 || msg.Source == nil {
		gf.P("\t// ", field.GoName, ": no path information")
		return
	}

	// Resolve path information
	pathInfo, err := resolvePathInfo(msg.Source, field.PathNumbers)
	if err != nil {
		gf.P("\t// ", field.GoName, ": path resolution error: ", err.Error())
		return
	}

	dstField := "p." + field.GoName
	getterChain := pathInfo.BuildGetterChain("pb")
	nilCheck := pathInfo.BuildNilCheck("pb")

	gf.P("\t// ", field.GoName, " from ", field.EmPath)
	gf.P("\tif ", nilCheck, " {")
	g.generateEmbedFieldAssignment(gf, field, pathInfo, dstField, getterChain)
	gf.P("\t}")

	// Generate alternatives (for fields from other oneof variants)
	for _, alt := range field.OneofAlternatives {
		altPathInfo, err := resolvePathInfo(msg.Source, alt.PathNumbers)
		if err != nil {
			gf.P("\t// ", field.GoName, " alt (", alt.OneofVariant, "): path resolution error: ", err.Error())
			continue
		}
		altGetterChain := altPathInfo.BuildGetterChain("pb")
		altNilCheck := altPathInfo.BuildNilCheck("pb")

		gf.P("\t// ", field.GoName, " from ", alt.EmPath, " (variant: ", alt.OneofVariant, ")")
		gf.P("\tif ", altNilCheck, " {")
		g.generateEmbedFieldAssignment(gf, field, altPathInfo, dstField, altGetterChain)
		gf.P("\t}")
	}
}

// generateEmbedFieldAssignment generates the assignment code for an embedded field
func (g *Generator) generateEmbedFieldAssignment(gf *protogen.GeneratedFile, field *IRField, pathInfo *PathInfo, dstField, getterChain string) {
	leafField := pathInfo.LeafField
	// Plain is pointer if GoType.IsPointer OR field is optional (for nullable fields like optional Timestamp -> *time.Time)
	plainIsPointer := field.GoType.IsPointer || (field.IsOptional && !field.GoType.IsSlice && !field.IsRepeated)

	// Handle different field types
	if field.Kind == KindMessage {
		msgOpts := g.getMessageOptionsFromField(field)
		if msgOpts != nil && msgOpts.Generate {
			// Plain type - call IntoPlain()
			if field.IsRepeated {
				// Repeated message with generate=true
				gf.P("\t\tfor _, v := range ", getterChain, " {")
				gf.P("\t\t\t", dstField, " = append(", dstField, ", *v.IntoPlain())")
				gf.P("\t\t}")
			} else {
				gf.P("\t\t", dstField, " = ", getterChain, ".IntoPlain()")
			}
		} else if field.NeedsCaster {
			// Message with type override (e.g., Timestamp -> time.Time)
			if plainIsPointer {
				// Plain is pointer (e.g., *time.Time) - need to take address of caster result
				gf.P("\t\t_tmp := ", g.casterCallWithImport(gf, field, getterChain, true))
				gf.P("\t\t", dstField, " = &_tmp")
			} else {
				gf.P("\t\t", dstField, " = ", g.casterCallWithImport(gf, field, getterChain, true))
			}
		} else {
			// Protobuf type - check if slice element types match
			if field.IsRepeated && !field.GoType.IsPointer && leafField != nil && leafField.Message != nil {
				// Plain is []T, proto is []*T - need to convert
				gf.P("\t\tfor _, v := range ", getterChain, " {")
				gf.P("\t\t\tif v != nil {")
				gf.P("\t\t\t\t", dstField, " = append(", dstField, ", *v)")
				gf.P("\t\t\t}")
				gf.P("\t\t}")
			} else {
				gf.P("\t\t", dstField, " = ", getterChain)
			}
		}
	} else if field.NeedsCaster {
		// Scalar with type override
		gf.P("\t\t", dstField, " = ", g.casterCallWithImport(gf, field, getterChain, true))
	} else {
		// Scalar, enum, bytes - direct assignment
		// Note: protobuf getters always return values (not pointers) for scalars
		gf.P("\t\t", dstField, " = ", getterChain)
	}

}

// generateIntoPlainSerializedField handles serialized field (message -> bytes)
func (g *Generator) generateIntoPlainSerializedField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	protoPkg := protogen.GoImportPath("google.golang.org/protobuf/proto")

	path := g.buildPbNavigationPath(field, msg)
	dstField := "p." + field.GoName

	gf.P("\t// ", field.GoName, " serialized from ", field.EmPath)
	gf.P("\tif ", path.NilCheck, " {")
	gf.P("\t\tif data, err := ", gf.QualifiedGoIdent(protoPkg.Ident("Marshal")), "(", path.Value, "); err == nil {")
	gf.P("\t\t\t", dstField, " = data")
	gf.P("\t\t}")
	gf.P("\t}")
}

// generateIntoPlainTypeAliasField handles type alias field
func (g *Generator) generateIntoPlainTypeAliasField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	if len(field.PathNumbers) == 0 || msg.Source == nil {
		gf.P("\t// ", field.GoName, " type alias: no path information")
		return
	}

	// Resolve path information using path_nav
	pathInfo, err := resolvePathInfo(msg.Source, field.PathNumbers)
	if err != nil {
		gf.P("\t// ", field.GoName, " type alias: path resolution error: ", err.Error())
		return
	}

	dstField := "p." + field.GoName
	getterChain := pathInfo.BuildGetterChain("pb")
	nilCheck := pathInfo.BuildNilCheck("pb")

	gf.P("\t// ", field.GoName, " type alias from ", field.EmPath)
	gf.P("\tif ", nilCheck, " {")
	gf.P("\t\t", dstField, " = ", getterChain)
	gf.P("\t}")
}

// generateIntoPb generates method to convert plain struct back to protobuf
// func (p *MessagePlain) IntoPb(c *MessageCasters) *OriginalMessage (castersAsStruct=true)
// func (p *MessagePlain) IntoPb(field1 cast.Caster[A,B], ...) *OriginalMessage (castersAsStruct=false)
func (g *Generator) generateIntoPb(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File, casterFields []*IRField, castersAsStruct bool) {
	if msg.Source == nil {
		return
	}

	pbType := msg.Source.GoIdent
	plainType := msg.GoName
	hasCasters := len(casterFields) > 0

	gf.P("// IntoPb converts plain struct to protobuf message")
	if hasCasters {
		if castersAsStruct {
			gf.P("func (p *", plainType, ") IntoPb(c *", msg.GoName, "Casters) *", gf.QualifiedGoIdent(pbType), " {")
		} else {
			// Generate separate arguments
			gf.P("func (p *", plainType, ") IntoPb(")
			g.generateCasterArgs(gf, casterFields, f, false) // toPlain=false
			gf.P(") *", gf.QualifiedGoIdent(pbType), " {")
		}
	} else {
		gf.P("func (p *", plainType, ") IntoPb() *", gf.QualifiedGoIdent(pbType), " {")
	}
	gf.P("\tif p == nil {")
	gf.P("\t\treturn nil")
	gf.P("\t}")
	gf.P("\tpb := &", gf.QualifiedGoIdent(pbType), "{}")
	gf.P()

	// Generate field assignments
	for _, field := range msg.Fields {
		g.generateIntoPbField(gf, field, msg, f)
	}

	gf.P("\treturn pb")
	gf.P("}")
	gf.P()
}

// generateIntoPbField generates code to copy one field from plain to pb
func (g *Generator) generateIntoPbField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	switch field.Origin {
	case OriginDirect:
		g.generateIntoPbDirectField(gf, field, msg, f)
	case OriginEmbed, OriginOneofEmbed:
		g.generateIntoPbEmbedField(gf, field, msg, f)
	case OriginVirtual:
		// Virtual fields don't go back to protobuf
		gf.P("\t// ", field.GoName, " is virtual, skipping")
	case OriginSerialized:
		g.generateIntoPbSerializedField(gf, field, msg, f)
	case OriginTypeAlias:
		g.generateIntoPbTypeAliasField(gf, field, msg, f)
	}
}

// generateIntoPbDirectField handles direct field copy to protobuf
func (g *Generator) generateIntoPbDirectField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	if field.Source == nil {
		return
	}

	srcField := "p." + field.GoName
	dstField := "pb." + field.Source.GoName

	// Check if proto field is optional (pointer) but plain field is not
	// Note: bytes type in proto3 is never a pointer, even with optional keyword
	protoIsPointer := (field.Source.Desc.HasOptionalKeyword() || field.Source.Desc.HasPresence()) &&
		field.Source.Desc.Kind() != protoreflect.BytesKind
	// Plain is pointer if GoType.IsPointer OR field is optional (for nullable fields like optional Timestamp -> *time.Time)
	plainIsPointer := field.GoType.IsPointer || (field.IsOptional && !field.GoType.IsSlice && !field.IsRepeated)

	if field.IsMap && field.MapValue != nil && field.MapValue.Kind == KindMessage {
		// Map with message value - check if value type has generate=true
		msgOpts := g.getMessageOptionsFromField(field.MapValue)
		if msgOpts != nil && msgOpts.Generate {
			// Need to convert each value via IntoPb()
			keyType := "string"
			if field.MapKey != nil {
				keyType = field.MapKey.GoType.Name
			}
			pbValueType := g.buildPbMapValueType(gf, field, f)
			gf.P("\tif len(", srcField, ") > 0 {")
			gf.P("\t\t", dstField, " = make(map[", keyType, "]", pbValueType, ", len(", srcField, "))")
			gf.P("\t\tfor k, v := range ", srcField, " {")
			gf.P("\t\t\tif v != nil {")
			gf.P("\t\t\t\t", dstField, "[k] = v.IntoPb()")
			gf.P("\t\t\t}")
			gf.P("\t\t}")
			gf.P("\t}")
		} else {
			gf.P("\t", dstField, " = ", srcField)
		}
	} else if field.Kind == KindMessage {
		msgOpts := g.getMessageOptionsFromField(field)
		if msgOpts != nil && msgOpts.Generate {
			if field.IsRepeated {
				// Repeated plain: []PlainType (value, not pointer)
				// Need to take address for IntoPb() call: (&srcField[i]).IntoPb()
				gf.P("\tif len(", srcField, ") > 0 {")
				gf.P("\t\t", dstField, " = make(", g.buildPbSliceType(gf, field, f), ", len(", srcField, "))")
				gf.P("\t\tfor i := range ", srcField, " {")
				gf.P("\t\t\t", dstField, "[i] = (&", srcField, "[i]).IntoPb()")
				gf.P("\t\t}")
				gf.P("\t}")
			} else {
				gf.P("\tif ", srcField, " != nil {")
				gf.P("\t\t", dstField, " = ", srcField, ".IntoPb()")
				gf.P("\t}")
			}
		} else if field.NeedsCaster {
			// Message with type override (e.g., time.Time -> Timestamp)
			if plainIsPointer {
				// Plain is pointer (e.g., *time.Time) - need nil check and dereference
				gf.P("\tif ", srcField, " != nil {")
				gf.P("\t\t", dstField, " = ", g.casterCallWithImport(gf, field, "*"+srcField, false))
				gf.P("\t}")
			} else {
				gf.P("\t", dstField, " = ", g.casterCallWithImport(gf, field, srcField, false))
			}
		} else {
			gf.P("\t", dstField, " = ", srcField)
		}
	} else if protoIsPointer && !plainIsPointer {
		// Proto wants pointer, plain has value - take address (with non-zero check for strings)
		switch field.GoType.Name {
		case "string":
			gf.P("\tif ", srcField, " != \"\" {")
			if field.NeedsCaster {
				gf.P("\t\t_tmp := ", g.casterCallWithImport(gf, field, srcField, false))
				gf.P("\t\t", dstField, " = &_tmp")
			} else {
				gf.P("\t\t", dstField, " = &", srcField)
			}
			gf.P("\t}")
		default:
			if field.NeedsCaster {
				gf.P("\t_tmp := ", g.casterCallWithImport(gf, field, srcField, false))
				gf.P("\t", dstField, " = &_tmp")
			} else {
				gf.P("\t", dstField, " = &", srcField)
			}
		}
	} else if !protoIsPointer && plainIsPointer {
		// Proto wants value, plain has pointer - dereference
		gf.P("\tif ", srcField, " != nil {")
		if field.NeedsCaster {
			gf.P("\t\t", dstField, " = ", g.casterCallWithImport(gf, field, "*"+srcField, false))
		} else {
			gf.P("\t\t", dstField, " = *", srcField)
		}
		gf.P("\t}")
	} else if field.EnumAsString {
		// String back to enum conversion
		enumType := g.qualifyType(gf, GoType{
			Name:       field.Source.Enum.GoIdent.GoName,
			ImportPath: string(field.Source.Enum.GoIdent.GoImportPath),
		}, f)
		gf.P("\t", dstField, " = ", enumType, "(", enumType, "_value[", srcField, "])")
	} else if field.EnumAsInt {
		// Int32 back to enum conversion
		enumType := g.qualifyType(gf, GoType{
			Name:       field.Source.Enum.GoIdent.GoName,
			ImportPath: string(field.Source.Enum.GoIdent.GoImportPath),
		}, f)
		gf.P("\t", dstField, " = ", enumType, "(", srcField, ")")
	} else {
		// Direct copy or with cast
		if field.NeedsCaster {
			gf.P("\t", dstField, " = ", g.casterCallWithImport(gf, field, srcField, false))
		} else if g.needsTypeCast(field) && !field.GoType.IsSlice && field.Kind != KindBytes {
			// Type override without explicit caster - use simple cast to source type
			// Skip cast for slice types and bytes - direct assignment works
			srcTypeName := g.getSourceTypeName(field)
			gf.P("\t", dstField, " = ", srcTypeName, "(", srcField, ")")
		} else {
			gf.P("\t", dstField, " = ", srcField)
		}
	}
}

// generateIntoPbEmbedField handles embedded field assignment back to protobuf
func (g *Generator) generateIntoPbEmbedField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	// Special handling for direct oneof fields (PathNumbers is empty, but oneof info present)
	if len(field.PathNumbers) == 0 && field.OneofName != "" && field.OneofVariant != "" && field.Source != nil {
		g.generateIntoPbOneofField(gf, field, msg, f)
		return
	}

	if len(field.PathNumbers) == 0 || msg.Source == nil {
		gf.P("\t// ", field.GoName, ": no path information")
		return
	}

	// Resolve path information
	pathInfo, err := resolvePathInfo(msg.Source, field.PathNumbers)
	if err != nil {
		gf.P("\t// ", field.GoName, ": path resolution error: ", err.Error())
		return
	}

	srcField := "p." + field.GoName
	leafField := pathInfo.LeafField

	// For oneof fields, add case check
	caseCheck := ""
	if field.OneofName != "" && field.OneofVariant != "" {
		caseFieldName := field.OneofGoName + "Case"
		caseCheck = fmt.Sprintf(" && p.%s == %q", caseFieldName, field.OneofVariant)
	}

	gf.P("\t// ", field.GoName, " -> ", field.EmPath)

	// Check if proto field is optional (pointer) but plain is value
	// For oneof scalar fields, proto does NOT use pointer (wrapper contains value directly)
	isOneofScalar := leafField != nil && leafField.Oneof != nil && !leafField.Oneof.Desc.IsSynthetic() && leafField.Message == nil
	protoIsPointer := leafField != nil && !isOneofScalar && (leafField.Desc.HasOptionalKeyword() || leafField.Desc.HasPresence())
	// Plain is pointer if GoType.IsPointer OR field is optional (for nullable fields like optional Timestamp -> *time.Time)
	plainIsPointer := field.GoType.IsPointer || (field.IsOptional && !field.GoType.IsSlice && !field.IsRepeated)

	// Determine if value needs conversion
	valueExpr := srcField
	valueIsPointer := field.GoType.IsPointer

	// Handle type conversions
	if field.Kind == KindMessage {
		msgOpts := g.getMessageOptionsFromField(field)
		if msgOpts != nil && msgOpts.Generate {
			if field.IsRepeated {
				// Repeated message - need to handle slice conversion
				// This is complex for embedded fields, generate helper code
				initCode, _ := pathInfo.BuildSetterCode(gf, "pb", "nil", true)

				// Add case check for oneof fields
				if caseCheck != "" {
					gf.P("\tif p.", field.OneofGoName, "Case == \"", field.OneofVariant, "\" {")
					if initCode != "" {
						gf.P(initCode)
					}
					gf.P("\t\t// TODO: repeated message conversion for embedded field")
					gf.P("\t}")
				} else {
					if initCode != "" {
						gf.P(initCode)
					}
					gf.P("\t// TODO: repeated message conversion for embedded field")
				}
				return
			}
			// Plain type - call IntoPb()
			valueExpr = srcField + ".IntoPb()"
			valueIsPointer = true // IntoPb returns pointer
		} else if field.IsRepeated && !field.GoType.IsPointer && leafField != nil && leafField.Message != nil {
			// Plain is []T, proto is []*T - need to convert
			initCode, _ := pathInfo.BuildSetterCode(gf, "pb", "nil", true)
			elemType := gf.QualifiedGoIdent(leafField.Message.GoIdent)
			getterChain := pathInfo.BuildGetterChain("pb")

			// Add case check for oneof fields
			if caseCheck != "" {
				gf.P("\tif p.", field.OneofGoName, "Case == \"", field.OneofVariant, "\" && len(", srcField, ") > 0 {")
			} else {
				gf.P("\tif len(", srcField, ") > 0 {")
			}
			if initCode != "" {
				gf.P(initCode)
			}
			gf.P("\t\tfor _, v := range ", srcField, " {")
			gf.P("\t\t\tvCopy := v")
			gf.P("\t\t\t_ = ", getterChain) // Ensure path is initialized
			gf.P("\t\t\t// Append to slice")
			gf.P("\t\t\t_ = &", elemType, "{} // ensure import")
			gf.P("\t\t\t_ = vCopy")
			gf.P("\t\t}")
			gf.P("\t}")
			gf.P("\t// TODO: complete repeated field assignment")
			return
		} else if field.NeedsCaster {
			// Message with type override (e.g., time.Time -> Timestamp)
			if plainIsPointer {
				// Plain is pointer (e.g., *time.Time) - need to dereference
				valueExpr = g.casterCallWithImport(gf, field, "*"+srcField, false)
			} else {
				valueExpr = g.casterCallWithImport(gf, field, srcField, false)
			}
			valueIsPointer = true // Caster returns pointer for Timestamp
		}
	} else if field.NeedsCaster {
		// Scalar with type override
		if plainIsPointer {
			// Plain is pointer - need to dereference
			valueExpr = g.casterCallWithImport(gf, field, "*"+srcField, false)
		} else {
			valueExpr = g.casterCallWithImport(gf, field, srcField, false)
		}
	} else if protoIsPointer && !plainIsPointer {
		// Proto wants pointer, plain has value - take address
		valueExpr = "&" + srcField
		valueIsPointer = true
	}

	// Build setter code with oneof handling
	initCode, assignCode := pathInfo.BuildSetterCode(gf, "pb", valueExpr, valueIsPointer)

	// Generate nil check for source value (with case check for oneof fields)
	// For overridden types (e.g., time.Time from Timestamp), check nil if plainIsPointer
	needsNilCheck := plainIsPointer || (field.Kind == KindMessage && !field.IsRepeated && !field.NeedsCaster)
	if needsNilCheck {
		gf.P("\tif ", srcField, " != nil", caseCheck, " {")
		if initCode != "" {
			gf.P(initCode)
		}
		gf.P("\t\t", assignCode)
		gf.P("\t}")
	} else if field.IsRepeated {
		gf.P("\tif len(", srcField, ") > 0", caseCheck, " {")
		if initCode != "" {
			gf.P(initCode)
		}
		gf.P("\t\t", assignCode)
		gf.P("\t}")
	} else if field.GoType.Name == "string" {
		// Check for non-empty string (with case check for oneof)
		if caseCheck != "" {
			gf.P("\tif p.", field.OneofGoName, "Case == \"", field.OneofVariant, "\" {")
		} else {
			gf.P("\tif ", srcField, " != \"\" {")
		}
		if initCode != "" {
			gf.P(initCode)
		}
		gf.P("\t\t", assignCode)
		gf.P("\t}")
	} else if caseCheck != "" {
		// Scalar with case check
		gf.P("\tif p.", field.OneofGoName, "Case == \"", field.OneofVariant, "\" {")
		if initCode != "" {
			gf.P(initCode)
		}
		gf.P("\t\t", assignCode)
		gf.P("\t}")
	} else {
		// Scalar - always set (with initialization)
		if initCode != "" {
			gf.P(initCode)
		}
		gf.P("\t", assignCode)
	}
}

// generateIntoPbOneofField handles oneof field assignment (scalar or message)
func (g *Generator) generateIntoPbOneofField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	if field.Source == nil {
		return
	}

	srcField := "p." + field.GoName
	caseFieldName := field.OneofGoName + "Case"

	// Get oneof info from source field
	oneof := field.Source.Oneof
	if oneof == nil {
		gf.P("\t// ", field.GoName, ": not a oneof field")
		return
	}

	// Build wrapper type name: ParentMessage_FieldName
	wrapperIdent := protogen.GoIdent{
		GoName:       oneof.Parent.GoIdent.GoName + "_" + field.Source.GoName,
		GoImportPath: oneof.Parent.GoIdent.GoImportPath,
	}
	wrapperType := gf.QualifiedGoIdent(wrapperIdent)

	gf.P("\t// ", field.GoName, " -> ", field.EmPath)

	// Determine if field is message or scalar
	if field.Source.Message != nil {
		// Message field - check for nil
		gf.P("\tif ", srcField, " != nil && p.", caseFieldName, " == \"", field.OneofVariant, "\" {")
		gf.P("\t\tpb.", oneof.GoName, " = &", wrapperType, "{", field.Source.GoName, ": ", srcField, "}")
		gf.P("\t}")
	} else {
		// Scalar field - check for case only
		gf.P("\tif p.", caseFieldName, " == \"", field.OneofVariant, "\" {")
		gf.P("\t\tpb.", oneof.GoName, " = &", wrapperType, "{", field.Source.GoName, ": ", srcField, "}")
		gf.P("\t}")
	}
}

// generateIntoPbSerializedField handles deserialization of bytes back to message
func (g *Generator) generateIntoPbSerializedField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	protoPkg := protogen.GoImportPath("google.golang.org/protobuf/proto")

	srcField := "p." + field.GoName
	assignment := g.buildPbAssignmentPath(gf, field, msg, f)

	gf.P("\t// ", field.GoName, " deserialize -> ", field.EmPath)
	gf.P("\tif len(", srcField, ") > 0 {")
	if assignment.InitCode != "" {
		gf.P(assignment.InitCode)
	}
	gf.P("\t\tvar msg ", g.getProtoTypeIdent(gf, field))
	gf.P("\t\tif err := ", gf.QualifiedGoIdent(protoPkg.Ident("Unmarshal")), "(", srcField, ", &msg); err == nil {")
	gf.P("\t\t\t", assignment.Path, " = &msg")
	gf.P("\t\t}")
	gf.P("\t}")
}

// generateIntoPbTypeAliasField handles type alias assignment
func (g *Generator) generateIntoPbTypeAliasField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	if field.Source == nil || field.Source.Message == nil {
		gf.P("\t// ", field.GoName, " type alias: no source message")
		return
	}

	srcField := "p." + field.GoName

	// Get the wrapper message type
	wrapperType := gf.QualifiedGoIdent(field.Source.Message.GoIdent)

	// Get the alias field name from message options
	msgOpts := g.getMessageOptions(field.Source.Message)
	aliasFieldName := "Value" // default
	if msgOpts != nil && msgOpts.TypeAliasField != "" {
		// Convert to Go name (capitalize first letter)
		aliasFieldName = strings.Title(msgOpts.TypeAliasField)
	}

	gf.P("\t// ", field.GoName, " type alias -> ", field.EmPath)

	// Check for zero value based on type
	switch field.ScalarKind {
	case protoreflect.StringKind:
		gf.P("\tif ", srcField, " != \"\" {")
	case protoreflect.BytesKind:
		gf.P("\tif len(", srcField, ") > 0 {")
	default:
		// For numeric types, skip zero-value check or always set
		gf.P("\t{")
	}

	gf.P("\t\tpb.", field.Source.GoName, " = &", wrapperType, "{", aliasFieldName, ": ", srcField, "}")
	gf.P("\t}")
}

// Helper types and functions

type NavigationPath struct {
	NilCheck string // e.g., "pb.GetHeartbeat() != nil && pb.GetHeartbeat().GetAgent() != nil"
	Value    string // e.g., "pb.GetHeartbeat().GetAgent()"
}

type AssignmentPath struct {
	InitCode string // Code to initialize parent structures
	Path     string // e.g., "pb.PlatformEvent.(*Heartbeat).Agent"
}

// buildPbNavigationPath builds the getter chain for reading from protobuf
func (g *Generator) buildPbNavigationPath(field *IRField, msg *IRMessage) NavigationPath {
	if field.Source != nil {
		srcGoName := field.Source.GoName
		// For direct fields (not embedded) - use pb.FieldName
		return NavigationPath{
			NilCheck: "pb." + srcGoName + " != nil",
			Value:    "pb." + srcGoName,
		}
	}
	if len(field.PathNumbers) == 0 {
		return NavigationPath{NilCheck: "false", Value: ""}
	}

	// For embedded fields - simplified version
	// Real implementation needs proper path navigation
	return NavigationPath{
		NilCheck: "pb != nil", // TODO: proper nil check chain
		Value:    "pb",        // TODO: proper getter chain
	}
}

// buildPbAssignmentPath builds the assignment path for writing to protobuf
func (g *Generator) buildPbAssignmentPath(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) AssignmentPath {
	if field.Source != nil && field.Origin == OriginDirect {
		return AssignmentPath{
			Path: "pb." + field.Source.GoName,
		}
	}

	// TODO: Implement proper path building for embedded fields
	return AssignmentPath{
		InitCode: "\t// TODO: Initialize parent structures for " + field.GoName,
		Path:     "pb." + field.GoName,
	}
}

func (g *Generator) buildTypeStringPlain(field *IRField, f *protogen.File) string {
	if field.GoType.IsPointer {
		return "*" + field.GoType.Name
	}
	return field.GoType.Name
}

func (g *Generator) buildPbSliceType(gf *protogen.GeneratedFile, field *IRField, f *protogen.File) string {
	if field.Source != nil && field.Source.Message != nil {
		return "[]*" + gf.QualifiedGoIdent(field.Source.Message.GoIdent)
	}
	return "[]" + field.GoType.Name
}

func (g *Generator) buildPbMapValueType(gf *protogen.GeneratedFile, field *IRField, f *protogen.File) string {
	if field.MapValue != nil && field.MapValue.Source != nil && field.MapValue.Source.Message != nil {
		return "*" + gf.QualifiedGoIdent(field.MapValue.Source.Message.GoIdent)
	}
	if field.MapValue != nil {
		return field.MapValue.GoType.Name
	}
	return "any"
}

func (g *Generator) getProtoTypeIdent(gf *protogen.GeneratedFile, field *IRField) string {
	if field.Source != nil && field.Source.Message != nil {
		return gf.QualifiedGoIdent(field.Source.Message.GoIdent)
	}
	return field.ProtoType
}

func (g *Generator) getMessageOptionsFromField(field *IRField) *goplain.MessageOptions {
	if field.Source == nil || field.Source.Message == nil {
		return nil
	}
	return g.getMessageOptions(field.Source.Message)
}

func (g *Generator) getMessageOptions(msg *protogen.Message) *goplain.MessageOptions {
	opts := msg.Desc.Options()
	if opts == nil {
		return nil
	}
	ext := proto.GetExtension(opts, goplain.E_Message)
	if ext == nil {
		return nil
	}
	return ext.(*goplain.MessageOptions)
}

// needsTypeCast checks if the field has override_type that differs from source type
func (g *Generator) needsTypeCast(field *IRField) bool {
	if field.Source == nil {
		return false
	}
	// Check if field has override_type set (different GoType than default)
	srcType := g.getSourceTypeName(field)
	return srcType != "" && srcType != field.GoType.Name
}

// getSourceTypeName returns the Go type name of the protobuf source field
func (g *Generator) getSourceTypeName(field *IRField) string {
	if field.Source == nil {
		return ""
	}
	switch field.Source.Desc.Kind() {
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
		return ""
	}
}
