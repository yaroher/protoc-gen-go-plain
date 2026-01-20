package converter

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-plain/logger"
	"github.com/yaroher/protoc-gen-plain/plain"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/typepb"
	"google.golang.org/protobuf/types/pluginpb"
)

type Option func(*Converter) error

func WithPlainSuffix(suffix string) Option {
	return func(g *Converter) error {
		if suffix == "" {
			return fmt.Errorf("plain suffix must not be empty")
		}
		g.plainSuffix = suffix
		return nil
	}
}

type Converter struct {
	Plugin         *protogen.Plugin
	plainSuffix    string
	messageRenames map[string]string
	metadata       *ConversionMetadata

	// Временное хранилище для построения метаданных
	originalPlugin *protogen.Plugin
}

func Convert(p *protogen.Plugin, opts ...Option) (*protogen.Plugin, *ConversionMetadata, error) {
	c, err := NewConverter(p, opts...)
	if err != nil {
		return nil, nil, err
	}
	plugin, metadata, err := c.Convert()
	if err != nil {
		return nil, nil, err
	}
	return plugin, metadata, nil
}

func NewConverter(p *protogen.Plugin, opts ...Option) (*Converter, error) {
	g := &Converter{
		Plugin:         p,
		plainSuffix:    "Plain",
		messageRenames: make(map[string]string),
		metadata:       NewConversionMetadata(),
		originalPlugin: p,
	}
	for _, opt := range opts {
		if err := opt(g); err != nil {
			return nil, err
		}
	}
	return g, nil
}

//func (g *Converter) BuildIR(f *protogen.File) (*plain.IrBundle, error) {
//return NewFileIR(f, g.Settings)
//return nil, nil
//}

// Convert creates a completely new protogen.Plugin from scratch with transformed messages based on plain options
func (g *Converter) Convert() (*protogen.Plugin, *ConversionMetadata, error) {
	// Create a completely new request from scratch using information from the original
	// Clone the original request to get all fields, then replace ProtoFile with transformed ones
	newRequest := proto.Clone(g.Plugin.Request).(*pluginpb.CodeGeneratorRequest)

	// Create a new file descriptor set with transformations applied
	// Transform ALL proto files, not just the ones to generate
	// This ensures all dependencies are also transformed
	transformedProtoFiles, err := g.transformFileDescriptors(g.Plugin.Request.GetProtoFile())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to transform file descriptors: %w", err)
	}

	// Replace proto files in the new request with transformed ones
	// This ensures that when protogen.Options{}.New creates the Plugin,
	// it will use transformed FileDescriptorProto for all files
	newRequest.ProtoFile = transformedProtoFiles

	// Validate transformed FileDescriptorProto before creating Plugin
	// This helps catch issues with protodesc.NewFile early
	for _, protoFile := range transformedProtoFiles {
		// Try to create FileDescriptor from transformed FileDescriptorProto
		// Use empty resolver for validation (we just want to check if it's valid)
		_, err := protodesc.NewFile(protoFile, nil)
		if err != nil {
			logger.Debug("Failed to create FileDescriptor from transformed FileDescriptorProto",
				zap.String("file", protoFile.GetName()),
				zap.Error(err))
			// Log field details for debugging
			for _, msg := range protoFile.MessageType {
				if msg.GetName() == "TestMessage" {
					logger.Debug("TestMessage fields in transformed FileDescriptorProto",
						zap.Int("fields", len(msg.Field)))
					for _, field := range msg.Field {
						logger.Debug("Field in transformed FileDescriptorProto",
							zap.String("name", field.GetName()),
							zap.String("type", field.GetType().String()),
							zap.String("type_name", func() string {
								if field.TypeName != nil {
									return *field.TypeName
								}
								return "<nil>"
							}()))
					}
				}
			}
			// Continue anyway - protogen.Options{}.New will handle the error
		}
	}

	// Create completely new plugin with transformed request
	// This will create a new fileReg and all structures (Files, Messages, Fields) from scratch
	// using the transformed FileDescriptorProto
	opts := protogen.Options{}
	newPlugin, err := opts.New(newRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new plugin: %w", err)
	}

	// Preserve feature flags that were originally requested (proto3 optional etc).
	supportedFeatures := g.Plugin.SupportedFeatures
	supportedMin := g.Plugin.SupportedEditionsMinimum
	supportedMax := g.Plugin.SupportedEditionsMaximum

	// Replace the original plugin contents with the transformed one so that
	// protoc continues generating from the modified descriptors.
	*g.Plugin = *newPlugin
	g.Plugin.SupportedFeatures = supportedFeatures
	g.Plugin.SupportedEditionsMinimum = supportedMin
	g.Plugin.SupportedEditionsMaximum = supportedMax

	// Построить метаданные, сравнивая originalPlugin и newPlugin
	if err := g.buildMetadata(newPlugin); err != nil {
		return nil, nil, fmt.Errorf("failed to build metadata: %w", err)
	}

	return g.Plugin, g.metadata, nil
}

