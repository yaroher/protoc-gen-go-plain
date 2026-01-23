package generator

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/crf"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/typepb"
)

type AliasInfo struct {
	Field   *protogen.Field
	Kind    typepb.Field_Kind
	TypeUrl string
}

type Builder struct {
	g *Generator

	messagesByFullName map[string]*protogen.Message
	enumsByFullName    map[string]*protogen.Enum

	generatedMessages map[string]bool
	aliases           map[string]*AliasInfo

	overrides []*goplain.TypeOverride
}

func newBuilder(g *Generator) *Builder {
	return &Builder{
		g:                  g,
		messagesByFullName: make(map[string]*protogen.Message),
		enumsByFullName:    make(map[string]*protogen.Enum),
		generatedMessages:  make(map[string]bool),
		aliases:            make(map[string]*AliasInfo),
		overrides:          g.GetOverrides(),
	}
}

func (b *Builder) collectSymbols() {
	for _, file := range b.g.Plugin.Files {
		for _, msg := range file.Messages {
			b.walkMessage(msg)
		}
		for _, enum := range file.Enums {
			b.enumsByFullName[string(enum.Desc.FullName())] = enum
		}
	}
}

func (b *Builder) walkMessage(msg *protogen.Message) {
	b.messagesByFullName[string(msg.Desc.FullName())] = msg
	for _, enum := range msg.Enums {
		b.enumsByFullName[string(enum.Desc.FullName())] = enum
	}
	for _, child := range msg.Messages {
		b.walkMessage(child)
	}
}

func (b *Builder) shouldGenerateMessage(msg *protogen.Message) bool {
	if msg.Desc.IsMapEntry() {
		return false
	}
	if opts, ok := messageOptions(msg.Desc.Options()); ok {
		return opts.GetGenerate()
	}
	return true
}

func (b *Builder) collectAliases() error {
	for _, msg := range b.messagesByFullName {
		opts, ok := messageOptions(msg.Desc.Options())
		if !ok || !opts.GetTypeAlias() {
			continue
		}
		aliasFieldName := strings.TrimSpace(opts.GetTypeAliasField())
		if aliasFieldName == "" {
			aliasFieldName = "value"
		}
		var aliasField *protogen.Field
		for _, field := range msg.Fields {
			if string(field.Desc.Name()) == aliasFieldName {
				aliasField = field
				break
			}
		}
		if aliasField == nil {
			return fmt.Errorf("type alias message %s: field %q not found", msg.Desc.FullName(), aliasFieldName)
		}
		kind, typeURL := fieldKindAndURL(aliasField)
		b.aliases[string(msg.Desc.FullName())] = &AliasInfo{
			Field:   aliasField,
			Kind:    kind,
			TypeUrl: typeURL,
		}
	}
	return nil
}

func (b *Builder) markGeneratedMessages() {
	for _, file := range b.g.Plugin.Files {
		if !file.Generate {
			continue
		}
		for _, msg := range file.Messages {
			b.markGeneratedMessage(msg)
		}
	}
}

func (b *Builder) markGeneratedMessage(msg *protogen.Message) {
	if msg == nil {
		return
	}
	if b.shouldGenerateMessage(msg) {
		b.generatedMessages[string(msg.Desc.FullName())] = true
	}
	for _, child := range msg.Messages {
		b.markGeneratedMessage(child)
	}
}

func (b *Builder) plainFullName(fullName protoreflect.FullName) string {
	parts := strings.Split(string(fullName), ".")
	if len(parts) == 0 {
		return string(fullName) + b.g.suffix
	}
	parts[len(parts)-1] = parts[len(parts)-1] + b.g.suffix
	return strings.Join(parts, ".")
}

