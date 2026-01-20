package generator

import (
	"fmt"
	"strings"

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

func generateConverters(g *protogen.GeneratedFile, plainMsg *protogen.Message, pbMsg *protogen.Message, msgIR *ir.MessageIR, generatedEnums map[string]struct{}) {
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

	generateIntoPlain(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources)
	generateIntoPlainErr(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources)
	generateIntoPb(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources)
	generateIntoPbErr(g, plainMsg, pbMsg, fieldPlans, pbFields, embedSources)
}

func generateIntoPlain(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, false, false)
	g.P("func (m *", pbMsg.GoIdent.GoName, ") IntoPlain(", strings.Join(params, ", "), ") *", plainMsg.GoIdent.GoName, " {")
	g.P("\tif m == nil { return nil }")

	oneofVars := buildOneofPlainVars(g, plainMsg, fieldPlans, pbFields)
	for _, line := range oneofVars.lines {
		g.P("\t" + line)
	}

	g.P("\treturn &", plainMsg.GoIdent.GoName, "{")
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil {
			continue
		}
		if fp.Origin.IsOneof {
			if v, ok := oneofVars.fieldVar[f.GoName]; ok {
				g.P("\t\t", f.GoName, ": ", v, ",")
			}
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

func generateIntoPlainErr(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, true, false)
	g.P("func (m *", pbMsg.GoIdent.GoName, ") IntoPlainErr(", strings.Join(params, ", "), ") (*", plainMsg.GoIdent.GoName, ", error) {")
	g.P("\tif m == nil { return nil, nil }")

	oneofVars := buildOneofPlainVars(g, plainMsg, fieldPlans, pbFields)
	for _, line := range oneofVars.lines {
		g.P("\t" + line)
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
		if fp.Origin.IsOneof {
			if v, ok := oneofVars.fieldVar[f.GoName]; ok {
				g.P("\t\t", f.GoName, ": ", v, ",")
			}
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

func generateIntoPb(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, false, true)
	g.P("func (m *", plainMsg.GoIdent.GoName, ") IntoPb(", strings.Join(params, ", "), ") *", pbMsg.GoIdent.GoName, " {")
	g.P("\tif m == nil { return nil }")

	// build embedded structs
	embedStructs := buildEmbedStructs(g, plainMsg, fieldPlans, pbFields, embedSources)
	for _, line := range embedStructs {
		g.P("\t" + line)
	}

	oneofVars := buildOneofVars(g, plainMsg, fieldPlans, pbFields)
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

func generateIntoPbErr(g *protogen.GeneratedFile, plainMsg, pbMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field, embedSources map[string]*protogen.Field) {
	params := buildCasterParams(g, plainMsg, fieldPlans, pbFields, true, true)
	g.P("func (m *", plainMsg.GoIdent.GoName, ") IntoPbErr(", strings.Join(params, ", "), ") (*", pbMsg.GoIdent.GoName, ", error) {")
	g.P("\tif m == nil { return nil, nil }")

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

	oneofVars := buildOneofVars(g, plainMsg, fieldPlans, pbFields)
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
	casterIdent := g.QualifiedGoIdent(protogen.GoIdent{GoName: "Caster", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
	if withErr {
		casterIdent = g.QualifiedGoIdent(protogen.GoIdent{GoName: "CasterErr", GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast"})
	}
	for _, f := range plainMsg.Fields {
		fp := fieldPlans[string(f.Desc.Name())]
		if fp == nil || fp.OrigField == nil {
			continue
		}
		if !hasOverride(fp) {
			continue
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
	PbField        *protogen.Field
}

type oneofPlainVars struct {
	lines    []string
	fieldVar map[string]string
}

type oneofVars struct {
	lines    []string
	groupVar map[string]string
}

func groupOneofFields(plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field) map[string][]oneofItem {
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
			PbField:        pbField,
		})
	}
	return result
}

func buildOneofPlainVars(g *protogen.GeneratedFile, plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field) oneofPlainVars {
	out := oneofPlainVars{
		lines:    []string{},
		fieldVar: make(map[string]string),
	}
	groups := groupOneofFields(plainMsg, fieldPlans, pbFields)
	usedNames := map[string]bool{}
	for groupName, items := range groups {
		if len(items) == 0 || items[0].PbField == nil || items[0].PbField.Oneof == nil {
			continue
		}
		for _, it := range items {
			if it.PlainField == nil {
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
			out.fieldVar[it.PlainFieldName] = varName
			out.lines = append(out.lines, "var "+varName+" "+getFieldGoTypeForGen(g, it.PlainField))
		}
		oneofGetter := "m.Get" + groupName + "()"
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
	}
	return out
}

func buildOneofVars(g *protogen.GeneratedFile, plainMsg *protogen.Message, fieldPlans map[string]*ir.FieldPlan, pbFields map[string]*protogen.Field) oneofVars {
	out := oneofVars{
		lines:    []string{},
		groupVar: make(map[string]string),
	}
	groups := groupOneofFields(plainMsg, fieldPlans, pbFields)
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