// transformFileDescriptors applies plain options to transform file descriptors
func (g *Converter) transformFileDescriptors(protoFiles []*descriptorpb.FileDescriptorProto) ([]*descriptorpb.FileDescriptorProto, error) {
	result := make([]*descriptorpb.FileDescriptorProto, 0, len(protoFiles))

	// Track transformed descriptors so dependencies see the latest version.
	transformedByName := make(map[string]*descriptorpb.FileDescriptorProto, len(protoFiles))

	for _, file := range protoFiles {
		// Build resolution set using transformed descriptors when available.
		resolutionFiles := make([]*descriptorpb.FileDescriptorProto, 0, len(protoFiles))
		for _, candidate := range protoFiles {
			if transformed, ok := transformedByName[candidate.GetName()]; ok {
				resolutionFiles = append(resolutionFiles, transformed)
				continue
			}
			resolutionFiles = append(resolutionFiles, candidate)
		}

		transformedFile, err := g.transformFile(file, resolutionFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to transform file %s: %w", file.GetName(), err)
		}
		transformedByName[file.GetName()] = transformedFile
		result = append(result, transformedFile)
	}

	return result, nil
}

// findMessageByTypeName finds a message descriptor by its type name across all proto files
func (g *Converter) findMessageByTypeName(typeName string, protoFiles []*descriptorpb.FileDescriptorProto) *descriptorpb.DescriptorProto {
	// Type name format: .package.MessageName or .package.OuterMessage.InnerMessage or package.MessageName
	// Remove leading dot if present
	cleanTypeName := typeName
	if len(cleanTypeName) > 0 && cleanTypeName[0] == '.' {
		cleanTypeName = cleanTypeName[1:]
	}

	// Split into parts (package and message path)
	parts := make([]string, 0)
	{
		remaining := cleanTypeName
		for remaining != "" {
			dotIdx := -1
			for i := 0; i < len(remaining); i++ {
				if remaining[i] == '.' {
					dotIdx = i
					break
				}
			}
			if dotIdx == -1 {
				parts = append(parts, remaining)
				break
			}
			parts = append(parts, remaining[:dotIdx])
			remaining = remaining[dotIdx+1:]
		}
	}

	if len(parts) == 0 {
		return nil
	}

	// Search through all files
	for _, file := range protoFiles {
		filePackage := file.GetPackage()

		// Try to match package
		if len(parts) > 1 && parts[0] == filePackage {
			// Package matches, search for message path
			messagePath := parts[1:]
			if msg := g.findMessageByPath(file.MessageType, messagePath); msg != nil {
				return msg
			}
		} else if len(parts) == 1 || parts[0] == filePackage {
			// Try without package (maybe it's in the same package)
			messagePath := parts
			if len(parts) > 1 && parts[0] == filePackage {
				messagePath = parts[1:]
			}
			if msg := g.findMessageByPath(file.MessageType, messagePath); msg != nil {
				return msg
			}
		}
	}

	return nil
}

// findMessageByPath finds a message by path (e.g., ["OuterMessage", "InnerMessage"])
func (g *Converter) findMessageByPath(messages []*descriptorpb.DescriptorProto, path []string) *descriptorpb.DescriptorProto {
	if len(path) == 0 {
		return nil
	}

	// Find the first message in the path
	for _, msg := range messages {
		if msg.GetName() == path[0] {
			// If this is the last part of the path, return it
			if len(path) == 1 {
				return msg
			}
			// Otherwise, search in nested messages
			return g.findMessageByPath(msg.NestedType, path[1:])
		}
	}

	return nil
}

