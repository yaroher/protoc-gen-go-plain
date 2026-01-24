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
func (g *Generator) generateConversionMethods(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	g.generateIntoPlain(gf, msg, f)
	g.generateIntoPb(gf, msg, f)
}

// generateIntoPlain generates method to convert protobuf message to plain struct
// func (pb *OriginalMessage) IntoPlain() *MessagePlain
func (g *Generator) generateIntoPlain(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	if msg.Source == nil {
		return
	}

	pbType := msg.Source.GoIdent
	plainType := msg.GoName

	gf.P("// IntoPlain converts protobuf message to plain struct")
	gf.P("func (pb *", gf.QualifiedGoIdent(pbType), ") IntoPlain() *", plainType, " {")
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

// generateOneofCaseDetection generates code to detect which oneof variant is set
func (g *Generator) generateOneofCaseDetection(gf *protogen.GeneratedFile, eo *EmbeddedOneof) {
	gf.P("\t// Detect ", eo.Name, " oneof case")
	gf.P("\tswitch pb.", eo.GoName, ".(type) {")
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
	srcAppend := fmt.Sprintf("p.Src_ = append(p.Src_, %d)", field.Index)

	// Check if proto field is optional (pointer) but plain field is not
	protoIsPointer := field.Source.Desc.HasOptionalKeyword() || field.Source.Desc.HasPresence()
	plainIsPointer := field.GoType.IsPointer

	// Check if types match or need conversion
	if field.Kind == KindMessage {
		// Message fields need IntoPlain() call if the nested type has generate=true
		msgOpts := g.getMessageOptionsFromField(field)
		if msgOpts != nil && msgOpts.Generate {
			if field.IsRepeated {
				gf.P("\tif len(", srcField, ") > 0 {")
				gf.P("\t\t", dstField, " = make([]", g.buildTypeStringPlain(field, f), ", len(", srcField, "))")
				gf.P("\t\tfor i, v := range ", srcField, " {")
				gf.P("\t\t\t", dstField, "[i] = v.IntoPlain()")
				gf.P("\t\t}")
				gf.P("\t\t", srcAppend)
				gf.P("\t}")
			} else {
				gf.P("\tif ", srcField, " != nil {")
				gf.P("\t\t", dstField, " = ", srcField, ".IntoPlain()")
				gf.P("\t\t", srcAppend)
				gf.P("\t}")
			}
		} else {
			// Use original protobuf type
			gf.P("\t", dstField, " = ", srcField)
			gf.P("\t", srcAppend)
		}
	} else if protoIsPointer && !plainIsPointer {
		// Proto has optional (pointer), plain has value - dereference with nil check
		gf.P("\tif ", srcField, " != nil {")
		if field.ToPlainCast != "" {
			gf.P("\t\t", dstField, " = ", field.ToPlainCast, "(*", srcField, ")")
		} else {
			gf.P("\t\t", dstField, " = *", srcField)
		}
		gf.P("\t\t", srcAppend)
		gf.P("\t}")
	} else if !protoIsPointer && plainIsPointer {
		// Proto has value, plain has pointer - take address
		if field.ToPlainCast != "" {
			gf.P("\t_tmp := ", field.ToPlainCast, "(", srcField, ")")
			gf.P("\t", dstField, " = &_tmp")
		} else {
			gf.P("\t", dstField, " = &", srcField)
		}
		gf.P("\t", srcAppend)
	} else {
		// Scalar, enum, bytes - direct copy (types match) or with cast
		if field.ToPlainCast != "" {
			gf.P("\t", dstField, " = ", field.ToPlainCast, "(", srcField, ")")
		} else {
			gf.P("\t", dstField, " = ", srcField)
		}
		gf.P("\t", srcAppend)
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
	srcAppend := fmt.Sprintf("p.Src_ = append(p.Src_, %d)", field.Index)

	gf.P("\t// ", field.GoName, " from ", field.EmPath)
	gf.P("\tif ", nilCheck, " {")

	// Check source field characteristics
	leafField := pathInfo.LeafField

	// Handle different field types
	if field.Kind == KindMessage {
		msgOpts := g.getMessageOptionsFromField(field)
		if msgOpts != nil && msgOpts.Generate {
			// Plain type - call IntoPlain()
			if field.IsRepeated {
				// Repeated message with generate=true
				elemType := field.GoType.Name
				gf.P("\t\tfor _, v := range ", getterChain, " {")
				gf.P("\t\t\t", dstField, " = append(", dstField, ", *v.IntoPlain())")
				gf.P("\t\t}")
				_ = elemType
			} else {
				gf.P("\t\t", dstField, " = ", getterChain, ".IntoPlain()")
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
	} else {
		// Scalar, enum, bytes - direct assignment
		// Note: protobuf getters always return values (not pointers) for scalars
		gf.P("\t\t", dstField, " = ", getterChain)
	}

	gf.P("\t\t", srcAppend)
	gf.P("\t}")
}

// generateIntoPlainSerializedField handles serialized field (message -> bytes)
func (g *Generator) generateIntoPlainSerializedField(gf *protogen.GeneratedFile, field *IRField, msg *IRMessage, f *protogen.File) {
	protoPkg := protogen.GoImportPath("google.golang.org/protobuf/proto")

	path := g.buildPbNavigationPath(field, msg)
	dstField := "p." + field.GoName
	srcAppend := fmt.Sprintf("p.Src_ = append(p.Src_, %d)", field.Index)

	gf.P("\t// ", field.GoName, " serialized from ", field.EmPath)
	gf.P("\tif ", path.NilCheck, " {")
	gf.P("\t\tif data, err := ", gf.QualifiedGoIdent(protoPkg.Ident("Marshal")), "(", path.Value, "); err == nil {")
	gf.P("\t\t\t", dstField, " = data")
	gf.P("\t\t\t", srcAppend)
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
	srcAppend := fmt.Sprintf("p.Src_ = append(p.Src_, %d)", field.Index)
	getterChain := pathInfo.BuildGetterChain("pb")
	nilCheck := pathInfo.BuildNilCheck("pb")

	gf.P("\t// ", field.GoName, " type alias from ", field.EmPath)
	gf.P("\tif ", nilCheck, " {")
	gf.P("\t\t", dstField, " = ", getterChain)
	gf.P("\t\t", srcAppend)
	gf.P("\t}")
}

// generateIntoPb generates method to convert plain struct back to protobuf
// func (p *MessagePlain) IntoPb() *OriginalMessage
func (g *Generator) generateIntoPb(gf *protogen.GeneratedFile, msg *IRMessage, f *protogen.File) {
	if msg.Source == nil {
		return
	}

	pbType := msg.Source.GoIdent
	plainType := msg.GoName

	gf.P("// IntoPb converts plain struct to protobuf message")
	gf.P("func (p *", plainType, ") IntoPb() *", gf.QualifiedGoIdent(pbType), " {")
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
	protoIsPointer := field.Source.Desc.HasOptionalKeyword() || field.Source.Desc.HasPresence()
	plainIsPointer := field.GoType.IsPointer

	if field.Kind == KindMessage {
		msgOpts := g.getMessageOptionsFromField(field)
		if msgOpts != nil && msgOpts.Generate {
			if field.IsRepeated {
				gf.P("\tif len(", srcField, ") > 0 {")
				gf.P("\t\t", dstField, " = make(", g.buildPbSliceType(gf, field, f), ", len(", srcField, "))")
				gf.P("\t\tfor i, v := range ", srcField, " {")
				gf.P("\t\t\t", dstField, "[i] = v.IntoPb()")
				gf.P("\t\t}")
				gf.P("\t}")
			} else {
				gf.P("\tif ", srcField, " != nil {")
				gf.P("\t\t", dstField, " = ", srcField, ".IntoPb()")
				gf.P("\t}")
			}
		} else {
			gf.P("\t", dstField, " = ", srcField)
		}
	} else if protoIsPointer && !plainIsPointer {
		// Proto wants pointer, plain has value - take address (with non-zero check for strings)
		switch field.GoType.Name {
		case "string":
			gf.P("\tif ", srcField, " != \"\" {")
			if field.ToPbCast != "" {
				gf.P("\t\t_tmp := ", field.ToPbCast, "(", srcField, ")")
				gf.P("\t\t", dstField, " = &_tmp")
			} else {
				gf.P("\t\t", dstField, " = &", srcField)
			}
			gf.P("\t}")
		default:
			if field.ToPbCast != "" {
				gf.P("\t_tmp := ", field.ToPbCast, "(", srcField, ")")
				gf.P("\t", dstField, " = &_tmp")
			} else {
				gf.P("\t", dstField, " = &", srcField)
			}
		}
	} else if !protoIsPointer && plainIsPointer {
		// Proto wants value, plain has pointer - dereference
		gf.P("\tif ", srcField, " != nil {")
		if field.ToPbCast != "" {
			gf.P("\t\t", dstField, " = ", field.ToPbCast, "(*", srcField, ")")
		} else {
			gf.P("\t\t", dstField, " = *", srcField)
		}
		gf.P("\t}")
	} else {
		// Direct copy or with cast
		if field.ToPbCast != "" {
			gf.P("\t", dstField, " = ", field.ToPbCast, "(", srcField, ")")
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
	plainIsPointer := field.GoType.IsPointer

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
		}
	} else if protoIsPointer && !plainIsPointer {
		// Proto wants pointer, plain has value - take address
		valueExpr = "&" + srcField
		valueIsPointer = true
	}

	// Build setter code with oneof handling
	initCode, assignCode := pathInfo.BuildSetterCode(gf, "pb", valueExpr, valueIsPointer)

	// Generate nil check for source value (with case check for oneof fields)
	if field.GoType.IsPointer || (field.Kind == KindMessage && !field.IsRepeated) {
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
	if len(field.PathNumbers) == 0 {
		if field.Source != nil {
			return NavigationPath{
				NilCheck: "pb != nil",
				Value:    "pb." + field.Source.GoName,
			}
		}
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
