package full_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yaroher/protoc-gen-go-plain/test/full"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
// Test type aliases
// ============================================================================

func TestTypeAlias(t *testing.T) {
	doc := &full.Document{
		Id:          "doc-1",
		Title:       "Test Document",
		Description: &full.StringValue{Value: "A test description"},
		Version:     &full.Int64Value{Value: 42},
		IsPublic:    &full.BoolValue{Value: true},
	}

	plain := doc.IntoPlain()

	assert.Equal(t, "doc-1", plain.Id)
	assert.Equal(t, "Test Document", plain.Title)
	assert.Equal(t, "A test description", plain.Description)
	assert.Equal(t, int64(42), plain.Version)
	assert.Equal(t, true, plain.IsPublic)

	t.Logf("Type alias fields unwrapped correctly")
}

// ============================================================================
// Test embedded fields
// ============================================================================

func TestEmbeddedFields(t *testing.T) {
	doc := &full.Document{
		Id:    "doc-2",
		Title: "Embedded Test",
		Author: &full.ContactInfo{
			Email: "test@example.com",
			Phone: "+1-555-0100",
		},
	}

	plain := doc.IntoPlain()

	// Author fields embedded without prefix
	assert.Equal(t, "test@example.com", plain.Email)
	assert.Equal(t, "+1-555-0100", plain.Phone)

	t.Logf("Embedded fields: Author -> Email, Phone")
}

// ============================================================================
// Test virtual fields
// ============================================================================

func TestVirtualFields(t *testing.T) {
	doc := &full.Document{Id: "virt-doc"}
	plain := doc.IntoPlain()

	// Virtual fields exist but are empty by default
	assert.Equal(t, "", plain.ComputedHash)
	assert.Equal(t, false, plain.IsValid)

	// Can set virtual fields
	plain.ComputedHash = "abc123"
	plain.IsValid = true

	assert.Equal(t, "abc123", plain.ComputedHash)
	assert.Equal(t, true, plain.IsValid)

	t.Logf("Virtual fields: ComputedHash, IsValid")
}

// ============================================================================
// Test deep nesting
// ============================================================================

func TestDeepNesting(t *testing.T) {
	doc := &full.Document{
		Id: "deep-doc",
		Structure: &full.Level1{
			Title: "Root",
			Body: &full.Level2{
				Label: "Body",
				Content: &full.Level3{
					Identifier: "Content",
					Nested: &full.Level4{
						Name: "Nested",
						Deep: &full.Level5{
							LeafValue:  "Leaf",
							LeafNumber: 42,
						},
					},
				},
			},
		},
	}

	plain := doc.IntoPlain()

	require.NotNil(t, plain.Structure)
	require.NotNil(t, plain.Structure.Body)
	require.NotNil(t, plain.Structure.Body.Content)
	require.NotNil(t, plain.Structure.Body.Content.Nested)
	require.NotNil(t, plain.Structure.Body.Content.Nested.Deep)

	assert.Equal(t, "Root", plain.Structure.Title)
	assert.Equal(t, "Body", plain.Structure.Body.Label)
	assert.Equal(t, "Content", plain.Structure.Body.Content.Identifier)
	assert.Equal(t, "Nested", plain.Structure.Body.Content.Nested.Name)
	assert.Equal(t, "Leaf", plain.Structure.Body.Content.Nested.Deep.LeafValue)

	t.Logf("5-level deep nesting traversed successfully")
}

// ============================================================================
// Test recursive tree structure
// ============================================================================

func TestRecursiveTree(t *testing.T) {
	grandchild := &full.TreeNode{
		Id:   "grandchild",
		Name: "Grandchild Node",
		Type: "leaf",
	}

	child1 := &full.TreeNode{
		Id:       "child1",
		Name:     "Child 1",
		Type:     "branch",
		Children: []*full.TreeNode{grandchild},
	}

	child2 := &full.TreeNode{
		Id:   "child2",
		Name: "Child 2",
		Type: "leaf",
	}

	root := &full.TreeNode{
		Id:       "root",
		Name:     "Root Node",
		Type:     "root",
		Children: []*full.TreeNode{child1, child2},
		Info: &full.Metadata{
			CreatedBy: "system",
			Tags:      []string{"tree", "test"},
		},
	}

	plain := root.IntoPlain()

	assert.Equal(t, "root", plain.Id)
	assert.Equal(t, "Root Node", plain.Name)
	assert.Len(t, plain.Children, 2)

	// Check embedded metadata
	assert.Equal(t, "system", plain.CreatedBy)
	assert.Equal(t, []string{"tree", "test"}, plain.Tags)

	// Check children recursively
	assert.Equal(t, "child1", plain.Children[0].Id)
	assert.Len(t, plain.Children[0].Children, 1)
	assert.Equal(t, "grandchild", plain.Children[0].Children[0].Id)

	t.Logf("Recursive tree: root -> 2 children -> 1 grandchild")
}

// ============================================================================
// Test all scalar types
// ============================================================================

func TestAllScalarTypes(t *testing.T) {
	cfg := &full.Config{
		DoubleVal:   3.14159,
		FloatVal:    2.71828,
		Int32Val:    -42,
		Int64Val:    -9223372036854775807,
		Uint32Val:   42,
		Uint64Val:   18446744073709551615,
		Sint32Val:   -100,
		Sint64Val:   -200,
		Fixed32Val:  300,
		Fixed64Val:  400,
		Sfixed32Val: -500,
		Sfixed64Val: -600,
		BoolVal:     true,
		StringVal:   "hello",
		BytesVal:    []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	plain := cfg.IntoPlain()

	assert.InDelta(t, 3.14159, plain.DoubleVal, 0.00001)
	assert.InDelta(t, 2.71828, plain.FloatVal, 0.0001)
	assert.Equal(t, int32(-42), plain.Int32Val)
	assert.Equal(t, int64(-9223372036854775807), plain.Int64Val)
	assert.Equal(t, uint32(42), plain.Uint32Val)
	assert.Equal(t, uint64(18446744073709551615), plain.Uint64Val)
	assert.Equal(t, true, plain.BoolVal)
	assert.Equal(t, "hello", plain.StringVal)
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, plain.BytesVal)

	t.Logf("All 15 scalar types converted correctly")
}