// resolveTypeAlias resolves a type alias message to its underlying scalar type
func (g *Converter) resolveTypeAlias(typeName string, protoFiles []*descriptorpb.FileDescriptorProto) (*descriptorpb.FieldDescriptorProto_Type, bool) {
	// Remove leading dot if present
	cleanTypeName := typeName
	if len(cleanTypeName) > 0 && cleanTypeName[0] == '.' {
		cleanTypeName = cleanTypeName[1:]
	}

	msg := g.findMessageByTypeName(cleanTypeName, protoFiles)
	if msg == nil {
		logger.Debug("Could not find message for type alias", zap.String("type", cleanTypeName))
		return nil, false
	}

	// Check if this message is a type alias
	msgOpts := g.getMessageOptions(msg)
	if msgOpts == nil || !msgOpts.GetTypeAlias() {
		return nil, false
	}

	// Type alias messages should have exactly one field named "value"
	if len(msg.Field) != 1 {
		logger.Debug("Type alias message does not have exactly one field", zap.String("type", cleanTypeName), zap.Int("field_count", len(msg.Field)))
		return nil, false
	}

	field := msg.Field[0]
	if field.GetName() != "value" {
		logger.Debug("Type alias message field is not named 'value'", zap.String("type", cleanTypeName), zap.String("field_name", field.GetName()))
		return nil, false
	}

	// Return the type of the value field
	logger.Debug("Resolved type alias",
		zap.String("alias_type", cleanTypeName),
		zap.String("underlying_type", field.GetType().String()))
	return field.Type, true
}

// transformFile transforms a single file descriptor
func (g *Converter) transformFile(file *descriptorpb.FileDescriptorProto, allProtoFiles []*descriptorpb.FileDescriptorProto) (*descriptorpb.FileDescriptorProto, error) {
	// Clone the file descriptor
	clonedFile := proto.Clone(file).(*descriptorpb.FileDescriptorProto)

	// Skip transformation for system/imported proto files
	// We only transform files from the target package
	fileName := clonedFile.GetName()
	if g.isSystemProtoFile(fileName) {
		return clonedFile, nil
	}

	// Extract plain file options
	fileOpts := g.getFileOptions(clonedFile)

	// Add virtual types if specified
	if fileOpts != nil && len(fileOpts.GetVirtualTypes()) > 0 {
		// Virtual types will be added to the file
		// TODO: implement virtual types conversion
	}

	// Transform messages in-place for those marked with generate=true
	// Use provided proto files for type resolution (can be original or transformed)
	if len(clonedFile.MessageType) > 0 {
		err := g.transformMessagesInPlace(clonedFile.MessageType, clonedFile.GetPackage(), "", append(allProtoFiles, clonedFile))
		if err != nil {
			return nil, err
		}
	}

	return clonedFile, nil
}

// isSystemProtoFile checks if a file is a system/imported proto file that should not be transformed
func (g *Converter) isSystemProtoFile(fileName string) bool {
	// Don't transform google protobuf well-known types
	if len(fileName) > 15 && fileName[:15] == "google/protobuf" {
		return true
	}
	// Don't transform plain options file (our extension definitions)
	if len(fileName) > 13 && fileName[:13] == "plain/options" {
		return true
	}
	// Add other system proto paths as needed
	return false
}

// transformMessagesInPlace transforms messages in-place (modifies existing messages)
func (g *Converter) transformMessagesInPlace(messages []*descriptorpb.DescriptorProto, packageName, parentPath string, protoFiles []*descriptorpb.FileDescriptorProto) error {
	for _, msg := range messages {
		// Check if message should be transformed
		msgOpts := g.getMessageOptions(msg)

		// Only transform messages explicitly marked with generate=true
		if msgOpts == nil || !msgOpts.GetGenerate() {
			// Also recursively check nested messages
			if len(msg.NestedType) > 0 {
				if err := g.transformMessagesInPlace(msg.NestedType, packageName, parentPath, protoFiles); err != nil {
					return err
				}
			}
			continue
		}

		// Transform this message in-place
		if err := g.transformMessageInPlace(msg, packageName, parentPath, protoFiles); err != nil {
			return err
		}
	}

	return nil
}

