package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/generator/empath"
	"github.com/yaroher/protoc-gen-go-plain/generator/marker"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/sourcecontextpb"
	"google.golang.org/protobuf/types/known/typepb"
)

type Generator struct {
	Settings *PluginSettings
	Plugin   *protogen.Plugin
	suffix   string

	overrides   []*goplain.TypeOverride
	typeAliases map[string]typeAliasInfo
}

type Option func(*Generator) error

func WithPlainSuffix(suffix string) Option {
	return func(g *Generator) error {
		g.suffix = suffix
		return nil
	}
}

func WithTypeOverrides(overrides []*goplain.TypeOverride) Option {
	return func(g *Generator) error {
		g.overrides = overrides
		return nil
	}
}

func NewGenerator(p *protogen.Plugin, opts ...Option) (*Generator, error) {
	settings, err := NewPluginSettingsFromPlugin(p)
	if err != nil {
		return nil, err
	}
	g := &Generator{
		Settings:    settings,
		Plugin:      p,
		suffix:      "Plain",
		typeAliases: make(map[string]typeAliasInfo),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(g); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (g *Generator) GetOverrides() []*goplain.TypeOverride {
	return g.overrides
}

func (g *Generator) AddOverride(override *goplain.TypeOverride) {
	g.overrides = append(g.overrides, override)
}

type ResultMessage struct {
}
type TypePbIR struct {
	File     *protogen.File
	Messages map[string]*typepb.Type
}

type typeAliasInfo struct {
	fieldName   string
	fieldGoName string
	kind        typepb.Field_Kind
	pbTypeName  string
}

const (
	// ---------------------------------
	embedMarker  = "embed"
	prefixMarker = "prefix"
	trueVal      = "true"
	//----------------------------------
)

func (g *Generator) Collect() []*TypePbIR {
	result := make([]*TypePbIR, 0)
	l := logger.Logger.Named("Collect")
	for _, file := range g.Plugin.Files {
		if strings.Contains(string(file.Desc.FullName()), "goplain") ||
			strings.Contains(string(file.Desc.FullName()), "google.protobuf") {
			continue
		}

		l.Debug("file", zap.String("full_path", string(file.Desc.FullName())))
		newResult := &TypePbIR{
			File:     file,
			Messages: make(map[string]*typepb.Type),
		}
		fOpt := file.Desc.Options().(*descriptorpb.FileOptions)
		fGen := proto.GetExtension(fOpt, goplain.E_File).(*goplain.FileOptions)
		fileOverrides := fGen.GetGoTypesOverrides()
		for _, vm := range fGen.GetVirtualTypes() {
			// TODO: VALIDATE
			mvApply := &typepb.Type{
				Name:    fmt.Sprintf("%s.%s", string(file.Desc.Package()), string(vm.Name)),
				Fields:  vm.Fields,
				Oneofs:  vm.Oneofs,
				Options: vm.Options,
				Syntax:  typepb.Syntax_SYNTAX_PROTO3,
				SourceContext: &sourcecontextpb.SourceContext{
					FileName: string(file.Desc.Package()) + "." + string(file.Desc.FullName()),
				},
			}
			newResult.Messages[vm.Name] = mvApply
			l.Debug("virtual_type", zap.String("full_path", vm.Name))
		}
		for _, message := range file.Messages {
			msgOpt := message.Desc.Options().(*descriptorpb.MessageOptions)
			msgGen := proto.GetExtension(msgOpt, goplain.E_Message).(*goplain.MessageOptions)
			//if !msgGen.GetGenerate() {
			//	continue
			//}

			resMessage := &typepb.Type{
				Name:    string(message.Desc.FullName()),
				Fields:  []*typepb.Field{},
				Oneofs:  []string{},
				Syntax:  typepb.Syntax_SYNTAX_PROTO3,
				Options: []*typepb.Option{},
				SourceContext: &sourcecontextpb.SourceContext{
					FileName: string(file.Desc.Package()) + "." + string(file.Desc.FullName()),
				},
			}
			if msgGen.GetTypeAlias() {
				fieldName := msgGen.GetTypeAliasField()
				if fieldName == "" {
					fieldName = "value"
				}
				found := false
				for _, f := range message.Fields {
					if string(f.Desc.Name()) != fieldName {
						continue
					}
					g.typeAliases[string(message.Desc.FullName())] = typeAliasInfo{
						fieldName:   fieldName,
						fieldGoName: f.GoName,
						kind:        typepb.Field_Kind(f.Desc.Kind()),
						pbTypeName:  message.GoIdent.GoName,
					}
					found = true
					break
				}
				if !found {
					logger.Error("type_alias field not found",
						zap.String("message", string(message.Desc.FullName())),
						zap.String("field_name", fieldName),
					)
				}
			}
			for cnt, vf := range msgGen.GetVirtualFields() {
				// TODO: VALIDATE
				vfApply := &typepb.Field{
					Kind:         vf.Kind,
					Cardinality:  vf.Cardinality,
					Number:       int32(-(cnt + 1)), // virtual fields are negative for alignment
					Name:         vf.Name,
					JsonName:     stringOrDefault(vf.JsonName, strcase.ToLowerCamel(vf.Name)),
					DefaultValue: vf.DefaultValue,
					TypeUrl:      vf.TypeUrl,
					OneofIndex:   0,
					//Options: []*typepb.Option{
					//	{Name: fmt.Sprintf("generate=%t", msgGen.GetGenerate())},
					//},
					Packed: vf.Packed,
				}
				l.Debug(
					"virtual_field",
					zap.String("full_path", vfApply.Name),
					zap.Int32("number", vfApply.Number),
					zap.String("kind", vfApply.Kind.String()),
					zap.String("cardinality", vfApply.Cardinality.String()),
					zap.String("json_name", vfApply.JsonName),
				)
				resMessage.Fields = append(resMessage.Fields, vfApply)
			}
			l.Debug("message", zap.String("full_path", string(message.Desc.FullName())))
			for _, oneof := range message.Oneofs {
				if oneof.Desc.IsSynthetic() {
					continue
				}
				oneOffOpts := oneof.Desc.Options().(*descriptorpb.OneofOptions)
				oneOffGen := proto.GetExtension(oneOffOpts, goplain.E_Oneof).(*goplain.OneofOptions)
				l.Debug(
					"oneof",
					zap.String("full_path", string(oneof.Desc.FullName())),
					zap.String("go_name", oneof.GoName),
					zap.Bool("generate", oneOffGen.GetEmbed()),
					zap.Bool("generate_prefix", oneOffGen.GetEmbedWithPrefix()),
				)
				oneoffName := string(oneof.Desc.FullName())
				if oneOffGen.GetEmbed() {
					oneoffName = marker.New(oneoffName, map[string]string{embedMarker: trueVal}).String()
				} else if oneOffGen.GetEmbedWithPrefix() {
					oneoffName = marker.New(oneoffName, map[string]string{embedMarker: trueVal, prefixMarker: trueVal}).String()
				}
				resMessage.Oneofs = append(resMessage.Oneofs, oneoffName)
			}
			for _, field := range message.Fields {
				fieldOpt := field.Desc.Options().(*descriptorpb.FieldOptions)
				fieldGen := proto.GetExtension(fieldOpt, goplain.E_Field).(*goplain.FieldOptions)
				var fieldOptions []*typepb.Option
				//for _, opt := range field.Desc.Options() {
				//	fieldOptions = &typepb.Option{
				//		Name:  opt.Get(),
				//		path: opt.GetValue(),
				//	}
				//}
				newField := &typepb.Field{
					Kind:         typepb.Field_Kind(field.Desc.Kind()),
					Cardinality:  typepb.Field_Cardinality(field.Desc.Cardinality()),
					Number:       int32(field.Desc.Number()),
					Name:         string(field.Desc.FullName()),
					Options:      fieldOptions,
					Packed:       field.Desc.IsPacked(),
					JsonName:     field.Desc.JSONName(),
					DefaultValue: field.Desc.Default().String(),
				}
				logOpts := []zap.Field{
					zap.String("full_path", string(field.Desc.FullName())),
					zap.String("go_name", field.GoName),
				}
				if field.Oneof != nil {
					if field.Oneof.Desc.IsSynthetic() {
						logOpts = append(logOpts, zap.String("from_oneoff", string(field.Oneof.Desc.FullName())))
						base := newField.TypeUrl
						if base == "" {
							base = newField.Name
						}
						newField.TypeUrl = marker.Parse(base).AddMarker(isOneoffedMarker, trueVal).String()
					} else {
						logOpts = append(
							logOpts,
							zap.String("from_oneoff", string(field.Oneof.Desc.FullName())),
							zap.Int32("oneof_index", int32(field.Desc.ContainingOneof().Index())),
						)
						for idx, oneof := range message.Oneofs {
							if oneof.Desc.FullName() == field.Oneof.Desc.FullName() {
								newField.OneofIndex = int32(idx + 1)
								break
							}
						}
						if newField.OneofIndex == 0 {
							panic("oneof not found")
						}
					}
				}
				if field.Message != nil {
					logOpts = append(logOpts, zap.String("is_message", string(field.Message.Desc.FullName())))
					fieldName := string(field.Message.Desc.FullName())
					if fieldGen.GetEmbed() {
						fieldName = marker.New(fieldName, map[string]string{embedMarker: trueVal}).String()
					} else if fieldGen.GetEmbedWithPrefix() {
						fieldName = marker.New(fieldName, map[string]string{embedMarker: trueVal, prefixMarker: trueVal}).String()
					}
					newField.TypeUrl = fieldName
				}
				if field.Desc.IsMap() && field.Message != nil {
					entry := field.Message
					if len(entry.Fields) >= 2 {
						key := entry.Fields[0]
						val := entry.Fields[1]
						base := newField.TypeUrl
						if base == "" {
							base = newField.Name
						}
						markers := map[string]string{
							mapMarker:    trueVal,
							mapKeyKind:   fmt.Sprint(int(typepb.Field_Kind(key.Desc.Kind()))),
							mapValueKind: fmt.Sprint(int(typepb.Field_Kind(val.Desc.Kind()))),
						}
						if val.Message != nil {
							markers[mapValueTypeURL] = encodeMarkerValue(string(val.Message.Desc.FullName()))
						}
						newField.TypeUrl = marker.Parse(base).AddMarkers(markers).String()
					}
				}
				override := g.resolveFieldOverride(field, fieldGen.GetOverrideType(), fileOverrides)
				g.applyOverrideMarkers(newField, override)
				l.Debug("field", logOpts...)
				resMessage.Fields = append(resMessage.Fields, newField)
			}
			newResult.Messages[string(message.Desc.FullName())] = resMessage
		}
		result = append(result, newResult)
	}
	return result
}

const (
	isOneoffedMarker = "is_oneoff"
	isMessageMarker  = "is_message"
	// CRF markers
	crfMarker      = "crf"        // marks field as CRF (Collision Resolution Field)
	crfForMarker   = "crf_for"    // name of the field this CRF resolves
	crfPathsMarker = "crf_paths"  // encoded list of collided paths
	empathMarker   = "empath"     // full EmPath for the field origin
	plainName      = "plain_name" // final plain name for the field
	// Map markers
	mapMarker       = "is_map"
	mapKeyKind      = "map_key_kind"
	mapValueKind    = "map_val_kind"
	mapValueTypeURL = "map_val_type_url"
)

func (g *Generator) processEmbedOneof(msg *typepb.Type) {
	//l := logger.Logger.Named("processEmbedOneof")
	newOneOffs := make([]string, 0)
	for oldIdx, oneoff := range msg.Oneofs {
		if !marker.Parse(oneoff).HasMarker(embedMarker) {
			for _, field := range msg.Fields {
				if field.OneofIndex != 0 && field.OneofIndex == int32(oldIdx+1) {
					field.OneofIndex = int32(len(newOneOffs) + 1)
				}
			}
			newOneOffs = append(newOneOffs, oneoff)
		}
		if marker.Parse(oneoff).HasMarker(embedMarker) {
			for _, field := range msg.Fields {
				if field.OneofIndex == int32(oldIdx+1) {
					field.OneofIndex = 0
					oneoffPath := empath.New(marker.Parse(oneoff).AddMarker(isOneoffedMarker, trueVal))
					field.TypeUrl = oneoffPath.Append(marker.Parse(field.TypeUrl)).String()
				}
			}
		}
	}
	msg.Oneofs = newOneOffs
}

// flattenedField represents a field with its EmPath origin and computed plain name
type flattenedField struct {
	field     *typepb.Field
	emPath    empath.EmPath
	plainName string // computed Go field name
}

// getShortName extracts the last segment from a full name like "test.File.path" -> "path"
func getShortName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) == 0 {
		return fullName
	}
	return parts[len(parts)-1]
}