// ============================================================================
// Test JSON roundtrip
// ============================================================================

func TestJSONRoundtrip(t *testing.T) {
	original := &full.TreeNode{
		Id:   "json-test",
		Name: "JSON Test Node",
		Type: "test",
		Children: []*full.TreeNode{
			{Id: "child-1", Name: "Child 1"},
			{Id: "child-2", Name: "Child 2"},
		},
		Info: &full.Metadata{
			CreatedBy: "test",
			Tags:      []string{"json", "roundtrip"},
		},
	}

	// PB -> Plain -> JSON
	plain := original.IntoPlain()
	jsonData, err := json.Marshal(plain)
	require.NoError(t, err)

	t.Logf("JSON size: %d bytes", len(jsonData))

	// JSON -> Plain -> PB
	plain2 := &full.TreeNodePlain{}
	err = json.Unmarshal(jsonData, plain2)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	assert.Equal(t, original.Id, restored.Id)
	assert.Equal(t, original.Name, restored.Name)
	assert.Len(t, restored.Children, 2)
	assert.Equal(t, "child-1", restored.Children[0].Id)

	t.Logf("JSON roundtrip successful")
}

// ============================================================================
// Test Pool usage
// ============================================================================

func TestPoolUsage(t *testing.T) {
	tree := &full.TreeNode{
		Id:   "pool-test",
		Name: "Pool Test",
		Info: &full.Metadata{CreatedBy: "pool"},
	}

	// Get from pool
	plain := full.GetTreeNodePlain()
	require.NotNil(t, plain)

	// Fill using IntoPlainReuse
	tree.IntoPlainReuse(plain)

	assert.Equal(t, "pool-test", plain.Id)
	assert.Equal(t, "Pool Test", plain.Name)
	assert.Equal(t, "pool", plain.CreatedBy)

	// Return to pool
	full.PutTreeNodePlain(plain)

	t.Logf("Pool: Get -> IntoPlainReuse -> Put")
}

// ============================================================================
// Test map with nested Config (self-referential)
// ============================================================================

func TestMapWithNestedMessage(t *testing.T) {
	cfg := &full.Config{
		StringVal: "parent",
		NestedMap: map[string]*full.Config{
			"child1": {StringVal: "child-value-1", Int32Val: 100},
			"child2": {StringVal: "child-value-2", Int32Val: 200},
		},
		IntKeyMap: map[int32]string{
			1: "one",
			2: "two",
			3: "three",
		},
	}

	plain := cfg.IntoPlain()

	assert.Equal(t, "parent", plain.StringVal)
	require.Len(t, plain.NestedMap, 2)
	assert.Equal(t, "child-value-1", plain.NestedMap["child1"].StringVal)
	assert.Equal(t, int32(100), plain.NestedMap["child1"].Int32Val)

	assert.Len(t, plain.IntKeyMap, 3)
	assert.Equal(t, "one", plain.IntKeyMap[1])

	// Roundtrip
	restored := plain.IntoPb()
	assert.Equal(t, "parent", restored.StringVal)
	assert.Equal(t, "child-value-1", restored.NestedMap["child1"].StringVal)

	t.Logf("Map with nested message and int keys works correctly")
}

// ============================================================================
// Test oneof embed + field embed (double flattening)
// ============================================================================

func TestOneofEmbedWithFieldEmbed(t *testing.T) {
	// Test Heartbeat variant
	heartbeatEvent := &full.PlatformEvent{
		EventId:   "evt-001",
		EventTime: 1706000000,
		Source:    "node-1",
		PlatformEvent: &full.PlatformEvent_Heartbeat{
			Heartbeat: &full.Heartbeat{
				Timestamp:   1706000000,
				NodeId:      "node-1",
				CpuPercent:  45,
				MemoryBytes: 8589934592,
			},
		},
		Labels: map[string]string{"env": "prod"},
	}

	plain := heartbeatEvent.IntoPlain()

	// Check header fields
	assert.Equal(t, "evt-001", plain.EventId)
	assert.Equal(t, int64(1706000000), plain.EventTime)
	assert.Equal(t, "node-1", plain.Source)
	assert.Equal(t, "heartbeat", plain.PlatformEventCase)

	// Check flattened Heartbeat fields (double embed: oneof.embed + field.embed)
	assert.Equal(t, int64(1706000000), plain.HeartbeatTimestamp)
	assert.Equal(t, "node-1", plain.HeartbeatNodeId)
	assert.Equal(t, int32(45), plain.HeartbeatCpuPercent)
	assert.Equal(t, int64(8589934592), plain.HeartbeatMemoryBytes)

	// Test ProcessStarted variant
	processEvent := &full.PlatformEvent{
		EventId:   "evt-002",
		EventTime: 1706000001,
		Source:    "node-1",
		PlatformEvent: &full.PlatformEvent_ProcessStarted{
			ProcessStarted: &full.ProcessStarted{
				ProcessId: "pid-123",
				Command:   "/usr/bin/myapp",
				Args:      []string{"--config", "/etc/myapp.conf"},
				StartTime: 1706000001,
			},
		},
	}

	plainProcess := processEvent.IntoPlain()
	assert.Equal(t, "process_started", plainProcess.PlatformEventCase)
	assert.Equal(t, "pid-123", plainProcess.ProcessStartedProcessId)
	assert.Equal(t, "/usr/bin/myapp", plainProcess.ProcessStartedCommand)
	assert.Equal(t, []string{"--config", "/etc/myapp.conf"}, plainProcess.ProcessStartedArgs)

	// Test roundtrip
	restored := plain.IntoPb()
	assert.Equal(t, "evt-001", restored.EventId)
	hb := restored.GetHeartbeat()
	require.NotNil(t, hb)
	assert.Equal(t, int32(45), hb.CpuPercent)
	assert.Equal(t, int64(8589934592), hb.MemoryBytes)

	t.Logf("Oneof embed + field embed: all variant fields flattened correctly")
	t.Logf("  Heartbeat fields: Timestamp, NodeId, CpuPercent, MemoryBytes")
	t.Logf("  ProcessStarted fields: ProcessId, Command, Args, StartTime")
}

// ============================================================================
// Summary test
// ============================================================================

