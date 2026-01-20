package generator

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/ir"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func buildMessageMap(p *protogen.Plugin) map[string]*protogen.Message {
	result := make(map[string]*protogen.Message)
	if p == nil {
		return result
	}
	for _, f := range p.Files {
		collectMessages(f.Messages, result)
	}
	return result
}

func collectMessages(msgs []*protogen.Message, result map[string]*protogen.Message) {
	for _, m := range msgs {
		result[string(m.Desc.FullName())] = m
		collectMessages(m.Messages, result)
	}
}

func emitEnumFullConsts(g *protogen.GeneratedFile, oneofEnums map[string]map[string]*oneofEnumInfo) {
	if len(oneofEnums) == 0 {
		return
	}
	enumSet := make(map[string]struct{})
	for _, infos := range oneofEnums {
		for _, info := range infos {
			if info == nil {
				continue
			}
			for _, vals := range info.FieldToEnums {
				for _, v := range vals {
					if v == nil || v.Desc == nil || v.Desc.Parent() == nil {
						continue
					}
					enumSet[string(v.Desc.Parent().FullName())] = struct{}{}
				}
			}
		}
	}
	if len(enumSet) == 0 {
		return
	}
	keys := make([]string, 0, len(enumSet))
	for k := range enumSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, full := range keys {
		name := enumFullConstName(full)
		if name == "" {
			continue
		}
		g.P("const ", name, " = ", strconv.Quote(strings.TrimPrefix(full, ".")))
	}
	g.P("")
}

func generateConverters(g *protogen.GeneratedFile, plainMsg *protogen.Message, pbMsg *protogen.Message, msgIR *ir.MessageIR, generatedEnums map[string]struct{}, enumValues map[string]*protogen.EnumValue, enumByFull map[string]*protogen.Enum) {
	if plainMsg == nil || pbMsg == nil || msgIR == nil {
		return
	}

	fieldPlans := make(map[string]*ir.FieldPlan)
	for _, fp := range msgIR.FieldPlan {
		if fp == nil {
			continue
		}
		fieldPlans[fp.NewField.Name] = fp
	}

	pbFields := make(map[string]*protogen.Field)
	for _, f := range pbMsg.Fields {
		pbFields[string(f.Desc.Name())] = f
	}

	embedSources := make(map[string]*protogen.Field)
	for _, fp := range msgIR.FieldPlan {
		if fp == nil || !fp.Origin.IsEmbedded || fp.Origin.EmbedSource == nil {
			continue
		}
		src := fp.Origin.EmbedSource.FieldName
		if f, ok := pbFields[src]; ok {
			embedSources[src] = f
		}
	}

	oneofEnums := buildOneofEnumInfo(plainMsg, pbMsg, msgIR, fieldPlans, pbFields, enumValues, enumByFull)
	emitEnumFullConsts(g, oneofEnums)
	generateIntoPlain(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources, oneofEnums, msgIR)
	generateIntoPlainErr(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources, oneofEnums, msgIR)
	generateIntoPb(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources, oneofEnums, msgIR)
	generateIntoPbErr(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources, oneofEnums, msgIR)
}

func generateIntoPlain(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field, oneofEnums map[string]map[string]*oneofEnumInfo, msgIR *ir.MessageIR) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, false, false)
	g.P("func (m *", pbMsg.GoIdent.GoName, ") IntoPlain(", strings.Join(params, ", "), ") *", plainMsg.GoIdent.GoName, " {")
	g.P("\tif m == nil { return nil }")

	oneofVars := buildOneofPlainVars(g, plainMsg, fieldPlans, pbFields, oneofEnums, false, msgIR)
	for _, line := range oneofVars.lines {
		g.P("\t" + line)
	}

	g.P("\treturn &", plainMsg.GoIdent.GoName, "{")
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil {
			continue
		}
		if v, ok := oneofVars.discVar[f.GoName]; ok {
			g.P("\t\t", f.GoName, ": ", v, ",")
			continue
		}
		if v, ok := oneofVars.fieldVar[f.GoName]; ok {
			g.P("\t\t", f.GoName, ": ", v, ",")
			continue
		}
		if fp.Origin.EnumAsString || fp.Origin.EnumAsInt {
			expr := plainFieldValueExpr(g, f, fp, pbFields, embedSources, false)
			if expr == "" {
				continue
			}
			if fp.Origin.EnumAsString {
				if f.Desc.Cardinality() == protoreflect.Repeated {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToSliceString", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				} else {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToString", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				}
			} else {
				if f.Desc.Cardinality() == protoreflect.Repeated {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToSliceInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				} else {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				}
			}
			g.P("\t\t", f.GoName, ": ", expr, ",")
			continue
		}
		expr := plainFieldValueExpr(g, f, fp, pbFields, embedSources, false)
		if expr == "" {
			continue
		}
		if hasOverride(fp) {
			expr = casterName(f) + "(" + expr + ")"
		}
		g.P("\t\t", f.GoName, ": ", expr, ",")
	}
	g.P("\t}")
	g.P("}")
	g.P("")
}