// findMessage looks up a message by its TypeUrl (with markers stripped)
func (g *Generator) findMessage(ir *TypePbIR, typeUrl string) (*typepb.Type, bool) {
	targetValue := empath.Parse(typeUrl).Last().Value()
	for _, m := range ir.Messages {
		if empath.Parse(m.Name).Last().Value() == targetValue {
			return m, true
		}
	}
	return nil, false
}

// flattenEmbeddedFields recursively flattens embedded fields into a list
// currentPath is the EmPath built so far
// prefix is the name prefix to apply (from embed_with_prefix)
func (g *Generator) flattenEmbeddedFields(
	ir *TypePbIR,
	fields []*typepb.Field,
	currentPath empath.EmPath,
	prefix string,
) []flattenedField {
	l := logger.Logger.Named("flattenEmbeddedFields")
	var result []flattenedField

	for _, field := range fields {
		// Skip fields that belong to non-embedded oneof (oneof_index > 0 means it's still in oneof)
		if field.OneofIndex > 0 {
			// Field belongs to a non-embedded oneof, keep as is
			result = append(result, flattenedField{
				field:     copyField(field),
				emPath:    currentPath.Append(marker.Parse(field.Name)),
				plainName: getShortName(field.Name),
			})
			continue
		}

		fieldPath := empath.Parse(field.TypeUrl)
		fieldMarker := fieldPath.Last()
		isEmbed := fieldMarker.HasMarker(embedMarker)

		// Check for prefix marker anywhere in the path (could be on oneof)
		hasPrefix := false
		for _, segment := range fieldPath {
			if segment.HasMarker(prefixMarker) {
				hasPrefix = true
				break
			}
		}

		// Build field's marker for EmPath
		fieldPathMarker := marker.Parse(field.Name)
		if isEmbed {
			fieldPathMarker = fieldPathMarker.AddMarker(embedMarker, trueVal)
		}
		if hasPrefix {
			fieldPathMarker = fieldPathMarker.AddMarker(prefixMarker, trueVal)
		}

		// Extend the path
		newPath := currentPath.Append(fieldPathMarker)

		// Compute new prefix for nested fields
		newPrefix := prefix
		if hasPrefix && newPrefix == "" {
			// Find the segment with prefix marker and use its name as prefix
			for _, segment := range fieldPath {
				if segment.HasMarker(prefixMarker) {
					oneofShortName := getShortName(segment.Value())
					newPrefix = strcase.ToCamel(oneofShortName)
					break
				}
			}
		}

		if isEmbed && field.Kind == typepb.Field_TYPE_MESSAGE {
			// Find the embedded message and flatten its fields
			msgType, ok := g.findMessage(ir, field.TypeUrl)
			if !ok {
				l.Error("embedded message not found",
					zap.String("type_url", field.TypeUrl),
					zap.String("field", field.Name),
				)
				continue
			}

			l.Debug("flattening embedded message",
				zap.String("message", msgType.Name),
				zap.String("field", field.Name),
				zap.String("path", newPath.String()),
			)

			// Recursively flatten the embedded message's fields
			nestedFields := g.flattenEmbeddedFields(ir, msgType.Fields, newPath, newPrefix)
			result = append(result, nestedFields...)
		} else {
			// Regular field or non-message embed - add to result
			fieldCopy := copyField(field)

			// Check if plain_name was already computed (from previous processing)
			// Search in any segment of the path
			existingPlainName := ""
			for _, segment := range fieldPath {
				if pn := segment.GetMarker(plainName); pn != "" {
					existingPlainName = pn
					break
				}
			}

			var computedName string
			if existingPlainName != "" {
				// Use existing plain_name, but add current prefix if any
				if prefix != "" {
					computedName = prefix + strcase.ToCamel(existingPlainName)
				} else {
					computedName = existingPlainName
				}
			} else {
				// Compute plain name from short field name
				// Use newPrefix if it was set from oneof with prefix marker
				fieldShortName := getShortName(field.Name)
				effectivePrefix := prefix
				if newPrefix != "" {
					effectivePrefix = newPrefix
				}
				computedName = fieldShortName
				if effectivePrefix != "" {
					computedName = effectivePrefix + strcase.ToCamel(fieldShortName)
				}
			}

			// Prefer existing empath (if field was already flattened) to preserve full chain
			empathValue := marker.Parse(fieldCopy.TypeUrl).GetMarker(empathMarker)
			pathForMarker := newPath
			if empathValue != "" {
				decoded := decodeEmpath(empathValue)
				if decoded != "" {
					pathForMarker = currentPath.AppendPath(empath.Parse(decoded))
				}
			}

			// Store EmPath in TypeUrl marker (encode separator chars to avoid marker parsing issues)
			encodedEmpath := strings.NewReplacer(
				"/", "|",
				"?", "%3F",
				";", "%3B",
				"=", "%3D",
			).Replace(pathForMarker.String())
			segments := empath.Parse(fieldCopy.TypeUrl)
			if len(segments) == 0 {
				segments = empath.New(marker.Parse(field.Name))
			}
			lastIdx := len(segments) - 1
			segments[lastIdx] = segments[lastIdx].
				AddMarker(empathMarker, encodedEmpath).
				AddMarker(plainName, computedName)
			fieldCopy.TypeUrl = segments.String()

			result = append(result, flattenedField{
				field:     fieldCopy,
				emPath:    pathForMarker,
				plainName: computedName,
			})

			l.Debug("flattened field",
				zap.String("original_name", field.Name),
				zap.String("plain_name", computedName),
				zap.String("empath", pathForMarker.String()),
			)
		}
	}

	return result
}