// transformMessageInPlace transforms a message in-place (modifies the existing message)
func (g *Converter) transformMessageInPlace(msg *descriptorpb.DescriptorProto, packageName, parentPath string, protoFiles []*descriptorpb.FileDescriptorProto) error {
	// Skip map Entry messages
	if msg.GetOptions() != nil && msg.GetOptions().GetMapEntry() {
		return nil
	}

	originalName := msg.GetName()
	newName := originalName + g.PlainSuffix()
	if newName != originalName {
		oldFull := g.messageFullName(packageName, parentPath, originalName)
		msg.Name = proto.String(newName)
		newFull := g.messageFullName(packageName, parentPath, newName)
		g.registerMessageRename(oldFull, newFull, protoFiles)
	}

	currentParentPath := joinParentPath(parentPath, msg.GetName())
	logger.Debug("Transforming message", zap.String("name", msg.GetName()), zap.Int("fields_before", len(msg.Field)))

	// Get message options
	msgOpts := g.getMessageOptions(msg)
	if msgOpts != nil {
		logger.Debug("Message options found",
			zap.String("name", msg.GetName()),
			zap.Bool("generate", msgOpts.GetGenerate()),
			zap.Int("virtual_fields_count", len(msgOpts.GetVirtualFields())))
	}

	// Transform fields in-place
	if len(msg.Field) > 0 {
		transformedFields, err := g.transformFields(msg.Field, msg, packageName, protoFiles)
		if err != nil {
			return err
		}
		msg.Field = transformedFields
	}

	// Add virtual fields if specified
	if msgOpts != nil && len(msgOpts.GetVirtualFields()) > 0 {
		logger.Debug("Adding virtual fields",
			zap.String("name", msg.GetName()),
			zap.Int("virtual_fields_count", len(msgOpts.GetVirtualFields())))
		virtualFields := g.convertVirtualFields(msgOpts.GetVirtualFields(), len(msg.Field))
		msg.Field = append(msg.Field, virtualFields...)
		logger.Debug("Virtual fields added",
			zap.String("name", msg.GetName()),
			zap.Int("fields_after", len(msg.Field)))
	}

	// Recursively transform nested messages
	if len(msg.NestedType) > 0 {
		if err := g.transformMessagesInPlace(msg.NestedType, packageName, currentParentPath, protoFiles); err != nil {
			return err
		}
	}

	logger.Debug("Message transformation complete",
		zap.String("name", msg.GetName()),
		zap.Int("final_field_count", len(msg.Field)))

	// Expand each oneof into single-field synthetic oneofs so proto3_optional
	// semantics remain valid even if we later drop the oneof from the generated code.
	if len(msg.OneofDecl) > 0 {
		oneofFields := make(map[int32][]*descriptorpb.FieldDescriptorProto)
		for _, field := range msg.Field {
			if field.OneofIndex != nil {
				idx := field.GetOneofIndex()
				oneofFields[idx] = append(oneofFields[idx], field)
			}
		}

		if len(oneofFields) > 0 {
			newOneofs := make([]*descriptorpb.OneofDescriptorProto, 0, len(msg.Field))
			for idx := 0; idx < len(msg.OneofDecl); idx++ {
				fields := oneofFields[int32(idx)]
				if len(fields) == 0 {
					continue
				}
				for _, field := range fields {
					decl := proto.Clone(msg.OneofDecl[idx]).(*descriptorpb.OneofDescriptorProto)
					decl.Name = proto.String(field.GetName() + "_oneof")
					newOneofs = append(newOneofs, decl)
					field.OneofIndex = proto.Int32(int32(len(newOneofs) - 1))
					field.Proto3Optional = proto.Bool(true)
				}
			}
			msg.OneofDecl = newOneofs
		}
	}

	return nil
}

func (g *Converter) messageFullName(packageName, parentPath, name string) string {
	parts := make([]string, 0, 3)
	if packageName != "" {
		parts = append(parts, packageName)
	}
	if parentPath != "" {
		parts = append(parts, parentPath)
	}
	if name != "" {
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return ""
	}
	return "." + strings.Join(parts, ".")
}

func joinParentPath(parentPath, name string) string {
	if parentPath == "" {
		return name
	}
	if name == "" {
		return parentPath
	}
	return parentPath + "." + name
}

func (g *Converter) registerMessageRename(oldFull, newFull string, protoFiles []*descriptorpb.FileDescriptorProto) {
	if oldFull == "" || oldFull == newFull {
		return
	}
	g.messageRenames[oldFull] = newFull
	for _, file := range protoFiles {
		for _, msg := range file.MessageType {
			g.replaceTypeNamesInMessage(msg, oldFull, newFull)
		}
	}
}