func generateIntoPlainErr(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field, oneofEnums map[string]map[string]*oneofEnumInfo, msgIR *ir.MessageIR) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, true, false)
	g.P("func (m *", pbMsg.GoIdent.GoName, ") IntoPlainErr(", strings.Join(params, ", "), ") (*", plainMsg.GoIdent.GoName, ", error) {")
	g.P("\tif m == nil { return nil, nil }")

	oneofVars := buildOneofPlainVars(g, plainMsg, fieldPlans, pbFields, oneofEnums, true, msgIR)
	for _, line := range oneofVars.lines {
		g.P("\t" + line)
	}

	for groupName, matchVar := range oneofVars.matchVar {
		info := primaryEnumInfo(oneofEnums[groupName])
		if info == nil || info.DiscriminatorPlain == "" || !info.UseEnumDiscriminator {
			continue
		}
		oneofGetter := "m.Get" + groupName + "()"
		errf := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Errorf", GoImportPath: "fmt"})
		g.P("\tif ", oneofGetter, " != nil && !", matchVar, " { return nil, ", errf, "(\"oneof %s discriminator mismatch\", \"", groupName, "\") }")
	}

	serializedVars := map[string]string{}
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil || !fp.Origin.IsSerialized {
			continue
		}
		pbField := pbFields[fp.OrigField.FieldName]
		if pbField == nil {
			continue
		}
		varName := f.GoName + "Val"
		serializedVars[f.GoName] = varName
		castFn := g.QualifiedGoIdent(protogen.GoIdent{GoName: "MessageToSliceByteErr", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
		g.P("\t", varName, ", err := ", castFn, "(m.Get", pbField.GoName, "())")
		g.P("\tif err != nil { return nil, err }")
	}

	// precompute error-returning casts
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil {
			continue
		}
		if hasOverride(fp) {
			expr := plainFieldValueExpr(g, f, fp, pbFields, embedSources, false)
			g.P("\t", f.GoName, "Val, err := ", casterName(f), "(", expr, ")")
			g.P("\tif err != nil { return nil, err }")
		}
	}

	g.P("\treturn &", plainMsg.GoIdent.GoName, "{")
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil {
			continue
		}
		if v, ok := oneofVars.discVar[f.GoName]; ok {
			g.P("\t\t", f.GoName, ": ", v, ",")
			continue
		}
		if v, ok := oneofVars.fieldVar[f.GoName]; ok {
			g.P("\t\t", f.GoName, ": ", v, ",")
			continue
		}
		if fp.Origin.EnumAsString || fp.Origin.EnumAsInt {
			expr := plainFieldValueExpr(g, f, fp, pbFields, embedSources, true)
			if expr == "" {
				continue
			}
			if fp.Origin.EnumAsString {
				if f.Desc.Cardinality() == protoreflect.Repeated {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToSliceString", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				} else {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToString", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				}
			} else {
				if f.Desc.Cardinality() == protoreflect.Repeated {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToSliceInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				} else {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumToInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					expr = enumCast + "(" + expr + ")"
				}
			}
			g.P("\t\t", f.GoName, ": ", expr, ",")
			continue
		}
		if fp.Origin.IsSerialized {
			if v, ok := serializedVars[f.GoName]; ok {
				g.P("\t\t", f.GoName, ": ", v, ",")
			}
			continue
		}
		if fp.OrigField == nil {
			continue
		}
		if hasOverride(fp) {
			g.P("\t\t", f.GoName, ": ", f.GoName, "Val,")
			continue
		}
		expr := plainFieldValueExpr(g, f, fp, pbFields, embedSources, false)
		if expr == "" {
			continue
		}
		g.P("\t\t", f.GoName, ": ", expr, ",")
	}
	g.P("\t}, nil")
	g.P("}")
	g.P("")
}

func generateIntoPb(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field, oneofEnums map[string]map[string]*oneofEnumInfo, msgIR *ir.MessageIR) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, false, true)
	g.P("func (m *", plainMsg.GoIdent.GoName, ") IntoPb(", strings.Join(params, ", "), ") *", pbMsg.GoIdent.GoName, " {")
	g.P("\tif m == nil { return nil }")

	// build embedded structs
	embedStructs := buildEmbedStructs(g, plainMsg, fieldPlans, pbFields, embedSources)
	for _, line := range embedStructs {
		g.P("\t" + line)
	}

	oneofVars := buildOneofVars(g, plainMsg, fieldPlans, pbFields, oneofEnums, false, msgIR)
	for _, line := range oneofVars.lines {
		g.P("\t" + line)
	}

	g.P("\treturn &", pbMsg.GoIdent.GoName, "{")
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil {
			continue
		}
		if fp.Origin.IsEmbedded {
			continue
		}
		pbField := pbFields[fp.OrigField.FieldName]
		if pbField == nil {
			continue
		}
		if fp.Origin.IsOneof {
			continue
		}
		if fp.Origin.EnumAsString || fp.Origin.EnumAsInt {
			enumType := g.QualifiedGoIdent(pbField.Enum.GoIdent)
			if fp.Origin.EnumAsString {
				if f.Desc.Cardinality() == protoreflect.Repeated {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromSliceString", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					g.P("\t\t", pbField.GoName, ": ", enumCast, "[", enumType, "](m.", f.GoName, "),")
				} else {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromString", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					g.P("\t\t", pbField.GoName, ": ", enumCast, "[", enumType, "](m.", f.GoName, "),")
				}
			} else {
				if f.Desc.Cardinality() == protoreflect.Repeated {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromSliceInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					g.P("\t\t", pbField.GoName, ": ", enumCast, "[", enumType, "](m.", f.GoName, "),")
				} else {
					enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
					g.P("\t\t", pbField.GoName, ": ", enumCast, "[", enumType, "](m.", f.GoName, "),")
				}
			}
			continue
		}
		if hasOverride(fp) {
			g.P("\t\t", pbField.GoName, ": ", casterName(f), "(m.", f.GoName, "),")
			continue
		}
		expr := pbFieldValueExpr(g, f, fp, pbField, false)
		if expr == "" {
			continue
		}
		g.P("\t\t", pbField.GoName, ": ", expr, ",")
	}
	for name := range embedSources {
		pbField := embedSources[name]
		if pbField == nil {
			continue
		}
		g.P("\t\t", pbField.GoName, ": ", embedVarName(name), ",")
	}
	for groupName, varName := range oneofVars.groupVar {
		g.P("\t\t", groupName, ": ", varName, ",")
	}
	g.P("\t}")
	g.P("}")
	g.P("")
}