// processCollisions detects collisions and adds CRF fields if EnableCRF is true
func (g *Generator) processCollisions(flattened []flattenedField) ([]*typepb.Field, error) {
	l := logger.Logger.Named("processCollisions")

	// Group by plain name
	byName := make(map[string][]flattenedField)
	for _, ff := range flattened {
		byName[ff.plainName] = append(byName[ff.plainName], ff)
	}

	var result []*typepb.Field

	for name, fields := range byName {
		if len(fields) == 1 {
			// No collision
			result = append(result, fields[0].field)
			continue
		}

		// Collision detected
		l.Debug("collision detected",
			zap.String("name", name),
			zap.Int("count", len(fields)),
		)

		if !g.Settings.EnableCRF {
			// Collisions not allowed
			var paths []string
			for _, f := range fields {
				paths = append(paths, f.emPath.String())
			}
			return nil, fmt.Errorf(
				"field name collision for '%s' from paths: %v. Enable CRF to resolve collisions",
				name, paths,
			)
		}

		// CRF enabled - merge fields and add CRF field
		// Use first field as the merged field
		mergedField := fields[0].field

		paths := make([]string, 0, len(fields))
		for _, f := range fields {
			var parts []string
			for _, segment := range f.emPath {
				if segment.HasMarker(isOneoffedMarker) {
					continue
				}
				name := getShortName(segment.Value())
				if name == "" {
					continue
				}
				parts = append(parts, name)
			}
			if len(parts) == 0 {
				continue
			}
			paths = append(paths, encodeMarkerValue(strings.Join(parts, "/")))
		}

		// Mark as having collision and add plain_name
		mergedField.TypeUrl = marker.Parse(mergedField.TypeUrl).
			AddMarker(crfMarker, trueVal).
			AddMarker(plainName, name). // add plain_name for later embed
			AddMarker(crfPathsMarker, strings.Join(paths, ",")).
			String()

		result = append(result, mergedField)

		// Create CRF field
		crfField := &typepb.Field{
			Kind:        typepb.Field_TYPE_STRING,
			Cardinality: typepb.Field_CARDINALITY_OPTIONAL,
			Number:      -(mergedField.Number + 1000), // negative to avoid conflicts
			Name:        name + "CRF",
			JsonName:    strcase.ToLowerCamel(name + "CRF"),
			TypeUrl: marker.New("", map[string]string{
				crfMarker:    trueVal,
				crfForMarker: name,
			}).String(),
		}

		result = append(result, crfField)

		l.Debug("added CRF field",
			zap.String("for_field", name),
			zap.String("crf_name", crfField.Name),
		)
	}

	return result, nil
}