func (g *Converter) replaceTypeNamesInMessage(msg *descriptorpb.DescriptorProto, oldFull, newFull string) {
	for _, field := range msg.Field {
		if field.TypeName != nil && strings.HasPrefix(*field.TypeName, oldFull) {
			suffix := strings.TrimPrefix(*field.TypeName, oldFull)
			field.TypeName = proto.String(newFull + suffix)
		}
	}
	for _, nested := range msg.NestedType {
		g.replaceTypeNamesInMessage(nested, oldFull, newFull)
	}
}

// transformFields transforms field descriptors
func (g *Converter) transformFields(fields []*descriptorpb.FieldDescriptorProto, parentMsg *descriptorpb.DescriptorProto, packageName string, protoFiles []*descriptorpb.FileDescriptorProto) ([]*descriptorpb.FieldDescriptorProto, error) {
	result := make([]*descriptorpb.FieldDescriptorProto, 0)

	for _, field := range fields {
		fieldOpts := g.getFieldOptions(field)

		// Handle embed option
		if fieldOpts != nil && fieldOpts.GetEmbed() {
			// Find the message type to embed
			if field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
				logger.Debug("Embed option on non-message field, skipping", zap.String("field", field.GetName()))
				continue
			}

			typeName := field.TypeName
			if typeName == nil {
				continue
			}

			// Remove leading dot if present
			typeNameStr := *typeName
			if len(typeNameStr) > 0 && typeNameStr[0] == '.' {
				typeNameStr = typeNameStr[1:]
			}

			// Find the message to embed
			embeddedMsg := g.findMessageByTypeName(typeNameStr, protoFiles)
			if embeddedMsg == nil {
				logger.Debug("Could not find message to embed", zap.String("type", typeNameStr))
				continue
			}

			// Get prefix for field names
			prefix := ""
			if fieldOpts.GetEmbedWithPrefix() {
				prefix = field.GetName() + "_"
			}

			// Helper function to get max field number
			getMaxFieldNumber := func() int32 {
				max := int32(0)
				for _, f := range parentMsg.Field {
					if f.GetNumber() > max {
						max = f.GetNumber()
					}
				}
				for _, f := range result {
					if f.GetNumber() > max {
						max = f.GetNumber()
					}
				}
				return max
			}

			// Embed all fields from the message
			for _, embeddedField := range embeddedMsg.Field {
				// Recalculate max field number before each embed to handle nested embeds
				maxFieldNumber := getMaxFieldNumber()
				nextFieldNumber := maxFieldNumber + 1

				clonedEmbeddedField := proto.Clone(embeddedField).(*descriptorpb.FieldDescriptorProto)

				// Update field name with prefix
				newName := prefix + embeddedField.GetName()
				clonedEmbeddedField.Name = proto.String(newName)

				// Assign new field number to avoid conflicts
				newFieldNumber := nextFieldNumber
				clonedEmbeddedField.Number = proto.Int32(newFieldNumber)

				// Recursively transform embedded fields (for nested embeds and type aliases)
				// This may add more fields to result, so we need to recalculate max each time
				transformedEmbeddedFields, err := g.transformFields([]*descriptorpb.FieldDescriptorProto{clonedEmbeddedField}, embeddedMsg, packageName, protoFiles)
				if err != nil {
					return nil, err
				}

				// Update field numbers for recursively transformed fields to avoid conflicts
				currentMax := getMaxFieldNumber()
				for _, tf := range transformedEmbeddedFields {
					if tf.GetNumber() <= currentMax {
						currentMax++
						tf.Number = proto.Int32(currentMax)
					} else {
						currentMax = tf.GetNumber()
					}
				}

				result = append(result, transformedEmbeddedFields...)

				logger.Debug("Embedded field",
					zap.String("original_field", field.GetName()),
					zap.String("embedded_field", embeddedField.GetName()),
					zap.String("new_name", newName),
					zap.Int32("original_number", embeddedField.GetNumber()),
					zap.Int32("new_number", newFieldNumber))
			}

			continue
		}

		// Clone the field
		clonedField := proto.Clone(field).(*descriptorpb.FieldDescriptorProto)

		// Handle type alias - replace alias types with their underlying scalar types
		if clonedField.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && clonedField.TypeName != nil {
			typeNameStr := *clonedField.TypeName
			if len(typeNameStr) > 0 && typeNameStr[0] == '.' {
				typeNameStr = typeNameStr[1:]
			}

			// Try to resolve as type alias
			if aliasType, isAlias := g.resolveTypeAlias(typeNameStr, protoFiles); isAlias {
				logger.Debug("Resolving type alias",
					zap.String("field", field.GetName()),
					zap.String("alias_type", typeNameStr),
					zap.String("resolved_type", aliasType.String()))
				clonedField.Type = aliasType
				clonedField.TypeName = nil
			}
		}

		// Handle serialize option
		if fieldOpts != nil && fieldOpts.GetSerialize() {
			// Convert field type to bytes
			clonedField.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
			clonedField.TypeName = nil
		}

		result = append(result, clonedField)
	}

	return result, nil
}