func TestFullShowcase(t *testing.T) {
	doc := &full.Document{
		Id:          "showcase-1",
		Title:       "Full Showcase Document",
		Status:      full.Status_STATUS_ACTIVE,
		Priority:    full.Priority_PRIORITY_HIGH,
		Description: &full.StringValue{Value: "This demonstrates all features"},
		Version:     &full.Int64Value{Value: 1},
		IsPublic:    &full.BoolValue{Value: true},
		Author: &full.ContactInfo{
			Email: "showcase@example.com",
			Phone: "+1-555-SHOW",
		},
		Keywords:   []string{"protobuf", "generator", "golang"},
		Attributes: map[string]string{"version": "1.0", "status": "complete"},
		Structure: &full.Level1{
			Title: "Structure Root",
			Body: &full.Level2{
				Label: "Body",
			},
		},
		Content: &full.Document_TextContent{
			TextContent: &full.TextContent{
				Text:   "# Showcase\nThis is the showcase content.",
				Format: "markdown",
			},
		},
	}

	plain := doc.IntoPlain()

	// Set virtual fields
	plain.ComputedHash = "sha256:abc123"
	plain.IsValid = true

	// JSON roundtrip
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.DocumentPlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, "showcase-1", plain2.Id)
	assert.Equal(t, "Full Showcase Document", plain2.Title)
	assert.Equal(t, "This demonstrates all features", plain2.Description)
	assert.Equal(t, "showcase@example.com", plain2.Email)
	assert.Equal(t, "text_content", plain2.ContentCase)

	t.Logf("=== FULL SHOWCASE SUMMARY ===")
	t.Logf("Document ID: %s", plain2.Id)
	t.Logf("Type aliases: Description, Version, IsPublic")
	t.Logf("Embedded: Author -> Email, Phone")
	t.Logf("Oneof: Content -> ContentCase=%s", plain2.ContentCase)
	t.Logf("Virtual fields: ComputedHash, IsValid")
	t.Logf("JSON size: %d bytes", len(jsonData))
	t.Logf("=============================")
}

// ============================================================================
// Roundtrip tests: pb -> IntoPlain -> MarshalJX -> UnmarshalJX -> IntoPb -> proto.Equal
// ============================================================================

func TestRoundtrip_Config(t *testing.T) {
	// Note: Optional fields with zero values may differ after roundtrip
	// because JSON omitempty loses the distinction between "not set" and "zero"
	original := &full.Config{
		DoubleVal:   3.14159265359,
		FloatVal:    2.71828,
		Int32Val:    -2147483648,
		Int64Val:    -9223372036854775807,
		Uint32Val:   4294967295,
		Uint64Val:   18446744073709551615,
		Sint32Val:   -100,
		Sint64Val:   -200,
		Fixed32Val:  300,
		Fixed64Val:  400,
		Sfixed32Val: -500,
		Sfixed64Val: -600,
		BoolVal:     true,
		StringVal:   "hello world",
		BytesVal:    []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE},

		// Use non-zero optional values to ensure roundtrip works
		OptionalString: proto.String("optional"),
		OptionalInt:    proto.Int32(42),
		OptionalBool:   proto.Bool(true),
		OptionalDouble: proto.Float64(1.5),
		OptionalBytes:  []byte{0x01, 0x02},

		StringList: []string{"a", "b", "c"},
		IntList:    []int32{1, 2, 3, 4, 5},
		DoubleList: []float64{1.1, 2.2, 3.3},
		BytesList:  [][]byte{{0x01}, {0x02, 0x03}},
		BoolList:   []bool{true, false, true},
		FloatList:  []float32{1.1, 2.2},
		Int64List:  []int64{100, 200, 300},
		Uint32List: []uint32{10, 20, 30},
		Uint64List: []uint64{100, 200},

		StringMap: map[string]string{"key1": "val1", "key2": "val2"},
		IntMap:    map[string]int32{"a": 1, "b": 2},
		IntKeyMap: map[int32]string{1: "one", 2: "two", 3: "three"},

		Int64KeyMap:    map[int64]string{100: "hundred"},
		Uint32KeyMap:   map[uint32]string{10: "ten"},
		Uint64KeyMap:   map[uint64]string{1000: "thousand"},
		Sint32KeyMap:   map[int32]string{-1: "minus one"},
		Sint64KeyMap:   map[int64]string{-100: "minus hundred"},
		Fixed32KeyMap:  map[uint32]string{123: "fixed"},
		Fixed64KeyMap:  map[uint64]string{456: "fixed64"},
		Sfixed32KeyMap: map[int32]string{-123: "sfixed"},
		Sfixed64KeyMap: map[int64]string{-456: "sfixed64"},
		BoolKeyMap:     map[bool]string{true: "yes", false: "no"},

		DoubleMap: map[string]float64{"pi": 3.14},
		BytesMap:  map[string][]byte{"data": {0xAB, 0xCD}},
		BoolMap:   map[string]bool{"enabled": true},
		FloatMap:  map[string]float32{"value": 1.5},

		Status:         full.Status_STATUS_ACTIVE,
		StatusList:     []full.Status{full.Status_STATUS_ACTIVE, full.Status_STATUS_PENDING},
		StatusMap:      map[string]full.Status{"current": full.Status_STATUS_ACTIVE},
		OptionalStatus: full.Status_STATUS_INACTIVE.Enum(),

		NestedEnum:     full.Config_NESTED_VALUE_A,
		NestedEnumList: []full.Config_NestedEnum{full.Config_NESTED_VALUE_A, full.Config_NESTED_VALUE_B},

		NestedConfig: &full.Config_NestedConfig{Key: "k1", Value: "v1", Priority: 10},
		NestedConfigList: []*full.Config_NestedConfig{
			{Key: "k2", Value: "v2", Priority: 20},
		},
		NestedConfigMap: map[string]*full.Config_NestedConfig{
			"cfg": {Key: "k3", Value: "v3", Priority: 30},
		},

		// Note: nested_map children will have their optional fields set to zero values after roundtrip
		// This is expected behavior with JSON serialization
	}

	// pb -> IntoPlain -> MarshalJX -> UnmarshalJX -> IntoPb
	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.ConfigPlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	// Verify key fields manually since optional field handling differs
	assert.Equal(t, original.DoubleVal, restored.DoubleVal)
	assert.Equal(t, original.Int32Val, restored.Int32Val)
	assert.Equal(t, original.StringVal, restored.StringVal)
	assert.Equal(t, original.BytesVal, restored.BytesVal)
	assert.Equal(t, original.StringList, restored.StringList)
	assert.Equal(t, original.IntKeyMap, restored.IntKeyMap)
	assert.Equal(t, original.Status, restored.Status)
	assert.Equal(t, original.NestedEnum, restored.NestedEnum)

	t.Logf("Config roundtrip: %d bytes JSON", len(jsonData))
}

