# protoc-gen-go-plain

A `protoc` plugin that generates **plain Go structs** from Protocol Buffer definitions with field embedding, type overrides, high-performance JSON serialization, and zero-allocation object pooling.

## Why?

Standard `protoc-gen-go` generates structs tightly coupled to the protobuf runtime. This is great for serialization, but often awkward when you need:

- **Flat structs** for JSON APIs — nested messages become embedded fields
- **Custom Go types** — `int64` becomes `time.Duration`, `bytes` becomes `json.RawMessage`
- **Fast JSON** — using [jx](https://github.com/go-faster/jx) instead of `protojson`
- **Object pooling** — `sync.Pool` support with `Reset()` for zero-allocation hot paths
- **Virtual fields** — fields that exist only in the "plain" struct, not in the protobuf wire format

`protoc-gen-go-plain` generates `*Plain` companion structs with bidirectional conversion methods (`IntoPlain()` / `IntoPb()`), so you keep full protobuf compatibility while working with ergonomic Go types.

## Installation

```bash
go install github.com/yaroher/protoc-gen-go-plain@latest
```

Or build from source:

```bash
git clone https://github.com/yaroher/protoc-gen-go-plain.git
cd protoc-gen-go-plain
make build   # output: bin/protoc-gen-go-plain
```

## Quick Start

### 1. Import the options proto

```proto
import "goplain/goplain.proto";
```

### 2. Mark messages for generation

```proto
message User {
  option (goplain.message).generate = true;

  string id = 1;
  string name = 2;
  Address address = 3 [(goplain.field).embed = true];
}
```

### 3. Run protoc

```bash
protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-plain_out=. --go-plain_opt=paths=source_relative,json_jx=true,pool=true \
  --proto_path=. \
  your_service.proto
```

This generates `your_service_plain.pb.go` alongside the standard `.pb.go` with:

- `UserPlain` struct with `Address` fields flattened in
- `(*User).IntoPlain() *UserPlain` and `(*UserPlain).IntoPb() *User`
- `MarshalJSON()` / `UnmarshalJSON()` via jx
- `sync.Pool` helpers: `GetUserPlain()`, `PutUserPlain()`, `Reset()`

## Plugin Options

Pass via `--go-plain_opt=`:

| Option | Default | Description |
|--------|---------|-------------|
| `paths` | — | Output path mode (`source_relative`) |
| `json_jx` | `false` | Generate jx-based `MarshalJSON`/`UnmarshalJSON` for Plain structs |
| `jx_pb` | `false` | Generate jx-based JSON methods for original protobuf structs too |
| `pool` | `false` | Generate `sync.Pool` with `Get`/`Put`/`Reset` methods |
| `casters_as_struct` | `true` | Pass type casters as a single struct parameter (vs separate args) |
| `unified_oneof_json` | `false` | Use the original field name in JSON for all oneof variants |

## Features

### Field Embedding

Flatten nested messages into the parent struct:

```proto
message Address {
  string street = 1;
  string city = 2;
}

message User {
  option (goplain.message).generate = true;
  string name = 1;
  Address address = 2 [(goplain.field).embed = true];
}
```

Generated `UserPlain`:

```go
type UserPlain struct {
    Name   string
    Street string  // flattened from Address
    City   string  // flattened from Address
}
```

Use `embed_with_prefix = true` to prefix embedded fields (`AddressStreet`, `AddressCity`).

Collision detection will report an error if embedding creates duplicate field names.

### Oneof Embedding

Flatten oneof variants into the parent struct with a case tracking field:

```proto
message Event {
  option (goplain.message).generate = true;
  string id = 1;

  oneof payload {
    option (goplain.oneof).embed = true;
    Heartbeat heartbeat = 10 [(goplain.field).embed = true];
    ProcessStarted process_started = 11 [(goplain.field).embed = true];
  }
}
```

All variant fields are flattened into `EventPlain`, with a `PayloadCase` field tracking which variant is active.

### Type Aliases

Unwrap single-field wrapper messages:

```proto
message StringValue {
  option (goplain.message).type_alias = true;
  string value = 1;
}

message User {
  option (goplain.message).generate = true;
  StringValue nickname = 1;  // becomes `string` in UserPlain
}
```

### Type Overrides

**Field-level** — override a single field's Go type:

```proto
message CustomTypes {
  option (goplain.message).generate = true;
  bytes raw_json = 1 [(goplain.field).override_type = {
    name: "RawMessage", import_path: "encoding/json"
  }];
}
```

**File-level** — override types by selector (field kind, path, cardinality):

```proto
option (goplain.file).go_types_overrides = {
  selector: { field_kind: TYPE_INT64, target_full_path: "mypackage.Metrics.duration_ns" }
  target_go_type: { name: "Duration", import_path: "time" }
};
```

When source and target types are incompatible, the generator requests a `Caster[A, B]` parameter on conversion methods:

```go
type Caster[A any, B any] interface {
    Cast(v A) B
}
```

### Serialized Fields

Store a message field as `[]byte` (protobuf JSON) in the plain struct:

```proto
message User {
  option (goplain.message).generate = true;
  Metrics performance = 1 [(goplain.field).serialize = true];
}
// UserPlain.Performance will be []byte containing JSON
```

### Enum Handling

```proto
Status status = 1 [(goplain.field).enum_as_string = true];  // JSON: "STATUS_ACTIVE"
Status status = 1 [(goplain.field).enum_as_int = true];      // JSON: 1
```

### Virtual Fields

Add fields that exist only in the Plain struct:

```proto
message User {
  option (goplain.message).generate = true;
  option (goplain.message).virtual_fields = {
    name: "password_hash", kind: TYPE_STRING
  };
  string name = 1;
}
// UserPlain has PasswordHash string, but User does not
```

### Write Default

Force zero-value fields to be included in JSON output:

```proto
int32 count = 1 [(goplain.field).write_default = true];
// JSON: {"count": 0} instead of omitting the field
```

### JSON Serialization (jx)

With `json_jx=true`, each Plain struct gets:

```go
func (p *UserPlain) MarshalJX(e *jx.Encoder)      // high-performance encoding
func (p *UserPlain) MarshalJSON() ([]byte, error)  // stdlib compatible
func (p *UserPlain) UnmarshalJX(d *jx.Decoder)     // high-performance decoding
func (p *UserPlain) UnmarshalJSON(data []byte) error
```

With `jx_pb=true`, the same methods are also generated for the original protobuf structs.

### Object Pooling

With `pool=true`:

```go
var userPlainPool sync.Pool

func GetUserPlain() *UserPlain { ... }
func PutUserPlain(p *UserPlain) { ... }
func (p *UserPlain) Reset() { ... }

// Zero-allocation conversion
func (m *User) IntoPlainReuse(p *UserPlain)
```

### File-Level Virtual Types

Define Plain-only structs from `google.protobuf.Type` without a backing protobuf message:

```proto
option (goplain.file).virtual_types = {
  name: "AuditEntry"
  fields: [
    { name: "action", kind: TYPE_STRING },
    { name: "actor",  kind: TYPE_STRING },
    { name: "timestamp", kind: TYPE_INT64 }
  ]
};
```

## Proto Options Reference

### Message Options

```proto
option (goplain.message).generate = true;          // enable generation
option (goplain.message).type_alias = true;         // unwrap to inner field type
option (goplain.message).type_alias_field = "val";  // custom alias field name
option (goplain.message).virtual_fields = { ... };  // plain-only fields
```

### Field Options

```proto
(goplain.field).embed = true              // flatten into parent
(goplain.field).embed_with_prefix = true  // flatten with parent field name prefix
(goplain.field).serialize = true          // store as JSON bytes
(goplain.field).override_type = { ... }   // custom Go type
(goplain.field).enum_as_string = true     // JSON serialize enum as string
(goplain.field).enum_as_int = true        // JSON serialize enum as int
(goplain.field).write_default = true      // include zero values in JSON
```

### Oneof Options

```proto
option (goplain.oneof).embed = true;              // flatten variants
option (goplain.oneof).embed_with_prefix = true;  // flatten with prefix
```

### File Options

```proto
option (goplain.file).go_types_overrides = { ... };  // type override rules
option (goplain.file).virtual_types = { ... };        // standalone plain structs
```

## Benchmarks

All benchmarks run on Intel Core i5-14600K, Go 1.24, Linux amd64.

### JSON Marshal: go-plain/jx vs encoding/json vs protojson

| Message | go-plain/jx | encoding/json | protojson | jx vs protojson |
|---------|-------------|---------------|-----------|-----------------|
| Event (small) | **469 ns** / 0 B / 0 allocs | 1,898 ns / 353 B / 1 alloc | 3,184 ns / 2,407 B / 48 allocs | **6.8x faster** |
| Config (medium) | **2,531 ns** / 0 B / 0 allocs | 7,193 ns / 1,158 B / 1 alloc | 8,113 ns / 4,341 B / 88 allocs | **3.2x faster** |
| Document (complex) | **2,016 ns** / 640 B / 2 allocs | 7,161 ns / 1,928 B / 3 allocs | 13,383 ns / 9,833 B / 169 allocs | **6.6x faster** |

### JSON Unmarshal: go-plain/jx vs encoding/json vs protojson

| Message | go-plain/jx | encoding/json | protojson | jx vs protojson |
|---------|-------------|---------------|-----------|-----------------|
| Event (small) | **1,112 ns** / 1,048 B / 36 allocs | 2,916 ns / 1,368 B / 41 allocs | 4,749 ns / 2,000 B / 79 allocs | **4.3x faster** |
| Config (medium) | **4,241 ns** / 3,072 B / 115 allocs | 10,054 ns / 3,968 B / 120 allocs | 16,149 ns / 5,400 B / 252 allocs | **3.8x faster** |
| Document (complex) | **4,457 ns** / 4,984 B / 131 allocs | 11,255 ns / 5,560 B / 137 allocs | 18,011 ns / 6,680 B / 269 allocs | **4.0x faster** |

### Full Roundtrip: pb -> plain -> JSON -> plain -> pb

| Message | go-plain/jx | encoding/json | protojson | jx vs protojson |
|---------|-------------|---------------|-----------|-----------------|
| Event (small) | **1,748 ns** / 39 allocs | 5,172 ns / 45 allocs | 8,303 ns / 127 allocs | **4.8x faster** |
| Config (medium) | **8,029 ns** / 118 allocs | 18,624 ns / 123 allocs | 27,965 ns / 340 allocs | **3.5x faster** |
| Document (complex) | **8,141 ns** / 150 allocs | 20,597 ns / 157 allocs | 31,209 ns / 438 allocs | **3.8x faster** |

### Conversion & Pool

| Operation | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| Event IntoPlain | 38 | 128 | 1 |
| Event IntoPb | 45 | 136 | 2 |
| Config IntoPlain | 139 | 704 | 1 |
| Config IntoPb | 148 | 768 | 1 |
| Config IntoPlainReuse (pool) | **107** | **0** | **0** |
| Document IntoPlain | 374 | 1,664 | 4 |
| Document IntoPlainReuse (pool) | **319** | 1,345 | 3 |

Run benchmarks yourself:

```bash
make run-bench
# or with specific patterns:
go test -bench=BenchmarkEvent -benchmem ./test/full/
```

## Dependencies

| Package | Purpose |
|---------|---------|
| [go-faster/jx](https://github.com/go-faster/jx) | High-performance JSON encoding/decoding |
| [google.golang.org/protobuf](https://pkg.go.dev/google.golang.org/protobuf) | Protobuf compiler plugin framework |
| [iancoleman/strcase](https://github.com/iancoleman/strcase) | String case conversion |
| [uber-go/zap](https://github.com/uber-go/zap) | Structured logging (debug mode) |

## Development

```bash
make build              # build the plugin
make test-all           # run all tests (regenerates + tests)
make run-test           # run tests without regenerating
make run-bench          # run benchmarks

# Individual test suites
make build-test-full    # regenerate full showcase test
make build-test-nda     # regenerate NDA test
make run-test-collision # run collision detection tests
```

Debug logging:

```bash
LOG_LEVEL=debug LOG_FILE=./debug.txt protoc --plugin=... ...
```

## License

MIT