// convertVirtualFields converts google.protobuf.Field to FieldDescriptorProto
func (g *Converter) convertVirtualFields(virtualFields []*typepb.Field, startNumber int) []*descriptorpb.FieldDescriptorProto {
	logger.Debug("convertVirtualFields called",
		zap.Int("virtualFields_count", len(virtualFields)),
		zap.Int("startNumber", startNumber))

	result := make([]*descriptorpb.FieldDescriptorProto, 0, len(virtualFields))

	for i, vf := range virtualFields {
		logger.Debug("Processing virtual field",
			zap.Int("index", i),
			zap.String("name", vf.GetName()),
			zap.Int32("number", vf.GetNumber()),
			zap.String("kind", vf.GetKind().String()))

		// Use the number from virtual field definition if provided, otherwise auto-generate
		fieldNumber := vf.GetNumber()
		if fieldNumber == 0 {
			fieldNumber = int32(startNumber + i + 1)
		}

		field := &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(vf.GetName()),
			Number: proto.Int32(fieldNumber),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}

		// Convert Kind to Type
		switch vf.GetKind() {
		case typepb.Field_TYPE_DOUBLE:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
		case typepb.Field_TYPE_FLOAT:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()
		case typepb.Field_TYPE_INT64:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
		case typepb.Field_TYPE_UINT64:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum()
		case typepb.Field_TYPE_INT32:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
		case typepb.Field_TYPE_FIXED64:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_FIXED64.Enum()
		case typepb.Field_TYPE_FIXED32:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_FIXED32.Enum()
		case typepb.Field_TYPE_BOOL:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
		case typepb.Field_TYPE_STRING:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
		case typepb.Field_TYPE_GROUP:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_GROUP.Enum()
		case typepb.Field_TYPE_MESSAGE:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			field.TypeName = proto.String(vf.GetTypeUrl())
		case typepb.Field_TYPE_BYTES:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
		case typepb.Field_TYPE_UINT32:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum()
		case typepb.Field_TYPE_ENUM:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
			field.TypeName = proto.String(vf.GetTypeUrl())
		case typepb.Field_TYPE_SFIXED32:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_SFIXED32.Enum()
		case typepb.Field_TYPE_SFIXED64:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_SFIXED64.Enum()
		case typepb.Field_TYPE_SINT32:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_SINT32.Enum()
		case typepb.Field_TYPE_SINT64:
			field.Type = descriptorpb.FieldDescriptorProto_TYPE_SINT64.Enum()
		}

		// Handle cardinality (repeated)
		if vf.GetCardinality() == typepb.Field_CARDINALITY_REPEATED {
			field.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		}

		logger.Debug("Created virtual field",
			zap.String("name", field.GetName()),
			zap.Int32("number", field.GetNumber()),
			zap.String("type", field.GetType().String()))

		result = append(result, field)
	}

	logger.Debug("convertVirtualFields returning",
		zap.Int("result_count", len(result)))

	return result
}

// Helper functions to extract options

func (g *Converter) getFileOptions(file *descriptorpb.FileDescriptorProto) *plain.FileOptions {
	if file.Options == nil {
		return nil
	}

	ext := proto.GetExtension(file.Options, plain.E_File)
	if ext == nil {
		return nil
	}

	fileOpts, ok := ext.(*plain.FileOptions)
	if !ok {
		return nil
	}

	return fileOpts
}

