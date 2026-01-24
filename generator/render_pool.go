package generator

import (
	"google.golang.org/protobuf/compiler/protogen"
)

var syncPkg = protogen.GoImportPath("sync")

// generatePoolMethods generates sync.Pool, Reset, Get, and Put methods for a Plain struct
func (g *Generator) generatePoolMethods(gf *protogen.GeneratedFile, msg *IRMessage) {
	plainType := msg.GoName
	poolVar := lowerFirst(plainType) + "Pool"

	// Generate pool variable
	gf.P("// ", poolVar, " is a sync.Pool for ", plainType, " objects")
	gf.P("var ", poolVar, " = ", gf.QualifiedGoIdent(syncPkg.Ident("Pool")), "{")
	gf.P("\tNew: func() interface{} {")
	gf.P("\t\treturn &", plainType, "{")
	gf.P("\t\t\tSrc_: make([]uint16, 0, 32),")
	gf.P("\t\t}")
	gf.P("\t},")
	gf.P("}")
	gf.P()

	// Generate Get function
	gf.P("// Get", plainType, " returns a ", plainType, " from the pool")
	gf.P("func Get", plainType, "() *", plainType, " {")
	gf.P("\treturn ", poolVar, ".Get().(*", plainType, ")")
	gf.P("}")
	gf.P()

	// Generate Put function
	gf.P("// Put", plainType, " returns a ", plainType, " to the pool after resetting it")
	gf.P("func Put", plainType, "(p *", plainType, ") {")
	gf.P("\tif p == nil {")
	gf.P("\t\treturn")
	gf.P("\t}")
	gf.P("\tp.Reset()")
	gf.P("\t", poolVar, ".Put(p)")
	gf.P("}")
	gf.P()

	// Generate Reset method
	g.generateResetMethod(gf, msg)
}

// generateResetMethod generates a Reset method that clears all fields
func (g *Generator) generateResetMethod(gf *protogen.GeneratedFile, msg *IRMessage) {
	plainType := msg.GoName

	gf.P("// Reset clears all fields in ", plainType, " for reuse")
	gf.P("func (p *", plainType, ") Reset() {")
	gf.P("\tif p == nil {")
	gf.P("\t\treturn")
	gf.P("\t}")
	gf.P()

	// Reset Src_ - keep capacity but clear length
	gf.P("\tp.Src_ = p.Src_[:0]")
	gf.P()

	// Reset embedded oneof case fields
	for _, eo := range msg.EmbeddedOneofs {
		gf.P("\tp.", eo.CaseFieldName, " = \"\"")
	}

	// Reset all fields to zero values
	for _, field := range msg.Fields {
		g.generateFieldReset(gf, field)
	}

	gf.P("}")
	gf.P()
}

// generateFieldReset generates reset code for a single field
func (g *Generator) generateFieldReset(gf *protogen.GeneratedFile, field *IRField) {
	fieldAccess := "p." + field.GoName

	if field.GoType.IsPointer {
		gf.P("\t", fieldAccess, " = nil")
	} else if field.IsRepeated || field.GoType.IsSlice {
		// Keep capacity for slices
		gf.P("\t", fieldAccess, " = ", fieldAccess, "[:0]")
	} else if field.IsMap {
		// Clear map - use clear() in Go 1.21+ or iterate and delete
		gf.P("\tfor k := range ", fieldAccess, " {")
		gf.P("\t\tdelete(", fieldAccess, ", k)")
		gf.P("\t}")
	} else if field.Kind == KindBytes {
		// bytes ([]byte) - set to nil
		gf.P("\t", fieldAccess, " = nil")
	} else {
		// Scalar zero values
		switch field.GoType.Name {
		case "string":
			gf.P("\t", fieldAccess, " = \"\"")
		case "bool":
			gf.P("\t", fieldAccess, " = false")
		case "int32", "int64", "uint32", "uint64", "float32", "float64", "int", "uint":
			gf.P("\t", fieldAccess, " = 0")
		case "[]byte":
			gf.P("\t", fieldAccess, " = nil")
		default:
			// For custom types (enums, time.Duration, etc.) use zero value
			if field.Kind == KindEnum {
				gf.P("\t", fieldAccess, " = 0")
			} else if field.Kind == KindMessage {
				gf.P("\t", fieldAccess, " = nil")
			} else {
				// Most custom types (like time.Duration) are numeric under the hood
				// Use 0 which works for int-based types
				gf.P("\t", fieldAccess, " = 0")
			}
		}
	}
}

// lowerFirst converts first character to lowercase
func lowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]+32) + s[1:]
}