func generateIntoPbErr(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field, oneofEnums map[string]map[string]*oneofEnumInfo, msgIR *ir.MessageIR) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, true, true)
	g.P("func (m *", plainMsg.GoIdent.GoName, ") IntoPbErr(", strings.Join(params, ", "), ") (*", pbMsg.GoIdent.GoName, ", error) {")
	g.P("\tif m == nil { return nil, nil }")

	serializedVars := map[string]string{}
	enumStringVars := map[string]string{}
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil || !fp.Origin.IsSerialized {
			continue
		}
		pbField := pbFields[fp.OrigField.FieldName]
		if pbField == nil {
			continue
		}
		varName := pbField.GoName + "Val"
		serializedVars[f.GoName] = varName
		castFn := g.QualifiedGoIdent(protogen.GoIdent{GoName: "MessageFromSliceByteErr", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
		msgIdent := pbField.Message.GoIdent.GoName
		g.P("\t", varName, ", err := ", castFn, "[*", msgIdent, "](m.", f.GoName, ")")
		g.P("\tif err != nil { return nil, err }")
	}

	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil {
			continue
		}
		if fp.Origin.EnumAsString {
			pbField := pbFields[fp.OrigField.FieldName]
			if pbField == nil {
				continue
			}
			enumType := g.QualifiedGoIdent(pbField.Enum.GoIdent)
			varName := f.GoName + "Val"
			enumStringVars[f.GoName] = varName
			if f.Desc.Cardinality() == protoreflect.Repeated {
				enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromSliceStringErr", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
				g.P("\t", varName, ", err := ", enumCast, "[", enumType, "](m.", f.GoName, ")")
			} else {
				enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromStringErr", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
				g.P("\t", varName, ", err := ", enumCast, "[", enumType, "](m.", f.GoName, ")")
			}
			g.P("\tif err != nil { return nil, err }")
			continue
		}
		if hasOverride(fp) {
			g.P("\t", f.GoName, "Val, err := ", casterName(f), "(m.", f.GoName, ")")
			g.P("\tif err != nil { return nil, err }")
		}
	}

	// build embedded structs
	embedStructs := buildEmbedStructs(g, plainMsg, fieldPlans, pbFields, embedSources)
	for _, line := range embedStructs {
		g.P("\t" + line)
	}

	oneofVars := buildOneofVars(g, plainMsg, fieldPlans, pbFields, oneofEnums, true, msgIR)
	for _, line := range oneofVars.lines {
		g.P("\t" + line)
	}

	g.P("\treturn &", pbMsg.GoIdent.GoName, "{")
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil {
			continue
		}
		if fp.Origin.IsEmbedded {
			continue
		}
		pbField := pbFields[fp.OrigField.FieldName]
		if pbField == nil {
			continue
		}
		if fp.Origin.IsOneof {
			continue
		}
		if fp.Origin.IsSerialized {
			if v, ok := serializedVars[f.GoName]; ok {
				g.P("\t\t", pbField.GoName, ": ", v, ",")
			}
			continue
		}
		if fp.Origin.EnumAsString || fp.Origin.EnumAsInt {
			if fp.Origin.EnumAsString {
				if v, ok := enumStringVars[f.GoName]; ok {
					g.P("\t\t", pbField.GoName, ": ", v, ",")
				}
				continue
			}
			enumType := g.QualifiedGoIdent(pbField.Enum.GoIdent)
			if f.Desc.Cardinality() == protoreflect.Repeated {
				enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromSliceInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
				g.P("\t\t", pbField.GoName, ": ", enumCast, "[", enumType, "](m.", f.GoName, "),")
			} else {
				enumCast := g.QualifiedGoIdent(protogen.GoIdent{GoName: "EnumFromInt32", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
				g.P("\t\t", pbField.GoName, ": ", enumCast, "[", enumType, "](m.", f.GoName, "),")
			}
			continue
		}
		if hasOverride(fp) {
			g.P("\t\t", pbField.GoName, ": ", f.GoName, "Val,")
			continue
		}
		expr := pbFieldValueExpr(g, f, fp, pbField, true)
		if expr == "" {
			continue
		}
		g.P("\t\t", pbField.GoName, ": ", expr, ",")
	}
	for name := range embedSources {
		pbField := embedSources[name]
		if pbField == nil {
			continue
		}
		g.P("\t\t", pbField.GoName, ": ", embedVarName(name), ",")
	}
	for groupName, varName := range oneofVars.groupVar {
		g.P("\t\t", groupName, ": ", varName, ",")
	}
	g.P("\t}, nil")
	g.P("}")
	g.P("")
}