func (g *Converter) getMessageOptions(msg *descriptorpb.DescriptorProto) *plain.MessageOptions {
	if msg.Options == nil {
		return nil
	}

	ext := proto.GetExtension(msg.Options, plain.E_Message)
	if ext == nil {
		return nil
	}

	msgOpts, ok := ext.(*plain.MessageOptions)
	if !ok {
		return nil
	}

	return msgOpts
}

func (g *Converter) getFieldOptions(field *descriptorpb.FieldDescriptorProto) *plain.FieldOptions {
	if field.Options == nil {
		return nil
	}

	ext := proto.GetExtension(field.Options, plain.E_Field)
	if ext == nil {
		return nil
	}

	fieldOpts, ok := ext.(*plain.FieldOptions)
	if !ok {
		return nil
	}

	return fieldOpts
}

func (g *Converter) getOneofOptions(oneof *descriptorpb.OneofDescriptorProto) *plain.OneofOptions {
	if oneof.Options == nil {
		return nil
	}

	ext := proto.GetExtension(oneof.Options, plain.E_Oneof)
	if ext == nil {
		return nil
	}

	oneofOpts, ok := ext.(*plain.OneofOptions)
	if !ok {
		return nil
	}

	return oneofOpts
}

// Вспомогательные функции для извлечения опций из protogen объектов

func (g *Converter) getMessageOptionsFromProto(opts *descriptorpb.MessageOptions) *plain.MessageOptions {
	if opts == nil {
		return nil
	}

	ext := proto.GetExtension(opts, plain.E_Message)
	if ext == nil {
		return nil
	}

	msgOpts, ok := ext.(*plain.MessageOptions)
	if !ok {
		return nil
	}

	return msgOpts
}

func (g *Converter) getFieldOptionsFromProto(opts *descriptorpb.FieldOptions) *plain.FieldOptions {
	if opts == nil {
		return nil
	}

	ext := proto.GetExtension(opts, plain.E_Field)
	if ext == nil {
		return nil
	}

	fieldOpts, ok := ext.(*plain.FieldOptions)
	if !ok {
		return nil
	}

	return fieldOpts
}

func (g *Converter) PlainSuffix() string {
	if g == nil || g.plainSuffix == "" {
		return "Plain"
	}
	return g.plainSuffix
}

func (g *Converter) Generate() error {
	return nil
}

// buildMetadata строит метаданные конверсии, сравнивая оригинальный и трансформированный plugin
func (g *Converter) buildMetadata(newPlugin *protogen.Plugin) error {
	// Создаем мапу оригинальных сообщений для быстрого поиска
	originalMessages := make(map[string]*protogen.Message)
	for _, file := range g.originalPlugin.Files {
		if file.Generate {
			g.collectMessages(file.Messages, originalMessages)
		}
	}

	// Проходим по всем трансформированным сообщениям
	for _, file := range newPlugin.Files {
		if file.Generate {
			g.buildMessageMetadata(file.Messages, originalMessages)
		}
	}

	return nil
}

// collectMessages рекурсивно собирает все сообщения
func (g *Converter) collectMessages(messages []*protogen.Message, result map[string]*protogen.Message) {
	for _, msg := range messages {
		fullName := string(msg.Desc.FullName())
		result[fullName] = msg
		g.collectMessages(msg.Messages, result)
	}
}

// buildMessageMetadata рекурсивно строит метаданные для сообщений
func (g *Converter) buildMessageMetadata(messages []*protogen.Message, originalMessages map[string]*protogen.Message) {
	for _, plainMsg := range messages {
		// Пытаемся найти оригинальное сообщение
		plaintFullName := string(plainMsg.Desc.FullName())
		originalFullName := plaintFullName
		if len(originalFullName) > len(g.plainSuffix) && strings.HasSuffix(originalFullName, g.plainSuffix) {
			originalFullName = originalFullName[:len(originalFullName)-len(g.plainSuffix)]
		}

		originalMsg := originalMessages[originalFullName]

		// Проверяем, было ли сообщение сгенерировано
		msgOpts := g.getMessageOptionsFromProto(plainMsg.Desc.Options().(*descriptorpb.MessageOptions))
		isGenerated := msgOpts != nil && msgOpts.GetGenerate()

		if isGenerated {
			msgMeta := &MessageMetadata{
				OriginalMessage: originalMsg,
				PlainMessage:    plainMsg,
				IsGenerated:     true,
				Fields:          make([]*FieldMetadata, 0),
			}

			// Строим метаданные для полей
			if originalMsg != nil {
				g.buildFieldMetadata(plainMsg, originalMsg, msgMeta)
			}

			// Сохраняем метаданные
			fullName := string(plainMsg.Desc.FullName())
			g.metadata.Messages[fullName] = msgMeta
		}

		// Рекурсивно обрабатываем вложенные сообщения
		g.buildMessageMetadata(plainMsg.Messages, originalMessages)
	}
}