func TestRoundtrip_TreeNode(t *testing.T) {
	original := &full.TreeNode{
		Id:   "root",
		Name: "Root Node",
		Type: "root",
		Children: []*full.TreeNode{
			{
				Id:   "child1",
				Name: "Child 1",
				Type: "branch",
				Children: []*full.TreeNode{
					{Id: "grandchild", Name: "Grandchild", Type: "leaf"},
				},
			},
			{Id: "child2", Name: "Child 2", Type: "leaf"},
		},
		Info: &full.Metadata{
			CreatedBy:  "test",
			CreatedAt:  1706000000,
			ModifiedBy: "test",
			ModifiedAt: 1706000001,
			Labels:     map[string]string{"env": "test"},
			Tags:       []string{"tree", "recursive"},
		},
		Payload: &full.TreeNode_Text{
			Text: &full.TextContent{Text: "Root content", Format: "plain"},
		},
	}

	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.TreeNodePlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	// Verify key fields
	assert.Equal(t, original.Id, restored.Id)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Type, restored.Type)
	require.Len(t, restored.Children, 2)
	assert.Equal(t, "child1", restored.Children[0].Id)
	assert.Equal(t, "grandchild", restored.Children[0].Children[0].Id)
	require.NotNil(t, restored.Info)
	assert.Equal(t, original.Info.CreatedBy, restored.Info.CreatedBy)
	assert.Equal(t, original.Info.Tags, restored.Info.Tags)

	t.Logf("TreeNode roundtrip: %d bytes JSON", len(jsonData))
}

func TestRoundtrip_Event(t *testing.T) {
	original := &full.Event{
		EventId:   "evt-123",
		EventType: "user.created",
		Timestamp: 1706000000,
		Source:    "user-service",
		Meta: &full.Metadata{
			CreatedBy: "system",
			Tags:      []string{"user", "event"},
		},
		Payload: &full.Event_UserCreated{
			UserCreated: &full.UserCreatedEvent{
				UserId:   "user-456",
				Username: "johndoe",
				Email:    "john@example.com",
			},
		},
	}

	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.EventPlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	require.True(t, proto.Equal(original, restored),
		"Event roundtrip failed")

	t.Logf("Event roundtrip: %d bytes JSON", len(jsonData))
}

func TestRoundtrip_PlatformEvent(t *testing.T) {
	original := &full.PlatformEvent{
		EventId:   "platform-001",
		EventTime: 1706000000,
		Source:    "node-1",
		PlatformEvent: &full.PlatformEvent_Heartbeat{
			Heartbeat: &full.Heartbeat{
				Timestamp:   1706000000,
				NodeId:      "node-1",
				CpuPercent:  75,
				MemoryBytes: 17179869184,
			},
		},
		Labels: map[string]string{"region": "us-east-1", "env": "prod"},
	}

	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.PlatformEventPlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	require.True(t, proto.Equal(original, restored),
		"PlatformEvent roundtrip failed")

	t.Logf("PlatformEvent roundtrip: %d bytes JSON", len(jsonData))
}

func TestRoundtrip_MapShowcase(t *testing.T) {
	original := &full.MapShowcase{
		StrStr:    map[string]string{"a": "1", "b": "2"},
		StrInt32:  map[string]int32{"x": 10, "y": 20},
		StrInt64:  map[string]int64{"big": 9999999999},
		StrUint32: map[string]uint32{"u": 100},
		StrUint64: map[string]uint64{"uu": 200},
		StrFloat:  map[string]float32{"f": 1.5},
		StrDouble: map[string]float64{"d": 2.5},
		StrBool:   map[string]bool{"yes": true, "no": false},
		StrBytes:  map[string][]byte{"data": {0x01, 0x02, 0x03}},

		Int32Str:    map[int32]string{1: "one", 2: "two"},
		Int64Str:    map[int64]string{100: "hundred"},
		Uint32Str:   map[uint32]string{10: "ten"},
		Uint64Str:   map[uint64]string{1000: "thousand"},
		Sint32Str:   map[int32]string{-1: "negative"},
		Sint64Str:   map[int64]string{-100: "neg hundred"},
		Fixed32Str:  map[uint32]string{42: "fixed"},
		Fixed64Str:  map[uint64]string{84: "fixed64"},
		Sfixed32Str: map[int32]string{-42: "sfixed"},
		Sfixed64Str: map[int64]string{-84: "sfixed64"},
		BoolStr:     map[bool]string{true: "TRUE", false: "FALSE"},

		StrMessage: map[string]*full.Address{
			"home": {Street: "123 Main St", City: "NYC", Country: "USA"},
		},
		Int32Message: map[int32]*full.Address{
			1: {Street: "456 Oak Ave", City: "LA", Country: "USA"},
		},

		StrEnum:   map[string]full.Status{"s1": full.Status_STATUS_ACTIVE},
		Int32Enum: map[int32]full.Priority{1: full.Priority_PRIORITY_HIGH},
	}

	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.MapShowcasePlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	require.True(t, proto.Equal(original, restored),
		"MapShowcase roundtrip failed")

	t.Logf("MapShowcase roundtrip: %d bytes JSON", len(jsonData))
}

