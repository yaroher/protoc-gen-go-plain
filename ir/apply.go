package ir

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// ApplyIR applies a transformation plan to the plugin and returns a new plugin.
func ApplyIR(p *protogen.Plugin, plan *IR) (*protogen.Plugin, error) {
	if p == nil {
		return nil, fmt.Errorf("nil plugin")
	}
	if plan == nil {
		return nil, fmt.Errorf("nil IR plan")
	}

	newRequest := proto.Clone(p.Request).(*pluginpb.CodeGeneratorRequest)
	transformed := make([]*descriptorpb.FileDescriptorProto, 0, len(newRequest.GetProtoFile()))
	for _, f := range newRequest.GetProtoFile() {
		tf, err := transformFile(plan, f)
		if err != nil {
			return nil, err
		}
		transformed = append(transformed, tf)
	}
	newRequest.ProtoFile = transformed

	// Skip validation here; import resolution requires a resolver.

	newPlugin, err := protogen.Options{}.New(newRequest)
	if err != nil {
		return nil, err
	}

	supportedFeatures := p.SupportedFeatures
	supportedMin := p.SupportedEditionsMinimum
	supportedMax := p.SupportedEditionsMaximum

	*p = *newPlugin
	p.SupportedFeatures = supportedFeatures
	p.SupportedEditionsMinimum = supportedMin
	p.SupportedEditionsMaximum = supportedMax

	return p, nil
}

func transformFile(plan *IR, file *descriptorpb.FileDescriptorProto) (*descriptorpb.FileDescriptorProto, error) {
	cloned := proto.Clone(file).(*descriptorpb.FileDescriptorProto)

	if len(cloned.GetMessageType()) > 0 {
		newMsgs := make([]*descriptorpb.DescriptorProto, 0, len(cloned.GetMessageType()))
		for _, msg := range cloned.GetMessageType() {
			newMsgs = append(newMsgs, transformMessage(plan, cloned.GetPackage(), "", msg))
		}
		cloned.MessageType = newMsgs
	}

	applyRenameIndex(plan.Renames, cloned)
	return cloned, nil
}

func transformMessage(plan *IR, pkg, parent string, msg *descriptorpb.DescriptorProto) *descriptorpb.DescriptorProto {
	cloned := proto.Clone(msg).(*descriptorpb.DescriptorProto)
	full := fullName(pkg, parent, msg.GetName())
	msgIR := plan.Messages[full]

	if msgIR != nil && msgIR.Generate {
		cloned.Name = proto.String(msgIR.NewName)
		oneofDecls, indexMap := buildOneofDecls(msg, msgIR)
		cloned.Field = buildFieldsFromPlan(msgIR.FieldPlan, indexMap)
		cloned.OneofDecl = oneofDecls
		if len(msgIR.GeneratedEnums) > 0 {
			cloned.EnumType = append(cloned.EnumType, buildEnumDescs(msgIR.GeneratedEnums)...)
		}
	}

	if len(cloned.GetNestedType()) > 0 {
		newNested := make([]*descriptorpb.DescriptorProto, 0, len(cloned.GetNestedType()))
		nextParent := joinPath(parent, msg.GetName())
		for _, nested := range cloned.GetNestedType() {
			newNested = append(newNested, transformMessage(plan, pkg, nextParent, nested))
		}
		cloned.NestedType = newNested
	}

	return cloned
}

