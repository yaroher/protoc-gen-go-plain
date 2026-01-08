# protoc-gen-go-plain

This README documents how proto fields are transformed into "plain" Go structs and how conversion methods behave.

## Generation scope

- Only messages with `option (goplain.message).generate = true;` are generated.
- Messages marked `option (goplain.message).type_alias = true;` are **not** generated as plain structs.
- Enums are kept as the original generated Go enum type (no renaming).
- Plain type name is `MessageName + PlainSuffix` (default suffix: `Plain`, configurable).

## Naming rules

- Field names match protoc-gen-go Go names (`GoName`).
- Oneof fields are emitted as standalone fields with their Go names.
- Virtual field names are sanitized to valid Go identifiers.

## Scalar fields

Proto scalar kinds map to Go scalars:

- bool -> bool
- int32/sint32/sfixed32 -> int32
- uint32/fixed32 -> uint32
- int64/sint64/sfixed64 -> int64
- uint64/fixed64 -> uint64
- float -> float32
- double -> float64
- string -> string
- bytes -> []byte

## Optional fields (proto3 optional)

Optional fields become pointers: `T -> *T`.

## Oneof fields

Each oneof field becomes a separate nullable field in the plain struct:

- Non-message oneof fields are pointers (e.g. `*int32`, `*string`, `*[]byte`).
- Message oneof fields are pointers to the message type.

## Repeated fields

Repeated fields become slices:

- Scalars: `[]T`.
- Messages: `[]*Message`.
- Enums: `[]Enum`.

## Map fields

Map fields become Go maps with the same key type rules as protoc-gen-go.

- Keys are always scalar (as in protobuf).
- Message values become `*Message` in the map value type.

## Message fields

By default, message fields become pointers to the generated Go message type:

- `Message` -> `*Message` (for field), `[]*Message` (for repeated), `map[K]*Message` (for map values).

## Embedded fields

If a field has `(goplain.field).embedded = true`, the fields of that message
are flattened into the parent plain struct (recursively). The original field
is not present in the plain struct.

## Serialized fields

If a field has `(goplain.field).serialized = true`, the plain type is `[]byte`
and conversions use `cast.MessageToSliceByte` / `cast.MessageFromSliceByte`.

## Type aliases

A `type_alias` message must have exactly one field named `value`.
Fields of alias type are treated as if the alias wrapper does not exist:

- Plain type is derived from `value`.
- PB->Plain uses `msg.Value` conversion.
- Plain->PB wraps the value into `{Value: ...}`.

## Well-known types (WKT)

The generator has built-in conversions:

- `google.protobuf.Timestamp` -> `*time.Time`
- `google.protobuf.Duration` -> `*time.Duration`
- `google.protobuf.Struct` -> `map[string]any`
- `google.protobuf.Value` -> `[]byte` (serialized)
- `google.protobuf.ListValue` -> `[]byte` (serialized)
- `google.protobuf.Any` -> `[]byte` (serialized)
- `google.protobuf.Empty` -> `*struct{}`
- Wrapper types (`StringValue`, `Int64Value`, etc.) -> `*T` (pointer to scalar)

## Overrides (type replacement)

Overrides can be defined:

- Globally (generator options),
- Per file (`option (goplain.file).overwrite = {...}`),
- Per field (`(goplain.field).overwrite = {...}`).

Override matching uses:

- Full proto type name for messages/enums, or
- Scalar kind name (e.g. `string`, `bytes`, `int32`, etc.).

Override determines:

- Plain type (`go_type`),
- Converters (`to_plain` / `to_plain_body`, `to_pb` / `to_pb_body`),
- Pointer preference (`pointer`).

## Virtual fields

Virtual fields are added from options:

- `option (goplain.file).virtual_field = {...}`
- `option (goplain.message).virtual_fields = {...}`

In IR they are just fields with `IsVirtual = true` and a required `GoType`.
They are:

- Emitted into the plain struct.
- Not filled from PB during conversion.
- Set via options for `IntoPlain()` / `IntoPlainDeep()`.

Generated option helpers (only when virtual fields exist):

```
With<MessageName><FieldName>(value)
```

## Virtual messages

`option (goplain.file).virtual_message` defines extra plain structs generated
verbatim (not tied to proto messages).

## Conversion methods

Generated methods:

- `(*PB).IntoPlain()`
- `(*PB).IntoPlainDeep()`
- `(*Plain).IntoPb()`
- `(*Plain).IntoPbDeep()`

### Shallow vs Deep

- Shallow:
  - Maps/slices are assigned directly when no conversion is required.
  - Bytes are not cloned.

- Deep:
  - Maps/slices are copied element-by-element.
  - `[]byte` values are cloned.

Deep only affects containers and bytes; it does not deep-copy nested messages.

## Notes

- Virtual fields are ignored in PB->Plain and Plain->PB conversions.
- Embedded fields are flattened both in struct and in conversion logic.
- Type aliases are validated to have exactly one `value` field.