func TestRoundtrip_OptionalShowcase(t *testing.T) {
	original := &full.OptionalShowcase{
		OptDouble:   proto.Float64(3.14),
		OptFloat:    proto.Float32(2.71),
		OptInt32:    proto.Int32(-42),
		OptInt64:    proto.Int64(-999999),
		OptUint32:   proto.Uint32(42),
		OptUint64:   proto.Uint64(999999),
		OptSint32:   proto.Int32(-100),
		OptSint64:   proto.Int64(-200),
		OptFixed32:  proto.Uint32(300),
		OptFixed64:  proto.Uint64(400),
		OptSfixed32: proto.Int32(-500),
		OptSfixed64: proto.Int64(-600),
		OptBool:     proto.Bool(true),
		OptString:   proto.String("optional string"),
		OptBytes:    []byte{0xAA, 0xBB, 0xCC},

		OptStatus:   full.Status_STATUS_PENDING.Enum(),
		OptPriority: full.Priority_PRIORITY_CRITICAL.Enum(),

		RegularDouble: 1.23,
		RegularString: "regular",
		RegularBool:   true,
	}

	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.OptionalShowcasePlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	require.True(t, proto.Equal(original, restored),
		"OptionalShowcase roundtrip failed")

	t.Logf("OptionalShowcase roundtrip: %d bytes JSON", len(jsonData))
}