func buildFieldsFromPlan(plans []*FieldPlan, indexMap map[int32]int32) []*descriptorpb.FieldDescriptorProto {
	out := make([]*descriptorpb.FieldDescriptorProto, 0, len(plans))
	for _, fp := range plans {
		if fp == nil {
			continue
		}
		f := &descriptorpb.FieldDescriptorProto{
			Name:    proto.String(fp.NewField.Name),
			Number:  proto.Int32(fp.NewField.Number),
			Label:   fp.NewField.Label.Enum(),
			Type:    fp.NewField.Type.Enum(),
			Options: fp.NewField.Options,
		}
		if fp.NewField.TypeName != "" {
			tn := fp.NewField.TypeName
			if !strings.HasPrefix(tn, ".") {
				tn = "." + tn
			}
			f.TypeName = proto.String(tn)
		}
		if fp.NewField.OneofIndex != nil {
			idx := *fp.NewField.OneofIndex
			if indexMap != nil {
				if newIdx, ok := indexMap[idx]; ok {
					idx = newIdx
				} else {
					idx = -1
				}
			}
			if idx >= 0 {
				f.OneofIndex = proto.Int32(idx)
			}
		}
		if fp.NewField.Proto3Optional {
			f.Proto3Optional = proto.Bool(true)
		}
		out = append(out, f)
	}
	return out
}

func buildOneofDecls(msg *descriptorpb.DescriptorProto, msgIR *MessageIR) ([]*descriptorpb.OneofDescriptorProto, map[int32]int32) {
	if msgIR == nil || len(msgIR.OneofPlan) == 0 {
		indexMap := make(map[int32]int32, len(msg.GetOneofDecl()))
		for i := range msg.GetOneofDecl() {
			indexMap[int32(i)] = int32(i)
		}
		return msg.GetOneofDecl(), indexMap
	}
	planByName := make(map[string]*OneofPlan)
	for _, p := range msgIR.OneofPlan {
		planByName[p.OrigName] = p
	}

	kept := make([]*descriptorpb.OneofDescriptorProto, 0)
	indexMap := make(map[int32]int32)
	newIdx := int32(0)
	for _, oneof := range msg.GetOneofDecl() {
		p := planByName[oneof.GetName()]
		flatten := p != nil && (p.Embed || p.EnumDispatch != nil || p.Discriminator)
		if flatten {
			continue
		}
		kept = append(kept, proto.Clone(oneof).(*descriptorpb.OneofDescriptorProto))
		// original index is implied by order in original oneof decl slice
	}
	for i := range msg.GetOneofDecl() {
		oneof := msg.GetOneofDecl()[i]
		p := planByName[oneof.GetName()]
		flatten := p != nil && (p.Embed || p.EnumDispatch != nil || p.Discriminator)
		if flatten {
			continue
		}
		indexMap[int32(i)] = newIdx
		newIdx++
	}
	return kept, indexMap
}

func buildEnumDescs(enums []*EnumSpec) []*descriptorpb.EnumDescriptorProto {
	out := make([]*descriptorpb.EnumDescriptorProto, 0, len(enums))
	for _, e := range enums {
		if e == nil {
			continue
		}
		ed := &descriptorpb.EnumDescriptorProto{Name: proto.String(e.Name)}
		for _, v := range e.Values {
			ed.Value = append(ed.Value, &descriptorpb.EnumValueDescriptorProto{
				Name:   proto.String(v.Name),
				Number: proto.Int32(v.Number),
			})
		}
		out = append(out, ed)
	}
	return out
}

func applyRenameIndex(renames *RenameIndex, file *descriptorpb.FileDescriptorProto) {
	if renames == nil || len(renames.Messages) == 0 {
		return
	}
	for _, msg := range file.GetMessageType() {
		applyRenameToMessage(renames, msg)
	}
}

func applyRenameToMessage(renames *RenameIndex, msg *descriptorpb.DescriptorProto) {
	for _, field := range msg.GetField() {
		if field.TypeName != nil {
			oldName := *field.TypeName
			for oldFull, newFull := range renames.Messages {
				if oldName == oldFull || strings.HasPrefix(oldName, oldFull+".") {
					field.TypeName = proto.String(newFull + strings.TrimPrefix(oldName, oldFull))
					break
				}
			}
		}
	}
	for _, nested := range msg.GetNestedType() {
		applyRenameToMessage(renames, nested)
	}
}