func (g *Generator) processEmbeddedMessages(ir *TypePbIR, msg *typepb.Type) error {
	l := logger.Logger.Named("processEmbeddedMessages")

	l.Debug("processing message", zap.String("name", msg.Name))

	// Start with empty path
	initialPath := empath.New()

	// Flatten all embedded fields
	flattened := g.flattenEmbeddedFields(ir, msg.Fields, initialPath, "")

	// Process collisions
	processedFields, err := g.processCollisions(flattened)
	if err != nil {
		return fmt.Errorf("message %s: %w", msg.Name, err)
	}

	// Replace message fields with processed ones
	msg.Fields = processedFields

	return nil
}

// copyField creates a deep copy of a typepb.Field
func copyField(f *typepb.Field) *typepb.Field {
	return &typepb.Field{
		Kind:         f.Kind,
		Cardinality:  f.Cardinality,
		Number:       f.Number,
		Name:         f.Name,
		TypeUrl:      f.TypeUrl,
		OneofIndex:   f.OneofIndex,
		Packed:       f.Packed,
		Options:      f.Options, // shallow copy of options
		JsonName:     f.JsonName,
		DefaultValue: f.DefaultValue,
	}
}

func (g *Generator) ProcessOneoffs(typeIRs []*TypePbIR) ([]*TypePbIR, error) {
	for _, ir := range typeIRs {
		for _, msg := range ir.Messages {
			g.processEmbedOneof(msg)
		}
		processed := make(map[string]bool)
		visiting := make(map[string]bool)
		for _, msg := range ir.Messages {
			if err := g.processEmbeddedMessagesRecursive(ir, msg, visiting, processed); err != nil {
				return nil, err
			}
		}
	}
	return typeIRs, nil
}