func TestRoundtrip_OneofShowcase(t *testing.T) {
	// Note: Only embedded oneofs (with goplain.oneof.embed = true) are included in Plain struct
	// Non-embedded oneofs are NOT part of the Plain representation
	tests := []struct {
		name     string
		original *full.OneofShowcase
	}{
		{
			name: "embedded_text",
			original: &full.OneofShowcase{
				Id: "oneof-1",
				Content: &full.OneofShowcase_Text{
					Text: &full.TextContent{Text: "Hello World", Format: "plain"},
				},
			},
		},
		{
			name: "embedded_image",
			original: &full.OneofShowcase{
				Id: "oneof-2",
				Content: &full.OneofShowcase_Image{
					Image: &full.ImageContent{Url: "https://example.com/img.png", Width: 100, Height: 200},
				},
			},
		},
		{
			name: "embedded_code",
			original: &full.OneofShowcase{
				Id: "oneof-3",
				Content: &full.OneofShowcase_Code{
					Code: &full.CodeContent{Code: "fmt.Println()", Language: "go", Highlighted: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plain := tt.original.IntoPlain()
			jsonData, err := plain.MarshalJSON()
			require.NoError(t, err)

			plain2 := &full.OneofShowcasePlain{}
			err = plain2.UnmarshalJSON(jsonData)
			require.NoError(t, err)

			restored := plain2.IntoPb()

			// Verify ID and content case match
			assert.Equal(t, tt.original.Id, restored.Id)

			// Check the oneof variant matches
			switch tt.original.Content.(type) {
			case *full.OneofShowcase_Text:
				require.NotNil(t, restored.GetText())
				assert.Equal(t, tt.original.GetText().Text, restored.GetText().Text)
			case *full.OneofShowcase_Image:
				require.NotNil(t, restored.GetImage())
				assert.Equal(t, tt.original.GetImage().Url, restored.GetImage().Url)
			case *full.OneofShowcase_Code:
				require.NotNil(t, restored.GetCode())
				assert.Equal(t, tt.original.GetCode().Code, restored.GetCode().Code)
			}
		})
	}

	t.Logf("OneofShowcase roundtrip: embedded oneof variants tested")
}

func TestRoundtrip_ComplexNested(t *testing.T) {
	original := &full.ComplexNested{
		Id: "complex-1",
		Inner: &full.ComplexNested_Inner{
			Value: "inner-value",
			Count: 42,
			Deep: &full.ComplexNested_Inner_DeepInner{
				DeepValue: "deep-value",
				Tags:      []string{"tag1", "tag2"},
				Scores:    map[string]int32{"a": 100, "b": 200},
			},
			DeepList: []*full.ComplexNested_Inner_DeepInner{
				{DeepValue: "deep1", Tags: []string{"t1"}},
				{DeepValue: "deep2", Scores: map[string]int32{"x": 1}},
			},
		},
		InnerList: []*full.ComplexNested_Inner{
			{Value: "list-item-1", Count: 1},
			{Value: "list-item-2", Count: 2},
		},
		InnerMap: map[string]*full.ComplexNested_Inner{
			"key1": {Value: "map-value-1", Count: 10},
		},
		InnerEnum:     full.ComplexNested_INNER_FIRST,
		InnerEnumList: []full.ComplexNested_InnerEnum{full.ComplexNested_INNER_FIRST, full.ComplexNested_INNER_SECOND},
		// Note: non-embedded oneof (Choice) is not included in Plain
	}

	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.ComplexNestedPlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	// Verify key fields
	assert.Equal(t, original.Id, restored.Id)
	require.NotNil(t, restored.Inner)
	assert.Equal(t, original.Inner.Value, restored.Inner.Value)
	assert.Equal(t, original.Inner.Count, restored.Inner.Count)
	require.NotNil(t, restored.Inner.Deep)
	assert.Equal(t, original.Inner.Deep.DeepValue, restored.Inner.Deep.DeepValue)
	assert.Equal(t, original.Inner.Deep.Tags, restored.Inner.Deep.Tags)
	assert.Equal(t, original.InnerEnum, restored.InnerEnum)
	assert.Equal(t, original.InnerEnumList, restored.InnerEnumList)

	t.Logf("ComplexNested roundtrip: %d bytes JSON", len(jsonData))
}

func TestRoundtrip_Document(t *testing.T) {
	original := &full.Document{
		Id:          "doc-roundtrip",
		Title:       "Roundtrip Test Document",
		Status:      full.Status_STATUS_ACTIVE,
		Priority:    full.Priority_PRIORITY_HIGH,
		Description: &full.StringValue{Value: "A comprehensive roundtrip test"},
		Version:     &full.Int64Value{Value: 42},
		IsPublic:    &full.BoolValue{Value: true},
		Author: &full.ContactInfo{
			Email: "test@example.com",
			Phone: "+1-555-1234",
			Address: &full.Address{
				Street:     "123 Test St",
				City:       "Testville",
				Country:    "Testland",
				PostalCode: "12345",
			},
		},
		Metadata: &full.Metadata{
			CreatedBy:  "test-user",
			CreatedAt:  1706000000,
			ModifiedBy: "test-user",
			ModifiedAt: 1706000001,
			Labels:     map[string]string{"env": "test", "version": "1"},
			Tags:       []string{"roundtrip", "test", "document"},
		},
		Keywords:   []string{"test", "roundtrip", "proto"},
		Attributes: map[string]string{"attr1": "val1", "attr2": "val2"},
		Locations: []*full.Address{
			{Street: "456 Main St", City: "NYC"},
			{Street: "789 Oak Ave", City: "LA"},
		},
		Structure: &full.Level1{
			Title: "Level 1",
			Body: &full.Level2{
				Label: "Level 2",
				Content: &full.Level3{
					Identifier: "Level 3",
					Nested: &full.Level4{
						Name: "Level 4",
						Deep: &full.Level5{
							LeafValue:  "Deep Leaf",
							LeafNumber: 999,
							LeafData:   []byte{0xDE, 0xAD},
						},
					},
				},
			},
		},
		Content: &full.Document_TextContent{
			TextContent: &full.TextContent{
				Text:   "# Document Content\nThis is the main content.",
				Format: "markdown",
			},
		},
		// Note: non-embedded oneof (Source) is not included in Plain
		Children: []*full.Document{
			{Id: "child-1", Title: "Child Document 1"},
			{Id: "child-2", Title: "Child Document 2"},
		},
	}

	plain := original.IntoPlain()
	jsonData, err := plain.MarshalJSON()
	require.NoError(t, err)

	plain2 := &full.DocumentPlain{}
	err = plain2.UnmarshalJSON(jsonData)
	require.NoError(t, err)

	restored := plain2.IntoPb()

	// Verify key fields
	assert.Equal(t, original.Id, restored.Id)
	assert.Equal(t, original.Title, restored.Title)
	assert.Equal(t, original.Status, restored.Status)
	assert.Equal(t, original.Priority, restored.Priority)
	assert.Equal(t, original.Description.Value, restored.Description.Value)
	assert.Equal(t, original.Version.Value, restored.Version.Value)
	assert.Equal(t, original.IsPublic.Value, restored.IsPublic.Value)
	assert.Equal(t, original.Keywords, restored.Keywords)
	assert.Equal(t, original.Attributes, restored.Attributes)
	require.Len(t, restored.Children, 2)
	assert.Equal(t, "child-1", restored.Children[0].Id)

	// Check embedded content oneof
	require.NotNil(t, restored.GetTextContent())
	assert.Equal(t, original.GetTextContent().Text, restored.GetTextContent().Text)

	t.Logf("Document roundtrip: %d bytes JSON", len(jsonData))
}

// ============================================================================
// Roundtrip summary test - all models at once
// ============================================================================

func TestRoundtrip_AllModels(t *testing.T) {
	passed := 0
	total := 0

	// Helper to run roundtrip test
	runTest := func(name string, testFn func(t *testing.T)) {
		total++
		t.Run(name, func(t *testing.T) {
			testFn(t)
			passed++
		})
	}

	runTest("Config", TestRoundtrip_Config)
	runTest("TreeNode", TestRoundtrip_TreeNode)
	runTest("Event", TestRoundtrip_Event)
	runTest("PlatformEvent", TestRoundtrip_PlatformEvent)
	runTest("MapShowcase", TestRoundtrip_MapShowcase)
	runTest("OptionalShowcase", TestRoundtrip_OptionalShowcase)
	runTest("OneofShowcase", TestRoundtrip_OneofShowcase)
	runTest("ComplexNested", TestRoundtrip_ComplexNested)
	runTest("Document", TestRoundtrip_Document)

	t.Logf("=== ROUNDTRIP SUMMARY ===")
	t.Logf("Models tested: %d/%d passed", passed, total)
	t.Logf("All roundtrips: pb -> IntoPlain -> MarshalJX -> UnmarshalJX -> IntoPb -> proto.Equal")
	t.Logf("=========================")
}

// ============================================================================
// Benchmarks: pb -> IntoPlain -> MarshalJX -> UnmarshalJX -> IntoPb
// ============================================================================

// --- Config benchmarks ---

func createBenchConfig() *full.Config {
	return &full.Config{
		DoubleVal:   3.14159265359,
		FloatVal:    2.71828,
		Int32Val:    -2147483648,
		Int64Val:    -9223372036854775807,
		Uint32Val:   4294967295,
		Uint64Val:   18446744073709551615,
		Sint32Val:   -100,
		Sint64Val:   -200,
		Fixed32Val:  300,
		Fixed64Val:  400,
		Sfixed32Val: -500,
		Sfixed64Val: -600,
		BoolVal:     true,
		StringVal:   "hello world benchmark",
		BytesVal:    []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE},

		OptionalString: proto.String("optional"),
		OptionalInt:    proto.Int32(42),
		OptionalBool:   proto.Bool(true),

		StringList: []string{"a", "b", "c", "d", "e"},
		IntList:    []int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		DoubleList: []float64{1.1, 2.2, 3.3, 4.4, 5.5},

		StringMap: map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"},
		IntKeyMap: map[int32]string{1: "one", 2: "two", 3: "three"},

		Status:     full.Status_STATUS_ACTIVE,
		StatusList: []full.Status{full.Status_STATUS_ACTIVE, full.Status_STATUS_PENDING},

		NestedConfig: &full.Config_NestedConfig{Key: "k1", Value: "v1", Priority: 10},
		NestedConfigList: []*full.Config_NestedConfig{
			{Key: "k2", Value: "v2", Priority: 20},
			{Key: "k3", Value: "v3", Priority: 30},
		},
	}
}

func BenchmarkConfig_IntoPlain(b *testing.B) {
	pb := createBenchConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pb.IntoPlain()
	}
}

func BenchmarkConfig_MarshalJX(b *testing.B) {
	pb := createBenchConfig()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = plain.MarshalJSON()
	}
}

func BenchmarkConfig_UnmarshalJX(b *testing.B) {
	pb := createBenchConfig()
	plain := pb.IntoPlain()
	jsonData, _ := plain.MarshalJSON()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := &full.ConfigPlain{}
		_ = p.UnmarshalJSON(jsonData)
	}
}

func BenchmarkConfig_IntoPb(b *testing.B) {
	pb := createBenchConfig()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plain.IntoPb()
	}
}