func buildCasterParams(g *protogen.GeneratedFile, plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, withErr bool, reverse bool) []string {
	params := []string{}
	casterIdent := ""
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil {
			continue
		}
		if !hasOverride(fp) {
			continue
		}
		if casterIdent == "" {
			if withErr {
				casterIdent = g.QualifiedGoIdent(protogen.GoIdent{GoName: "CasterErr", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
			} else {
				casterIdent = g.QualifiedGoIdent(protogen.GoIdent{GoName: "Caster", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
			}
		}
		pbField := pbFields[fp.OrigField.FieldName]
		if pbField == nil {
			continue
		}
		srcType := getFieldGoTypeForGen(g, pbField)
		dstType := getOverrideGoType(g, f, fp)
		if reverse {
			srcType, dstType = dstType, srcType
		}
		params = append(params, fmt.Sprintf("%s %s[%s, %s]", casterName(f), casterIdent, srcType, dstType))
	}
	return params
}

func casterName(f *protogen.Field) string {
	return strings.ToLower(f.GoName[:1]) + f.GoName[1:] + "Cast"
}

func getOverrideGoType(g *protogen.GeneratedFile, f *protogen.Field, fp *ir.FieldPlan) string {
	for _, op := range fp.Ops {
		if op.Kind != ir.OpOverrideType {
			continue
		}
		name := op.Data["name"]
		importPath := op.Data["import_path"]
		if name == "" {
			break
		}
		if importPath == "" {
			return name
		}
		return g.QualifiedGoIdent(protogen.GoIdent{GoName: name, GoImportPath: protogen.GoImportPath(importPath)})
	}
	return getFieldGoTypeForGen(g, f)
}

func hasOverride(fp *ir.FieldPlan) bool {
	for _, op := range fp.Ops {
		if op.Kind == ir.OpOverrideType {
			return true
		}
	}
	return false
}

func embedVarName(name string) string {
	return "embed_" + name
}

func plainFieldValueExpr(g *protogen.GeneratedFile, f *protogen.Field, fp *ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field, useErr bool) string {
	if fp == nil || fp.OrigField == nil {
		return ""
	}
	if fp.Origin.IsEmbedded && fp.Origin.EmbedSource != nil {
		embedSrc := embedSources[fp.Origin.EmbedSource.FieldName]
		if embedSrc == nil || embedSrc.Message == nil {
			return ""
		}
		embeddedGoName := findEmbeddedGoName(embedSrc, fp.OrigField.FieldName)
		if embeddedGoName == "" {
			return ""
		}
		embed := "m.Get" + embedSrc.GoName + "()"
		zero := zeroValueForField(g, f)
		return fmt.Sprintf("func() %s { if %s == nil { return %s }; return %s.%s }()", getFieldGoTypeForGen(g, f), embed, zero, embed, embeddedGoName)
	}
	pbField := pbFields[fp.OrigField.FieldName]
	if pbField == nil {
		return ""
	}
	getter := "m.Get" + pbField.GoName + "()"
	if fp.Origin.IsTypeAlias {
		zero := zeroValueForField(g, f)
		return fmt.Sprintf("func() %s { if %s == nil { return %s }; return %s.Value }()", getFieldGoTypeForGen(g, f), getter, zero, getter)
	}
	if fp.Origin.IsSerialized {
		castFn := g.QualifiedGoIdent(protogen.GoIdent{GoName: "MessageToSliceByte", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
		if useErr {
			return "nil"
		}
		return fmt.Sprintf("%s(%s)", castFn, getter)
	}
	if fp.Origin.IsOneof {
		return ""
	}
	return getter
}

func pbFieldValueExpr(g *protogen.GeneratedFile, f *protogen.Field, fp *ir.FieldPlan, pbField *protogen.Field, useErr bool) string {
	if fp == nil || fp.OrigField == nil || pbField == nil {
		return ""
	}
	if fp.Origin.IsSerialized {
		castFn := g.QualifiedGoIdent(protogen.GoIdent{GoName: "MessageFromSliceByte", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
		msgIdent := pbField.Message.GoIdent.GoName
		if useErr {
			return "nil"
		}
		return fmt.Sprintf("%s[*%s](m.%s)", castFn, msgIdent, f.GoName)
	}
	if fp.Origin.IsTypeAlias {
		aliasIdent := pbField.Message.GoIdent
		return fmt.Sprintf("&%s{Value: m.%s}", aliasIdent.GoName, f.GoName)
	}
	if fp.Origin.IsOneof {
		wrapper := pbField.Parent.GoIdent.GoName + "_" + pbField.GoName
		return fmt.Sprintf("func() %s { if m.%s != nil { return &%s{%s: m.%s} }; return nil }()", pbField.GoName, f.GoName, wrapper, pbField.GoName, f.GoName)
	}
	return "m." + f.GoName
}

func findEmbeddedGoName(embedSrc *protogen.Field, embeddedName string) string {
	if embedSrc == nil || embedSrc.Message == nil {
		return ""
	}
	for _, ef := range embedSrc.Message.Fields {
		if string(ef.Desc.Name()) == embeddedName {
			return ef.GoName
		}
	}
	return ""
}

func buildEmbedStructs(g *protogen.GeneratedFile, plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field) []string {
	lines := []string{}
	for name, embedField := range embedSources {
		if embedField == nil {
			continue
		}
		fields := []string{}
		for _, f := range plainMsg.Fields {
			fp := fieldPlans[string(f.Desc.Name())]
			if fp == nil || fp.OrigField == nil || !fp.Origin.IsEmbedded || fp.Origin.EmbedSource == nil {
				continue
			}
			if fp.Origin.EmbedSource.FieldName != name {
				continue
			}
			embeddedGoName := findEmbeddedGoName(embedField, fp.OrigField.FieldName)
			if embeddedGoName == "" {
				continue
			}
			fields = append(fields, fmt.Sprintf("%s: m.%s", embeddedGoName, f.GoName))
		}
		if len(fields) == 0 {
			continue
		}
		lines = append(lines, "var "+embedVarName(name)+" *"+embedField.Message.GoIdent.GoName)
		lines = append(lines, fmt.Sprintf("%s = &%s{%s}", embedVarName(name), embedField.Message.GoIdent.GoName, strings.Join(fields, ", ")))
	}
	return lines
}

func zeroValueForField(g *protogen.GeneratedFile, f *protogen.Field) string {
	if f.Desc.IsList() || f.Desc.IsMap() {
		return "nil"
	}
	switch f.Desc.Kind() {
	case protoreflect.BoolKind:
		return "false"
	case protoreflect.EnumKind:
		return "0"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "0"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "0"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "0"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "0"
	case protoreflect.FloatKind:
		return "0"
	case protoreflect.DoubleKind:
		return "0"
	case protoreflect.StringKind:
		return "\"\""
	case protoreflect.BytesKind:
		return "nil"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "nil"
	default:
		return "0"
	}
}

func oneofValueCondition(f *protogen.Field) string {
	if f.Desc.IsList() || f.Desc.IsMap() {
		return "m." + f.GoName + " != nil"
	}
	switch f.Desc.Kind() {
	case protoreflect.BoolKind:
		return "m." + f.GoName
	case protoreflect.StringKind:
		return "m." + f.GoName + " != \"\""
	case protoreflect.BytesKind, protoreflect.MessageKind, protoreflect.GroupKind:
		return "m." + f.GoName + " != nil"
	default:
		return "m." + f.GoName + " != 0"
	}
}

type oneofItem struct {
	PlainField     *protogen.Field
	PlainFieldName string
	OrigFieldName  string
	PbField        *protogen.Field
}

type oneofEnumInfo struct {
	EnumFull             string
	DiscriminatorPlain   string
	UseEnumDiscriminator bool
	FieldToEnums         map[string][]*protogen.EnumValue
}

func enumValsForField(info *oneofEnumInfo, it oneofItem) []*protogen.EnumValue {
	if info == nil {
		return nil
	}
	if it.OrigFieldName != "" {
		if v := info.FieldToEnums[it.OrigFieldName]; len(v) > 0 {
			return v
		}
	}
	if it.PlainField != nil {
		if v := info.FieldToEnums[it.PlainField.GoName]; len(v) > 0 {
			return v
		}
		if v := info.FieldToEnums[string(it.PlainField.Desc.Name())]; len(v) > 0 {
			return v
		}
	}
	return info.FieldToEnums[it.PlainFieldName]
}

type oneofPlainVars struct {
	lines    []string
	fieldVar map[string]string
	discVar  map[string]string
	matchVar map[string]string
}

type oneofVars struct {
	lines    []string
	groupVar map[string]string
}

func groupOneofFields(plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, msgIR *ir.MessageIR) map[string][]oneofItem {
	result := make(map[string][]oneofItem)
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || !fp.Origin.IsOneof || fp.OrigField == nil {
			continue
		}
		pbField := pbFields[fp.OrigField.FieldName]
		if pbField == nil || pbField.Oneof == nil {
			continue
		}
		group := pbField.Oneof.GoName
		result[group] = append(result[group], oneofItem{
			PlainField:     f,
			PlainFieldName: f.GoName,
			OrigFieldName:  fp.OrigField.FieldName,
			PbField:        pbField,
		})
	}
	if msgIR != nil {
		for _, p := range msgIR.OneofPlan {
			if p == nil || !p.Discriminator {
				continue
			}
			plainField := findPlainFieldByProtoName(plainMsg, p.OrigName)
			if plainField == nil {
				continue
			}
			groupName := ""
			for _, oneof := range pbFields {
				if oneof.Oneof != nil && string(oneof.Oneof.Desc.Name()) == p.OrigName {
					groupName = oneof.Oneof.GoName
					break
				}
			}
			if groupName == "" {
				groupName = strcase.ToCamel(p.OrigName)
			}
			for _, name := range p.Fields {
				pbField := pbFields[name]
				if pbField == nil || pbField.Oneof == nil {
					continue
				}
				result[groupName] = append(result[groupName], oneofItem{
					PlainField:     plainField,
					PlainFieldName: plainField.GoName,
					OrigFieldName:  name,
					PbField:        pbField,
				})
			}
		}
	}
	return result
}

func buildOneofEnumInfo(plainMsg *protogen.Message, pbMsg *protogen.Message, msgIR *ir.MessageIR, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, enumValues map[string]*protogen.EnumValue, enumByFull map[string]*protogen.Enum) map[string]map[string]*oneofEnumInfo {
	result := make(map[string]map[string]*oneofEnumInfo)
	if plainMsg == nil || msgIR == nil {
		return result
	}
	oneofProtoByGo := make(map[string]string)
	oneofGoByProto := make(map[string]string)
	if pbMsg != nil {
		for _, o := range pbMsg.Oneofs {
			oneofProtoByGo[o.GoName] = string(o.Desc.Name())
			oneofGoByProto[string(o.Desc.Name())] = o.GoName
		}
	}
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || !fp.Origin.IsOneof || len(fp.Origin.OneofEnums) == 0 {
			continue
		}
		pbField := pbFields[fp.OrigField.FieldName]
		if pbField == nil || pbField.Oneof == nil {
			continue
		}
		group := pbField.Oneof.GoName
		if result[group] == nil {
			result[group] = make(map[string]*oneofEnumInfo)
		}
		for _, val := range fp.Origin.OneofEnums {
			enumFull, valueFull := parseEnumValueFullName(val)
			if enumFull == "" || valueFull == "" {
				continue
			}
			info := result[group][enumFull]
			if info == nil {
				info = &oneofEnumInfo{
					EnumFull:     enumFull,
					FieldToEnums: make(map[string][]*protogen.EnumValue),
				}
				result[group][enumFull] = info
			}
			if ev := lookupEnumValue(enumValues, enumByFull, valueFull); ev != nil {
				info.FieldToEnums[f.GoName] = append(info.FieldToEnums[f.GoName], ev)
				info.FieldToEnums[string(f.Desc.Name())] = append(info.FieldToEnums[string(f.Desc.Name())], ev)
			}
		}
	}

	for _, p := range msgIR.OneofPlan {
		if p == nil || len(p.FieldEnums) == 0 || !p.Discriminator {
			continue
		}
		groupName := oneofGoByProto[p.OrigName]
		if groupName == "" {
			groupName = strcase.ToCamel(p.OrigName)
		}
		if result[groupName] == nil {
			result[groupName] = make(map[string]*oneofEnumInfo)
		}
		for fieldName, vals := range p.FieldEnums {
			for _, v := range vals {
				enumFull, valueFull := parseEnumValueFullName(v)
				if enumFull == "" || valueFull == "" {
					continue
				}
				info := result[groupName][enumFull]
				if info == nil {
					info = &oneofEnumInfo{
						EnumFull:     enumFull,
						FieldToEnums: make(map[string][]*protogen.EnumValue),
					}
					result[groupName][enumFull] = info
				}
				if ev := lookupEnumValue(enumValues, enumByFull, valueFull); ev != nil {
					info.FieldToEnums[fieldName] = append(info.FieldToEnums[fieldName], ev)
				}
			}
		}
	}

	for groupName, enumInfos := range result {
		oneofProto := oneofProtoByGo[groupName]
		var oneofPlan *ir.OneofPlan
		for _, p := range msgIR.OneofPlan {
			if p == nil {
				continue
			}
			if p.OrigName == oneofProto {
				oneofPlan = p
				break
			}
		}
		for enumFull, info := range enumInfos {
			if info == nil || enumFull == "" {
				continue
			}
			if oneofPlan != nil && oneofPlan.EnumDispatch != nil {
				if oneofPlan.EnumDispatch.EnumFullName == enumFull {
					name := oneofPlan.OrigName + "_type"
					if oneofPlan.EnumDispatch.WithPrefix {
						name = oneofPlan.OrigName + "_" + name
					}
					if f := findPlainFieldByProtoName(plainMsg, name); f != nil {
						info.DiscriminatorPlain = f.GoName
					}
				}
				continue
			}
			if oneofPlan != nil && oneofPlan.Discriminator {
				info.UseEnumDiscriminator = true
				name := oneofPlan.OrigName + "_disc"
				if f := findPlainFieldByProtoName(plainMsg, name); f != nil {
					info.DiscriminatorPlain = f.GoName
				}
				continue
			}
			var candidate *ir.FieldPlan
			for _, fp := range msgIR.FieldPlan {
				if fp == nil || fp.Origin.IsOneof || fp.Origin.IsEmbedded || fp.OrigField == nil {
					continue
				}
				if fp.NewField.TypeName == enumFull {
					if candidate == nil {
						candidate = fp
						continue
					}
					if oneofProto != "" && fp.NewField.Name == oneofProto+"_type" {
						candidate = fp
					}
				}
			}
			if candidate != nil {
				if f := findPlainFieldByProtoName(plainMsg, candidate.NewField.Name); f != nil {
					info.DiscriminatorPlain = f.GoName
				}
			}
			if info.DiscriminatorPlain == "" && oneofProto != "" {
				if f := findPlainFieldByProtoName(plainMsg, oneofProto+"_disc"); f != nil {
					info.UseEnumDiscriminator = true
					info.DiscriminatorPlain = f.GoName
				}
			}
		}
	}

	return result
}

func primaryEnumInfo(groupInfos map[string]*oneofEnumInfo) *oneofEnumInfo {
	if len(groupInfos) == 0 {
		return nil
	}
	if len(groupInfos) == 1 {
		for _, info := range groupInfos {
			return info
		}
	}
	for _, info := range groupInfos {
		if info != nil && info.DiscriminatorPlain != "" {
			return info
		}
	}
	for _, info := range groupInfos {
		if info != nil {
			return info
		}
	}
	return nil
}

func parseEnumValueFullName(v string) (enumFull string, valueFull string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", ""
	}
	if !strings.HasPrefix(v, ".") {
		v = "." + v
	}
	lastDot := strings.LastIndex(v, ".")
	if lastDot <= 0 || lastDot == len(v)-1 {
		return "", ""
	}
	return v[:lastDot], v
}

func enumFullConstName(enumFull string) string {
	enumFull = strings.TrimPrefix(enumFull, ".")
	if enumFull == "" {
		return ""
	}
	return "enumFull_" + strcase.ToCamel(strings.ReplaceAll(enumFull, ".", "_"))
}

func lookupEnumValue(enumValues map[string]*protogen.EnumValue, enumByFull map[string]*protogen.Enum, valueFull string) *protogen.EnumValue {
	if valueFull == "" {
		return nil
	}
	if enumValues != nil {
		if ev := enumValues[valueFull]; ev != nil {
			return ev
		}
		if strings.HasPrefix(valueFull, ".") {
			if ev := enumValues[strings.TrimPrefix(valueFull, ".")]; ev != nil {
				return ev
			}
		} else {
			if ev := enumValues["."+valueFull]; ev != nil {
				return ev
			}
		}
	}
	if enumByFull == nil {
		return nil
	}
	enumFull, valueName := parseEnumValueFullName(valueFull)
	if enumFull == "" || valueName == "" {
		return nil
	}
	if idx := strings.LastIndex(valueName, "."); idx >= 0 && idx < len(valueName)-1 {
		valueName = valueName[idx+1:]
	}
	enum := enumByFull[enumFull]
	if enum == nil {
		enum = enumByFull[strings.TrimPrefix(enumFull, ".")]
	}
	if enum == nil {
		return nil
	}
	for _, v := range enum.Values {
		if v == nil {
			continue
		}
		if string(v.Desc.Name()) == valueName {
			return v
		}
	}
	return nil
}

func findPlainFieldByProtoName(plainMsg *protogen.Message, name string) *protogen.Field {
	if plainMsg == nil {
		return nil
	}
	for _, f := range plainMsg.Fields {
		if string(f.Desc.Name()) == name {
			return f
		}
	}
	return nil
}

func findPlainFieldByGoName(plainMsg *protogen.Message, name string) *protogen.Field {
	if plainMsg == nil {
		return nil
	}
	for _, f := range plainMsg.Fields {
		if f.GoName == name {
			return f
		}
	}
	return nil
}

func buildOneofPlainVars(g *protogen.GeneratedFile, plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, oneofEnums map[string]map[string]*oneofEnumInfo, useErr bool, msgIR *ir.MessageIR) oneofPlainVars {
	out := oneofPlainVars{
		lines:    []string{},
		fieldVar: make(map[string]string),
		discVar:  make(map[string]string),
		matchVar: make(map[string]string),
	}
	groups := groupOneofFields(plainMsg, fieldPlans, pbFields, msgIR)
	usedNames := map[string]bool{}
	declaredFields := map[string]bool{}
	for groupName, items := range groups {
		if len(items) == 0 || items[0].PbField == nil || items[0].PbField.Oneof == nil {
			continue
		}
		if info := primaryEnumInfo(oneofEnums[groupName]); info != nil && info.DiscriminatorPlain != "" {
			if existing := out.discVar[info.DiscriminatorPlain]; existing == "" {
				discVar := "disc_" + lowerFirst(info.DiscriminatorPlain)
				if usedNames[discVar] {
					for i := 2; ; i++ {
						candidate := fmt.Sprintf("%s_%d", discVar, i)
						if !usedNames[candidate] {
							discVar = candidate
							break
						}
					}
				}
				if field := findPlainFieldByGoName(plainMsg, info.DiscriminatorPlain); field != nil {
					usedNames[discVar] = true
					out.discVar[info.DiscriminatorPlain] = discVar
					fieldType := getFieldGoTypeForGen(g, field)
					if fp := fieldPlans[string(field.Desc.Name())]; fp != nil && hasOverride(fp) {
						fieldType = getOverrideGoType(g, field, fp)
					}
					out.lines = append(out.lines, "var "+discVar+" "+fieldType)
				}
			}
		}
		for _, it := range items {
			if it.PlainField == nil {
				continue
			}
			if declaredFields[it.PlainFieldName] {
				continue
			}
			varName := "oneof_" + lowerFirst(it.PlainFieldName)
			if usedNames[varName] {
				for i := 2; ; i++ {
					candidate := fmt.Sprintf("%s_%d", varName, i)
					if !usedNames[candidate] {
						varName = candidate
						break
					}
				}
			}
			usedNames[varName] = true
			declaredFields[it.PlainFieldName] = true
			out.fieldVar[it.PlainFieldName] = varName
			fieldType := getFieldGoTypeForGen(g, it.PlainField)
			if fp := fieldPlans[string(it.PlainField.Desc.Name())]; fp != nil && hasOverride(fp) {
				fieldType = getOverrideGoType(g, it.PlainField, fp)
			}
			out.lines = append(out.lines, "var "+varName+" "+fieldType)
		}
		oneofGetter := "m.Get" + groupName + "()"
		matchVar := ""
		if useErr {
			info := primaryEnumInfo(oneofEnums[groupName])
			if info == nil || info.DiscriminatorPlain == "" || !info.UseEnumDiscriminator {
				out.lines = append(out.lines, "switch x := "+oneofGetter+".(type) {")
				for _, it := range items {
					if it.PbField == nil {
						continue
					}
					wrapper := it.PbField.Parent.GoIdent.GoName + "_" + it.PbField.GoName
					varName := out.fieldVar[it.PlainFieldName]
					out.lines = append(out.lines, "case *"+wrapper+": "+varName+" = x."+it.PbField.GoName)
				}
				out.lines = append(out.lines, "}")
				continue
			}
			matchVar = "matched_" + lowerFirst(groupName)
			if usedNames[matchVar] {
				for i := 2; ; i++ {
					candidate := fmt.Sprintf("%s_%d", matchVar, i)
					if !usedNames[candidate] {
						matchVar = candidate
						break
					}
				}
			}
			usedNames[matchVar] = true
			out.matchVar[groupName] = matchVar
			out.lines = append(out.lines, "var "+matchVar+" bool")
		}
		out.lines = append(out.lines, "switch x := "+oneofGetter+".(type) {")
		for _, it := range items {
			if it.PbField == nil {
				continue
			}
			wrapper := it.PbField.Parent.GoIdent.GoName + "_" + it.PbField.GoName
			varName := out.fieldVar[it.PlainFieldName]
			discAssign := ""
			if infos := oneofEnums[groupName]; len(infos) > 0 {
				for _, info := range infos {
					if info == nil {
						continue
					}
					if info.DiscriminatorPlain == "" || !info.UseEnumDiscriminator {
						continue
					}
					discVar := out.discVar[info.DiscriminatorPlain]
					if discVar == "" {
						discVar = info.DiscriminatorPlain
					}
					if vals := enumValsForField(info, it); len(vals) > 0 {
						enumIdent := g.QualifiedGoIdent(vals[0].GoIdent)
						newDisc := g.QualifiedGoIdent(protogen.GoIdent{GoName: "NewDiscriminator", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/oneoff"})
						discAssign = "; " + discVar + " = " + newDisc + "(" + enumIdent + ")"
						if matchVar != "" {
							discAssign += "; " + matchVar + " = true"
						}
						break
					}
				}
			}
			out.lines = append(out.lines, "case *"+wrapper+": "+varName+" = x."+it.PbField.GoName+discAssign)
		}
		out.lines = append(out.lines, "}")
	}
	return out
}

func buildOneofVars(g *protogen.GeneratedFile, plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, oneofEnums map[string]map[string]*oneofEnumInfo, useErr bool, msgIR *ir.MessageIR) oneofVars {
	out := oneofVars{
		lines:    []string{},
		groupVar: make(map[string]string),
	}
	groups := groupOneofFields(plainMsg, fieldPlans, pbFields, msgIR)
	usedNames := map[string]bool{}
	for groupName, items := range groups {
		if len(items) == 0 || items[0].PbField == nil || items[0].PbField.Oneof == nil {
			continue
		}
		varName := "oneof_" + lowerFirst(groupName)
		if usedNames[varName] {
			for i := 2; ; i++ {
				candidate := fmt.Sprintf("%s_%d", varName, i)
				if !usedNames[candidate] {
					varName = candidate
					break
				}
			}
		}
		usedNames[varName] = true
		out.groupVar[groupName] = varName
		parent := items[0].PbField.Parent.GoIdent.GoName
		oneofType := "is" + parent + "_" + groupName
		out.lines = append(out.lines, "var "+varName+" "+oneofType)
		infos := oneofEnums[groupName]
		if len(infos) > 0 {
			info := primaryEnumInfo(infos)
			if info != nil && info.DiscriminatorPlain != "" {
				if info.UseEnumDiscriminator {
					anyVals := false
					for _, it := range items {
						for _, inf := range infos {
							if inf == nil {
								continue
							}
							if len(enumValsForField(inf, it)) > 0 {
								anyVals = true
								break
							}
						}
						if anyVals {
							break
						}
					}
					if !anyVals {
						goto fallback
					}
					parseDisc := g.QualifiedGoIdent(protogen.GoIdent{GoName: "ParseDiscriminator", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/oneoff"})
					if useErr {
						errf := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Errorf", GoImportPath: "fmt"})
						out.lines = append(out.lines, "if disc, err := "+parseDisc+"(m."+info.DiscriminatorPlain+"); err != nil { return nil, err } else {")
						out.lines = append(out.lines, "matched := false")
						for _, it := range items {
							if it.PbField == nil {
								continue
							}
							vals := []*protogen.EnumValue{}
							for _, inf := range infos {
								if inf == nil {
									continue
								}
								vals = append(vals, enumValsForField(inf, it)...)
							}
							if len(vals) == 0 {
								continue
							}
							wrapper := it.PbField.Parent.GoIdent.GoName + "_" + it.PbField.GoName
							conds := []string{}
							for _, v := range vals {
								enumFull := string(v.Desc.Parent().FullName())
								enumConst := enumFullConstName(enumFull)
								conds = append(conds, fmt.Sprintf("(string(disc.Descriptor().FullName()) == %s && disc.Number() == %d)", enumConst, v.Desc.Number()))
							}
							pbType := getFieldGoTypeForGen(g, it.PbField)
							out.lines = append(out.lines, "if "+strings.Join(conds, " || ")+" { if v, ok := m."+it.PlainFieldName+".("+pbType+"); ok { "+varName+" = &"+wrapper+"{"+it.PbField.GoName+": v}; matched = true } else { return nil, "+errf+"(\"oneof %s discriminator payload type mismatch\", \""+groupName+"\") } }")
						}
						out.lines = append(out.lines, "if !matched { return nil, "+errf+"(\"oneof %s discriminator mismatch\", \""+groupName+"\") }")
						out.lines = append(out.lines, "}")
						continue
					}
					out.lines = append(out.lines, "if disc, err := "+parseDisc+"(m."+info.DiscriminatorPlain+"); err == nil {")
					for _, it := range items {
						if it.PbField == nil {
							continue
						}
						vals := []*protogen.EnumValue{}
						for _, inf := range infos {
							if inf == nil {
								continue
							}
							vals = append(vals, enumValsForField(inf, it)...)
						}
						if len(vals) == 0 {
							continue
						}
						wrapper := it.PbField.Parent.GoIdent.GoName + "_" + it.PbField.GoName
						conds := []string{}
						for _, v := range vals {
							enumFull := string(v.Desc.Parent().FullName())
							enumConst := enumFullConstName(enumFull)
							conds = append(conds, fmt.Sprintf("(string(disc.Descriptor().FullName()) == %s && disc.Number() == %d)", enumConst, v.Desc.Number()))
						}
						pbType := getFieldGoTypeForGen(g, it.PbField)
						if useErr {
							errf := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Errorf", GoImportPath: "fmt"})
							out.lines = append(out.lines, "if "+strings.Join(conds, " || ")+" { if v, ok := m."+it.PlainFieldName+".("+pbType+"); ok { "+varName+" = &"+wrapper+"{"+it.PbField.GoName+": v}; matched = true } else { return nil, "+errf+"(\"oneof %s discriminator payload type mismatch\", \""+groupName+"\") } }")
						} else {
							out.lines = append(out.lines, "if "+strings.Join(conds, " || ")+" { if v, ok := m."+it.PlainFieldName+".("+pbType+"); ok { "+varName+" = &"+wrapper+"{"+it.PbField.GoName+": v} } }")
						}
					}
					out.lines = append(out.lines, "}")
					continue
				}
				out.lines = append(out.lines, "switch m."+info.DiscriminatorPlain+" {")
				for _, it := range items {
					if it.PbField == nil {
						continue
					}
					vals := enumValsForField(info, it)
					if len(vals) == 0 {
						continue
					}
					wrapper := it.PbField.Parent.GoIdent.GoName + "_" + it.PbField.GoName
					caseConsts := []string{}
					for _, v := range vals {
						caseConsts = append(caseConsts, g.QualifiedGoIdent(v.GoIdent))
					}
					out.lines = append(out.lines, "case "+strings.Join(caseConsts, ", ")+": "+varName+" = &"+wrapper+"{"+it.PbField.GoName+": m."+it.PlainFieldName+"}")
				}
				out.lines = append(out.lines, "}")
				continue
			}
		}
	fallback:
		for _, it := range items {
			if it.PbField == nil {
				continue
			}
			wrapper := it.PbField.Parent.GoIdent.GoName + "_" + it.PbField.GoName
			cond := ""
			if it.PlainField != nil {
				cond = oneofValueCondition(it.PlainField)
			}
			if cond == "" {
				cond = "m." + it.PlainFieldName + " != nil"
			}
			out.lines = append(out.lines, fmt.Sprintf("if %s { %s = &%s{%s: m.%s} }", cond, varName, wrapper, it.PbField.GoName, it.PlainFieldName))
		}
	}
	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
