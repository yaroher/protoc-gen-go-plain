package generator

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// PathSegment represents one step in the path from root message to a field
type PathSegment struct {
	FieldNumber int32             // proto field number
	FieldName   string            // proto field name
	GoName      string            // Go field name (for getters)
	Message     *protogen.Message // message containing this field
	Field       *protogen.Field   // the field itself
	IsOneof     bool              // is this field part of oneof
	OneofName   string            // oneof name (if IsOneof)
	OneofGoName string            // Go name of oneof (e.g., "PlatformEvent")
}

// PathInfo contains resolved path information for navigation
type PathInfo struct {
	Segments []PathSegment
	// LeafField is the final field in the path
	LeafField *protogen.Field
}

// resolvePathInfo resolves PathNumbers into full PathInfo with all metadata
// rootMsg is the source protobuf message (e.g., EventData)
// pathNumbers is the array of field numbers [1, 2] meaning field#1 -> field#2
func resolvePathInfo(rootMsg *protogen.Message, pathNumbers []int32) (*PathInfo, error) {
	if len(pathNumbers) == 0 {
		return &PathInfo{}, nil
	}

	info := &PathInfo{
		Segments: make([]PathSegment, 0, len(pathNumbers)),
	}

	currentMsg := rootMsg

	for i, fieldNum := range pathNumbers {
		// Find field by number in current message
		field := findFieldByNumber(currentMsg, fieldNum)
		if field == nil {
			return nil, fmt.Errorf("field number %d not found in message %s", fieldNum, currentMsg.Desc.Name())
		}

		segment := PathSegment{
			FieldNumber: fieldNum,
			FieldName:   string(field.Desc.Name()),
			GoName:      field.GoName,
			Message:     currentMsg,
			Field:       field,
		}

		// Check if field is part of oneof
		if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
			segment.IsOneof = true
			segment.OneofName = string(field.Oneof.Desc.Name())
			segment.OneofGoName = field.Oneof.GoName
		}

		info.Segments = append(info.Segments, segment)

		// Move to next message level (if not last segment)
		if i < len(pathNumbers)-1 {
			if field.Message == nil {
				return nil, fmt.Errorf("field %s is not a message, cannot navigate deeper", field.Desc.Name())
			}
			currentMsg = field.Message
		} else {
			info.LeafField = field
		}
	}

	return info, nil
}

// findFieldByNumber finds a field by its proto number in a message
func findFieldByNumber(msg *protogen.Message, number int32) *protogen.Field {
	for _, field := range msg.Fields {
		if int32(field.Desc.Number()) == number {
			return field
		}
	}
	return nil
}

// BuildGetterChain builds the Go getter chain for reading from protobuf
// Example: "pb.GetHeartbeat().GetAgent()" for path [1, 3]
func (p *PathInfo) BuildGetterChain(rootVar string) string {
	if len(p.Segments) == 0 {
		return rootVar
	}

	var parts []string
	parts = append(parts, rootVar)

	for _, seg := range p.Segments {
		parts = append(parts, fmt.Sprintf("Get%s()", seg.GoName))
	}

	return strings.Join(parts, ".")
}

// BuildNilCheck builds nil check conditions for safe navigation
// Example: "pb.GetHeartbeat() != nil && pb.GetHeartbeat().GetAgent() != nil"
func (p *PathInfo) BuildNilCheck(rootVar string) string {
	if len(p.Segments) == 0 {
		return rootVar + " != nil"
	}

	var checks []string
	var chain []string
	chain = append(chain, rootVar)

	// For each segment except the last, we need a nil check
	for i := 0; i < len(p.Segments); i++ {
		seg := p.Segments[i]
		chain = append(chain, fmt.Sprintf("Get%s()", seg.GoName))

		// Only check nil for message types (not scalars on last segment)
		if i < len(p.Segments)-1 || (seg.Field != nil && seg.Field.Message != nil) {
			checks = append(checks, strings.Join(chain, ".")+" != nil")
		}
	}

	if len(checks) == 0 {
		return rootVar + " != nil"
	}

	return strings.Join(checks, " && ")
}