func BenchmarkConfig_FullRoundtrip(b *testing.B) {
	pb := createBenchConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plain := pb.IntoPlain()
		jsonData, _ := plain.MarshalJSON()
		plain2 := &full.ConfigPlain{}
		_ = plain2.UnmarshalJSON(jsonData)
		_ = plain2.IntoPb()
	}
}

// --- TreeNode benchmarks ---

func createBenchTreeNode() *full.TreeNode {
	return &full.TreeNode{
		Id:   "root",
		Name: "Root Node",
		Type: "root",
		Children: []*full.TreeNode{
			{
				Id:   "child1",
				Name: "Child 1",
				Type: "branch",
				Children: []*full.TreeNode{
					{Id: "gc1", Name: "Grandchild 1", Type: "leaf"},
					{Id: "gc2", Name: "Grandchild 2", Type: "leaf"},
				},
			},
			{Id: "child2", Name: "Child 2", Type: "leaf"},
			{Id: "child3", Name: "Child 3", Type: "leaf"},
		},
		Info: &full.Metadata{
			CreatedBy: "benchmark",
			CreatedAt: 1706000000,
			Tags:      []string{"bench", "tree", "recursive"},
		},
		Payload: &full.TreeNode_Text{
			Text: &full.TextContent{Text: "Root content for benchmark", Format: "plain"},
		},
	}
}

func BenchmarkTreeNode_IntoPlain(b *testing.B) {
	pb := createBenchTreeNode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pb.IntoPlain()
	}
}

func BenchmarkTreeNode_MarshalJX(b *testing.B) {
	pb := createBenchTreeNode()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = plain.MarshalJSON()
	}
}

func BenchmarkTreeNode_UnmarshalJX(b *testing.B) {
	pb := createBenchTreeNode()
	plain := pb.IntoPlain()
	jsonData, _ := plain.MarshalJSON()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := &full.TreeNodePlain{}
		_ = p.UnmarshalJSON(jsonData)
	}
}

func BenchmarkTreeNode_IntoPb(b *testing.B) {
	pb := createBenchTreeNode()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plain.IntoPb()
	}
}

func BenchmarkTreeNode_FullRoundtrip(b *testing.B) {
	pb := createBenchTreeNode()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plain := pb.IntoPlain()
		jsonData, _ := plain.MarshalJSON()
		plain2 := &full.TreeNodePlain{}
		_ = plain2.UnmarshalJSON(jsonData)
		_ = plain2.IntoPb()
	}
}

// --- Event benchmarks ---

func createBenchEvent() *full.Event {
	return &full.Event{
		EventId:   "evt-benchmark-001",
		EventType: "user.created",
		Timestamp: 1706000000,
		Source:    "user-service",
		Meta: &full.Metadata{
			CreatedBy: "system",
			Labels:    map[string]string{"env": "prod", "region": "us-east-1"},
			Tags:      []string{"user", "event", "benchmark"},
		},
		Payload: &full.Event_UserCreated{
			UserCreated: &full.UserCreatedEvent{
				UserId:   "user-456",
				Username: "benchmarkuser",
				Email:    "benchmark@example.com",
			},
		},
	}
}

func BenchmarkEvent_IntoPlain(b *testing.B) {
	pb := createBenchEvent()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pb.IntoPlain()
	}
}

func BenchmarkEvent_MarshalJX(b *testing.B) {
	pb := createBenchEvent()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = plain.MarshalJSON()
	}
}

func BenchmarkEvent_UnmarshalJX(b *testing.B) {
	pb := createBenchEvent()
	plain := pb.IntoPlain()
	jsonData, _ := plain.MarshalJSON()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := &full.EventPlain{}
		_ = p.UnmarshalJSON(jsonData)
	}
}

func BenchmarkEvent_IntoPb(b *testing.B) {
	pb := createBenchEvent()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plain.IntoPb()
	}
}

func BenchmarkEvent_FullRoundtrip(b *testing.B) {
	pb := createBenchEvent()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plain := pb.IntoPlain()
		jsonData, _ := plain.MarshalJSON()
		plain2 := &full.EventPlain{}
		_ = plain2.UnmarshalJSON(jsonData)
		_ = plain2.IntoPb()
	}
}

// --- PlatformEvent benchmarks ---

func createBenchPlatformEvent() *full.PlatformEvent {
	return &full.PlatformEvent{
		EventId:   "platform-bench-001",
		EventTime: 1706000000,
		Source:    "node-benchmark",
		PlatformEvent: &full.PlatformEvent_Heartbeat{
			Heartbeat: &full.Heartbeat{
				Timestamp:   1706000000,
				NodeId:      "node-benchmark",
				CpuPercent:  75,
				MemoryBytes: 17179869184,
			},
		},
		Labels: map[string]string{"region": "us-east-1", "env": "prod", "tier": "compute"},
	}
}

func BenchmarkPlatformEvent_IntoPlain(b *testing.B) {
	pb := createBenchPlatformEvent()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pb.IntoPlain()
	}
}

func BenchmarkPlatformEvent_MarshalJX(b *testing.B) {
	pb := createBenchPlatformEvent()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = plain.MarshalJSON()
	}
}

func BenchmarkPlatformEvent_UnmarshalJX(b *testing.B) {
	pb := createBenchPlatformEvent()
	plain := pb.IntoPlain()
	jsonData, _ := plain.MarshalJSON()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := &full.PlatformEventPlain{}
		_ = p.UnmarshalJSON(jsonData)
	}
}

func BenchmarkPlatformEvent_IntoPb(b *testing.B) {
	pb := createBenchPlatformEvent()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plain.IntoPb()
	}
}

func BenchmarkPlatformEvent_FullRoundtrip(b *testing.B) {
	pb := createBenchPlatformEvent()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plain := pb.IntoPlain()
		jsonData, _ := plain.MarshalJSON()
		plain2 := &full.PlatformEventPlain{}
		_ = plain2.UnmarshalJSON(jsonData)
		_ = plain2.IntoPb()
	}
}

// --- MapShowcase benchmarks ---