func (b *Builder) flattenMessageFields(msg *protogen.Message, path []*protogen.Field, prefix string, inheritedOneof *protogen.Oneof) ([]*FieldWrapper, error) {
	var out []*FieldWrapper

	for _, field := range msg.Fields {
		oneof := inheritedOneof
		if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
			oneof = field.Oneof
		}

		oneofPrefix := ""
		if oneof != nil {
			if oOpts, ok := oneofOptions(oneof.Desc.Options()); ok && oOpts.GetEmbedWithPrefix() {
				oneofPrefix = string(oneof.Desc.Name())
			}
		}

		fieldPrefix := prefix
		if oneofPrefix != "" {
			fieldPrefix = applyPrefix(fieldPrefix, oneofPrefix)
		}

		fOpts, _ := fieldOptions(field.Desc.Options())

		if fOpts != nil && fOpts.GetSerialize() {
			wrapper := b.buildFieldWrapper(field, appendPath(path, field), applyPrefix(fieldPrefix, string(field.Desc.Name())), oneof, true)
			out = append(out, wrapper)
			continue
		}

		if fOpts != nil && fOpts.GetEmbed() && field.Message != nil && !field.Desc.IsMap() {
			embedPrefix := ""
			if fOpts.GetEmbedWithPrefix() {
				embedPrefix = string(field.Desc.Name())
			}
			nextPrefix := fieldPrefix
			if embedPrefix != "" {
				nextPrefix = applyPrefix(nextPrefix, embedPrefix)
			}
			nested, err := b.flattenMessageFields(field.Message, appendPath(path, field), nextPrefix, oneof)
			if err != nil {
				return nil, err
			}
			out = append(out, nested...)
			continue
		}

		wrapper := b.buildFieldWrapper(field, appendPath(path, field), applyPrefix(fieldPrefix, string(field.Desc.Name())), oneof, false)
		out = append(out, wrapper)
	}

	if opts, ok := messageOptions(msg.Desc.Options()); ok {
		for _, virtual := range opts.GetVirtualFields() {
			clone := *virtual
			clone.Name = applyPrefix(prefix, clone.GetName())
			out = append(out, &FieldWrapper{
				Field:   &clone,
				Source:  nil,
				Path:    nil,
				Oneof:   nil,
				Origins: []FieldOrigin{{FullName: fmt.Sprintf("%s.%s", msg.Desc.FullName(), clone.GetName())}},
			})
		}
	}

	return out, nil
}

func (b *Builder) buildFieldWrapper(field *protogen.Field, path []*protogen.Field, name string, oneof *protogen.Oneof, forceBytes bool) *FieldWrapper {
	kind, typeURL := fieldKindAndURL(field)
	cardinality := cardinalityFromField(field)

	fOpts, _ := fieldOptions(field.Desc.Options())
	if fOpts != nil {
		if fOpts.GetEnumAsString() {
			kind = typepb.Field_TYPE_STRING
			typeURL = ""
		} else if fOpts.GetEnumAsInt() {
			kind = typepb.Field_TYPE_INT32
			typeURL = ""
		}
	}

	if field.Desc.Kind() == protoreflect.MessageKind {
		if alias, ok := b.aliases[string(field.Desc.Message().FullName())]; ok {
			kind = alias.Kind
			typeURL = alias.TypeUrl
		}
	}

	if forceBytes {
		kind = typepb.Field_TYPE_BYTES
		typeURL = ""
	}

	return &FieldWrapper{
		Field: &typepb.Field{
			Kind:        kind,
			Cardinality: cardinality,
			Number:      0,
			Name:        name,
			TypeUrl:     typeURL,
			JsonName:    name,
		},
		Source:  field,
		Path:    path,
		Oneof:   oneof,
		Origins: []FieldOrigin{{Source: field, Path: path, Oneof: oneof, FullName: string(field.Desc.FullName())}},
	}
}

func (b *Builder) resolveCollisions(fields []*FieldWrapper) ([]*FieldWrapper, *crf.CRF, error) {
	seen := make(map[string]*FieldWrapper, len(fields))
	var out []*FieldWrapper
	var entries []crf.Entry
	entryIndex := make(map[string]int)

	for _, fw := range fields {
		if fw == nil || fw.Field == nil {
			continue
		}
		name := fw.Field.GetName()
		if existing, ok := seen[name]; ok {
			existing.Origins = append(existing.Origins, fw.Origins...)
			if !b.g.Settings.EnableCRF {
				return nil, nil, crf.ErrFieldCollision{
					Field:   name,
					Sources: originNames(existing.Origins),
				}
			}
			idx, ok := entryIndex[name]
			if !ok {
				entries = append(entries, crf.Entry{Field: name})
				idx = len(entries) - 1
				entryIndex[name] = idx
			}
			appendOrigins(&entries[idx], existing.Origins)
			continue
		}
		seen[name] = fw
		out = append(out, fw)
	}

	var crfMeta *crf.CRF
	if len(entries) > 0 {
		crfMeta = &crf.CRF{Entries: entries}
	}
	return out, crfMeta, nil
}