// BuildSetterCode builds Go code to set a value through the path
// This handles oneof wrapper creation
// Returns: (initCode, assignmentPath)
// initCode - code to initialize intermediate structures
// assignmentPath - the path to assign the value to
func (p *PathInfo) BuildSetterCode(gf *protogen.GeneratedFile, rootVar string, valueExpr string, valueIsPointer bool) (initCode string, assignCode string) {
	if len(p.Segments) == 0 {
		return "", ""
	}

	var initLines []string
	currentPath := rootVar

	// Process all segments except the last (which is the actual assignment)
	for i := 0; i < len(p.Segments)-1; i++ {
		seg := p.Segments[i]

		if seg.IsOneof {
			// For oneof, we need to create wrapper type and assign
			// Example: pb.PlatformEvent = &EventData_Heartbeat{Heartbeat: &Heartbeat{}}
			// Use QualifiedGoIdent for wrapper type (it's in same package as parent message)
			wrapperIdent := protogen.GoIdent{
				GoName:       seg.Message.GoIdent.GoName + "_" + seg.GoName,
				GoImportPath: seg.Message.GoIdent.GoImportPath,
			}
			wrapperType := gf.QualifiedGoIdent(wrapperIdent)
			fieldType := gf.QualifiedGoIdent(seg.Field.Message.GoIdent)

			// Check if already set with correct type
			initLines = append(initLines,
				fmt.Sprintf("\tif _, ok := %s.%s.(*%s); !ok || %s.%s == nil {",
					currentPath, seg.OneofGoName, wrapperType, currentPath, seg.OneofGoName),
				fmt.Sprintf("\t\t%s.%s = &%s{%s: &%s{}}",
					currentPath, seg.OneofGoName, wrapperType, seg.GoName, fieldType),
				"\t}",
			)

			// Navigate into the oneof
			currentPath = fmt.Sprintf("%s.%s.(*%s).%s", currentPath, seg.OneofGoName, wrapperType, seg.GoName)
		} else {
			// Regular message field
			fieldType := gf.QualifiedGoIdent(seg.Field.Message.GoIdent)

			initLines = append(initLines,
				fmt.Sprintf("\tif %s.%s == nil {", currentPath, seg.GoName),
				fmt.Sprintf("\t\t%s.%s = &%s{}", currentPath, seg.GoName, fieldType),
				"\t}",
			)

			currentPath = fmt.Sprintf("%s.%s", currentPath, seg.GoName)
		}
	}

	// Last segment - the actual field assignment
	lastSeg := p.Segments[len(p.Segments)-1]

	if lastSeg.IsOneof {
		// Assigning to oneof field - use QualifiedGoIdent for wrapper type
		wrapperIdent := protogen.GoIdent{
			GoName:       lastSeg.Message.GoIdent.GoName + "_" + lastSeg.GoName,
			GoImportPath: lastSeg.Message.GoIdent.GoImportPath,
		}
		wrapperType := gf.QualifiedGoIdent(wrapperIdent)

		// Check if the field is a message type (needs pointer) or scalar (no pointer)
		isMessageField := lastSeg.Field != nil && lastSeg.Field.Message != nil

		if valueIsPointer || !isMessageField {
			// Value is already a pointer, or field is scalar (no pointer needed)
			assignCode = fmt.Sprintf("%s.%s = &%s{%s: %s}",
				currentPath, lastSeg.OneofGoName, wrapperType, lastSeg.GoName, valueExpr)
		} else {
			// Value is not pointer but field expects pointer (message type)
			assignCode = fmt.Sprintf("%s.%s = &%s{%s: &%s}",
				currentPath, lastSeg.OneofGoName, wrapperType, lastSeg.GoName, valueExpr)
		}
	} else {
		// Regular field assignment
		assignCode = fmt.Sprintf("%s.%s = %s", currentPath, lastSeg.GoName, valueExpr)
	}

	return strings.Join(initLines, "\n"), assignCode
}

// GetLeafGoType returns the Go type of the leaf field
func (p *PathInfo) GetLeafGoType(gf *protogen.GeneratedFile) string {
	if p.LeafField == nil {
		return ""
	}

	if p.LeafField.Message != nil {
		return "*" + gf.QualifiedGoIdent(p.LeafField.Message.GoIdent)
	}
	if p.LeafField.Enum != nil {
		return gf.QualifiedGoIdent(p.LeafField.Enum.GoIdent)
	}

	// Scalar type
	return goTypeFromKind(p.LeafField.Desc.Kind())
}

// goTypeFromKind returns Go type string for proto kind
func goTypeFromKind(kind protoreflect.Kind) string {
	switch kind {
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