// buildFieldMetadata строит метаданные для полей сообщения
func (g *Converter) buildFieldMetadata(plainMsg, originalMsg *protogen.Message, msgMeta *MessageMetadata) {
	// Создаем мапу оригинальных полей
	originalFields := make(map[string]*protogen.Field)
	for _, field := range originalMsg.Fields {
		originalFields[field.GoName] = field
	}

	// Проходим по всем plain полям
	for _, plainField := range plainMsg.Fields {
		fieldMeta := &FieldMetadata{
			PlainField: plainField,
		}

		// Проверяем виртуальное поле
		originalField := originalFields[plainField.GoName]
		if originalField == nil {
			// Поле отсутствует в оригинале - это виртуальное поле
			fieldMeta.IsVirtual = true
		} else {
			fieldMeta.OriginalField = originalField

			// Проверяем опции оригинального поля
			fieldOpts := g.getFieldOptionsFromProto(originalField.Desc.Options().(*descriptorpb.FieldOptions))
			if fieldOpts != nil {
				fieldMeta.IsSerialized = fieldOpts.GetSerialize()
			}

			// Проверяем type alias
			if originalField.Desc.Kind() == protoreflect.MessageKind && originalField.Message != nil {
				msgOpts := g.getMessageOptionsFromProto(originalField.Message.Desc.Options().(*descriptorpb.MessageOptions))
				if msgOpts != nil && msgOpts.GetTypeAlias() {
					fieldMeta.IsTypeAlias = true
					if len(originalField.Message.Fields) > 0 {
						// Сохраняем тип value поля type alias
						fieldMeta.OriginalType = getProtoFieldType(originalField.Message.Fields[0])
					}
				}
			}

			// Проверяем oneof
			if originalField.Oneof != nil && !originalField.Oneof.Desc.IsSynthetic() {
				fieldMeta.IsOneof = true
				fieldMeta.OneofGroupName = originalField.Oneof.GoName
			}
		}

		msgMeta.Fields = append(msgMeta.Fields, fieldMeta)
	}

	// Проверяем embedded поля
	for _, originalField := range originalMsg.Fields {
		fieldOpts := g.getFieldOptionsFromProto(originalField.Desc.Options().(*descriptorpb.FieldOptions))
		if fieldOpts != nil && fieldOpts.GetEmbed() {
			// Это embedded поле - найдем все развернутые поля в plain
			embedPath := originalField.GoName

			if originalField.Message != nil {
				prefix := ""
				if fieldOpts.GetEmbedWithPrefix() {
					prefix = originalField.GoName + "_"
				}

				// Ищем развернутые поля
				for _, embeddedField := range originalField.Message.Fields {
					plainFieldName := prefix + embeddedField.GoName

					// Найдем соответствующее plain поле
					for _, fieldMeta := range msgMeta.Fields {
						if fieldMeta.PlainField.GoName == plainFieldName {
							// Обновляем метаданные
							fieldMeta.IsEmbedded = true
							fieldMeta.EmbedPath = embedPath
							fieldMeta.EmbedSourceField = originalField
							fieldMeta.OriginalField = embeddedField

							// Проверяем type alias для embedded поля
							if embeddedField.Desc.Kind() == protoreflect.MessageKind && embeddedField.Message != nil {
								msgOpts := g.getMessageOptionsFromProto(embeddedField.Message.Desc.Options().(*descriptorpb.MessageOptions))
								if msgOpts != nil && msgOpts.GetTypeAlias() {
									fieldMeta.IsTypeAlias = true
									if len(embeddedField.Message.Fields) > 0 {
										fieldMeta.OriginalType = getProtoFieldType(embeddedField.Message.Fields[0])
									}
								}
							}
							break
						}
					}
				}
			}
		}
	}
}

// getProtoFieldType возвращает строковое представление типа поля
func getProtoFieldType(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
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
		return field.Desc.Kind().String()
	}
}