func (b *Builder) buildMessageType(msg *protogen.Message) (*TypeWrapper, error) {
	if msg == nil {
		return nil, nil
	}
	if !b.shouldGenerateMessage(msg) {
		return nil, nil
	}

	fullName := string(msg.Desc.FullName())
	plainName := b.plainFullName(msg.Desc.FullName())

	if alias, ok := b.aliases[fullName]; ok && alias != nil {
		t := &typepb.Type{
			Name: plainName,
		}
		return &TypeWrapper{
			Type:   t,
			Fields: nil,
		}, nil
	}

	fields, err := b.flattenMessageFields(msg, nil, "", nil)
	if err != nil {
		return nil, err
	}
	fields, crfMeta, err := b.resolveCollisions(fields)
	if err != nil {
		return nil, err
	}

	for i, fw := range fields {
		if fw != nil && fw.Field != nil {
			fw.Field.Number = int32(i + 1)
		}
	}

	var typeFields []*typepb.Field
	for _, fw := range fields {
		if fw != nil && fw.Field != nil {
			typeFields = append(typeFields, fw.Field)
		}
	}

	t := &typepb.Type{
		Name:   plainName,
		Fields: typeFields,
	}

	return &TypeWrapper{
		Type:   t,
		Fields: fields,
		CRF:    crfMeta,
	}, nil
}

func originNames(origins []FieldOrigin) []string {
	if len(origins) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(origins))
	var out []string
	for _, origin := range origins {
		name := origin.FullName
		if name == "" && origin.Source != nil {
			name = string(origin.Source.Desc.FullName())
		}
		if name == "" && len(origin.Path) > 0 {
			name = string(origin.Path[len(origin.Path)-1].Desc.FullName())
		}
		if name == "" {
			continue
		}
		if !seen[name] {
			out = append(out, name)
			seen[name] = true
		}
	}
	return out
}

func appendOrigins(entry *crf.Entry, origins []FieldOrigin) {
	if entry == nil || len(origins) == 0 {
		return
	}
	seen := make(map[string]bool, len(entry.Sources))
	for _, src := range entry.Sources {
		seen[src.Path] = true
	}
	for _, name := range originNames(origins) {
		if name == "" || seen[name] {
			continue
		}
		entry.Sources = append(entry.Sources, crf.Source{Path: name})
		seen[name] = true
	}
}

func appendPath(path []*protogen.Field, field *protogen.Field) []*protogen.Field {
	if field == nil {
		return append([]*protogen.Field(nil), path...)
	}
	out := make([]*protogen.Field, len(path)+1)
	copy(out, path)
	out[len(path)] = field
	return out
}

func applyPrefix(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "_" + name
}

func fieldKindAndURL(field *protogen.Field) (typepb.Field_Kind, string) {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return typepb.Field_TYPE_BOOL, ""
	case protoreflect.EnumKind:
		return typepb.Field_TYPE_ENUM, typeURL(string(field.Desc.Enum().FullName()))
	case protoreflect.Int32Kind:
		return typepb.Field_TYPE_INT32, ""
	case protoreflect.Sint32Kind:
		return typepb.Field_TYPE_SINT32, ""
	case protoreflect.Sfixed32Kind:
		return typepb.Field_TYPE_SFIXED32, ""
	case protoreflect.Uint32Kind:
		return typepb.Field_TYPE_UINT32, ""
	case protoreflect.Fixed32Kind:
		return typepb.Field_TYPE_FIXED32, ""
	case protoreflect.Int64Kind:
		return typepb.Field_TYPE_INT64, ""
	case protoreflect.Sint64Kind:
		return typepb.Field_TYPE_SINT64, ""
	case protoreflect.Sfixed64Kind:
		return typepb.Field_TYPE_SFIXED64, ""
	case protoreflect.Uint64Kind:
		return typepb.Field_TYPE_UINT64, ""
	case protoreflect.Fixed64Kind:
		return typepb.Field_TYPE_FIXED64, ""
	case protoreflect.FloatKind:
		return typepb.Field_TYPE_FLOAT, ""
	case protoreflect.DoubleKind:
		return typepb.Field_TYPE_DOUBLE, ""
	case protoreflect.StringKind:
		return typepb.Field_TYPE_STRING, ""
	case protoreflect.BytesKind:
		return typepb.Field_TYPE_BYTES, ""
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return typepb.Field_TYPE_MESSAGE, typeURL(string(field.Desc.Message().FullName()))
	default:
		return typepb.Field_TYPE_UNKNOWN, ""
	}
}

func cardinalityFromField(field *protogen.Field) typepb.Field_Cardinality {
	switch field.Desc.Cardinality() {
	case protoreflect.Required:
		return typepb.Field_CARDINALITY_REQUIRED
	case protoreflect.Repeated:
		return typepb.Field_CARDINALITY_REPEATED
	case protoreflect.Optional:
		return typepb.Field_CARDINALITY_OPTIONAL
	default:
		return typepb.Field_CARDINALITY_UNKNOWN
	}
}

func typeURL(fullName string) string {
	if fullName == "" {
		return ""
	}
	return "type.googleapis.com/" + fullName
}