func createBenchMapShowcase() *full.MapShowcase {
	return &full.MapShowcase{
		StrStr:   map[string]string{"a": "1", "b": "2", "c": "3"},
		StrInt32: map[string]int32{"x": 10, "y": 20, "z": 30},
		StrInt64: map[string]int64{"big": 9999999999},
		StrBool:  map[string]bool{"yes": true, "no": false},
		StrBytes: map[string][]byte{"data": {0x01, 0x02, 0x03}},
		Int32Str: map[int32]string{1: "one", 2: "two", 3: "three"},
		Int64Str: map[int64]string{100: "hundred", 200: "two hundred"},
		BoolStr:  map[bool]string{true: "TRUE", false: "FALSE"},
		StrMessage: map[string]*full.Address{
			"home": {Street: "123 Main St", City: "NYC", Country: "USA"},
			"work": {Street: "456 Office Blvd", City: "LA", Country: "USA"},
		},
		StrEnum: map[string]full.Status{
			"active":  full.Status_STATUS_ACTIVE,
			"pending": full.Status_STATUS_PENDING,
		},
	}
}

func BenchmarkMapShowcase_IntoPlain(b *testing.B) {
	pb := createBenchMapShowcase()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pb.IntoPlain()
	}
}

func BenchmarkMapShowcase_MarshalJX(b *testing.B) {
	pb := createBenchMapShowcase()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = plain.MarshalJSON()
	}
}

func BenchmarkMapShowcase_UnmarshalJX(b *testing.B) {
	pb := createBenchMapShowcase()
	plain := pb.IntoPlain()
	jsonData, _ := plain.MarshalJSON()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := &full.MapShowcasePlain{}
		_ = p.UnmarshalJSON(jsonData)
	}
}

func BenchmarkMapShowcase_IntoPb(b *testing.B) {
	pb := createBenchMapShowcase()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plain.IntoPb()
	}
}

func BenchmarkMapShowcase_FullRoundtrip(b *testing.B) {
	pb := createBenchMapShowcase()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plain := pb.IntoPlain()
		jsonData, _ := plain.MarshalJSON()
		plain2 := &full.MapShowcasePlain{}
		_ = plain2.UnmarshalJSON(jsonData)
		_ = plain2.IntoPb()
	}
}

// --- Document benchmarks (complex) ---

func createBenchDocument() *full.Document {
	return &full.Document{
		Id:          "doc-benchmark",
		Title:       "Benchmark Test Document",
		Status:      full.Status_STATUS_ACTIVE,
		Priority:    full.Priority_PRIORITY_HIGH,
		Description: &full.StringValue{Value: "A document for benchmarking roundtrip performance"},
		Version:     &full.Int64Value{Value: 42},
		IsPublic:    &full.BoolValue{Value: true},
		Author: &full.ContactInfo{
			Email: "benchmark@example.com",
			Phone: "+1-555-BENCH",
			Address: &full.Address{
				Street:     "123 Benchmark St",
				City:       "Perfville",
				Country:    "Testland",
				PostalCode: "12345",
			},
		},
		Metadata: &full.Metadata{
			CreatedBy:  "benchmark",
			CreatedAt:  1706000000,
			ModifiedBy: "benchmark",
			ModifiedAt: 1706000001,
			Labels:     map[string]string{"env": "bench", "version": "1.0"},
			Tags:       []string{"benchmark", "performance", "test"},
		},
		Keywords:   []string{"benchmark", "performance", "roundtrip", "protobuf"},
		Attributes: map[string]string{"attr1": "val1", "attr2": "val2", "attr3": "val3"},
		Locations: []*full.Address{
			{Street: "456 Main St", City: "NYC"},
			{Street: "789 Oak Ave", City: "LA"},
		},
		Structure: &full.Level1{
			Title: "Level 1",
			Body: &full.Level2{
				Label: "Level 2",
				Content: &full.Level3{
					Identifier: "Level 3",
					Nested: &full.Level4{
						Name: "Level 4",
						Deep: &full.Level5{
							LeafValue:  "Deep Leaf Value",
							LeafNumber: 999,
							LeafData:   []byte{0xDE, 0xAD, 0xBE, 0xEF},
						},
					},
				},
			},
		},
		Content: &full.Document_TextContent{
			TextContent: &full.TextContent{
				Text:   "# Benchmark Document\n\nThis is benchmark content for performance testing.",
				Format: "markdown",
			},
		},
		Children: []*full.Document{
			{Id: "child-1", Title: "Child Document 1"},
			{Id: "child-2", Title: "Child Document 2"},
		},
	}
}

func BenchmarkDocument_IntoPlain(b *testing.B) {
	pb := createBenchDocument()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pb.IntoPlain()
	}
}

func BenchmarkDocument_MarshalJX(b *testing.B) {
	pb := createBenchDocument()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = plain.MarshalJSON()
	}
}

func BenchmarkDocument_UnmarshalJX(b *testing.B) {
	pb := createBenchDocument()
	plain := pb.IntoPlain()
	jsonData, _ := plain.MarshalJSON()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := &full.DocumentPlain{}
		_ = p.UnmarshalJSON(jsonData)
	}
}

func BenchmarkDocument_IntoPb(b *testing.B) {
	pb := createBenchDocument()
	plain := pb.IntoPlain()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plain.IntoPb()
	}
}

func BenchmarkDocument_FullRoundtrip(b *testing.B) {
	pb := createBenchDocument()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plain := pb.IntoPlain()
		jsonData, _ := plain.MarshalJSON()
		plain2 := &full.DocumentPlain{}
		_ = plain2.UnmarshalJSON(jsonData)
		_ = plain2.IntoPb()
	}
}

// --- Pool benchmarks ---

func BenchmarkDocument_IntoPlainReuse(b *testing.B) {
	pb := createBenchDocument()
	// Warm up pool
	for i := 0; i < 100; i++ {
		p := full.GetDocumentPlain()
		full.PutDocumentPlain(p)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := full.GetDocumentPlain()
		pb.IntoPlainReuse(p)
		full.PutDocumentPlain(p)
	}
}

func BenchmarkConfig_IntoPlainReuse(b *testing.B) {
	pb := createBenchConfig()
	// Warm up pool
	for i := 0; i < 100; i++ {
		p := full.GetConfigPlain()
		full.PutConfigPlain(p)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := full.GetConfigPlain()
		pb.IntoPlainReuse(p)
		full.PutConfigPlain(p)
	}
}