func (g *Generator) processEmbeddedMessagesRecursive(
	ir *TypePbIR,
	msg *typepb.Type,
	visiting map[string]bool,
	processed map[string]bool,
) error {
	if processed[msg.Name] {
		return nil
	}
	if visiting[msg.Name] {
		return fmt.Errorf("embedded message cycle detected at %s", msg.Name)
	}

	visiting[msg.Name] = true

	for _, field := range msg.Fields {
		if field.OneofIndex > 0 {
			continue
		}
		if field.Kind != typepb.Field_TYPE_MESSAGE {
			continue
		}

		fieldPath := empath.Parse(field.TypeUrl)
		if len(fieldPath) == 0 {
			continue
		}
		if !fieldPath.Last().HasMarker(embedMarker) {
			continue
		}

		embedded, ok := g.findMessage(ir, field.TypeUrl)
		if !ok {
			continue
		}
		if err := g.processEmbeddedMessagesRecursive(ir, embedded, visiting, processed); err != nil {
			return err
		}
	}

	if err := g.processEmbeddedMessages(ir, msg); err != nil {
		return err
	}

	visiting[msg.Name] = false
	processed[msg.Name] = true
	return nil
}

func (g *Generator) Generate() error {
	collected := g.Collect()
	g.writeFile("collected", collected)
	oneoffs, err := g.ProcessOneoffs(collected)
	if err != nil {
		return err
	}
	g.writeFile("oneoffs", oneoffs)
	if err := g.Render(oneoffs); err != nil {
		return err
	}
	if g.Settings.JSONJX {
		if err := g.RenderJXJSON(oneoffs); err != nil {
			return err
		}
	}
	if err := g.RenderConverters(oneoffs); err != nil {
		return err
	}
	return nil
}

func (g *Generator) writeFile(name string, typeIRs []*TypePbIR) {
	for _, r := range typeIRs {
		jsonMessages, err := json.MarshalIndent(struct {
			Messages map[string]*typepb.Type `json:"messages"`
		}{Messages: r.Messages}, "", "  ")
		if err != nil {
			panic(err)
		}
		os.WriteFile(
			"bin/json/"+name+".json",
			jsonMessages,
			0644,
		)
	}
}
