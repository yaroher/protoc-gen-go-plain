# Proto Plain Generation

## Source Proto

```protobuf
option (goplain.file).virtual_types = {
    name: "VirtualType",
    fields: [
    {
        kind: TYPE_MESSAGE,
        cardinality: CARDINALITY_OPTIONAL,
        number: 1,
        name: "file",
        type_url: "test.File",
    }
    ],
};

message File {
    string path = 1;
}

message FileRename {
    option (goplain.message).generate = true;
    File file = 1 [(goplain.field).embed = true];
}

message FileCreate {
    option (goplain.message).generate = true;
    File file = 1 [(goplain.field).embed = true];
}

message Process {
    option (goplain.message).generate = true;
    File file = 1;
}

message EventData {
    option (goplain.message).generate = true;
    oneof platform_event {
        option (goplain.oneof).embed = true;
        FileRename file_rename = 1 [(goplain.field).embed = true];
        FileCreate file_create = 2 [(goplain.field).embed = true];
        string other_event = 3;
    }
    oneof non_platform_event {
        option (goplain.oneof).embed_with_prefix = true;
        string custom_event = 4;
        File file = 5 [(goplain.field).embed = true];
    }
    oneof no_removed_oneof {
        string no_remove = 6;
        File not_removed_file = 7 [(goplain.field).embed = true];
    }
}

message Event {
    option (goplain.message).generate = true;
    option (goplain.message).virtual_fields = {
        name: "event_virtual_type",
        kind: TYPE_STRING
    };
    int32 event_id = 1;
    string some_event_string_payload = 2 [(goplain.field).override_type = {name: "UUID", import_path: "github.com/google/uuid"}];
    Process process = 3;
    EventData data = 4 [(goplain.field).embed = true];
    string parent_event_id = 5;
}
```

---

## Правила генерации

1. **`embed=true` на поле** → поля вложенного message встраиваются в родителя
2. **`embed=true` на oneof** → oneof растворяется, поля становятся optional
3. **`embed_with_prefix=true`** → добавляется префикс имени oneof
4. **Коллизии** → поля с одинаковым именем и типом объединяются (требует `enable_crf=true`)
5. **CRF (Collision Resolution Field)** → для каждого объединённого поля добавляется `{Field}CRF` с EmPath

---

## Флаг enable_crf

```bash
protoc --go-plain_out=enable_crf=true:. file.proto
```

| `enable_crf` | Поведение |
|--------------|-----------|
| `false` (default) | Коллизии запрещены → ошибка генерации |
| `true` | Коллизии разрешены → добавляются CRF поля |

---

## Результат генерации

```golang
import "github.com/google/uuid"

// === Базовые типы ===

type VirtualTypePlain struct {
    File *FilePlain `json:"file"`
}

type FilePlain struct {
    Path string `json:"path"`
}

// FileRename - File встроен
type FileRenamePlain struct {
    Path string `json:"path"` // from File.path
}

// FileCreate - File встроен
type FileCreatePlain struct {
    Path string `json:"path"` // from File.path
}

// Process - File НЕ встроен
type ProcessPlain struct {
    File *FilePlain `json:"file"`
}

// === EventData ===

type EventDataPlain struct {
    // --- platform_event (embed=true) ---
    // FileRename.file.path и FileCreate.file.path ОБЪЕДИНЯЮТСЯ
    // т.к. одинаковое имя "path" и тип "string"
    Path    *string `json:"path,omitempty"`
    PathCRF string  `json:"pathCRF,omitempty"` // EmPath: "platform_event/file_rename/file" или "platform_event/file_create/file"

    OtherEvent *string `json:"otherEvent,omitempty"`
    // OtherEvent не имеет коллизий → без CRF

    // --- non_platform_event (embed_with_prefix=true) ---
    // Префикс избегает коллизий → без CRF
    NonPlatformEventCustomEvent *string `json:"nonPlatformEventCustomEvent,omitempty"`
    NonPlatformEventPath        *string `json:"nonPlatformEventPath,omitempty"`

    // --- no_removed_oneof (без embed) ---
    // Остаётся как oneof из protoc-gen-go
    NoRemovedOneof isEventData_NoRemovedOneof
}

// === Event (итоговая структура) ===

type EventPlain struct {
    // Virtual field
    EventVirtualType string `json:"eventVirtualType"`

    // Regular fields
    EventId                int32         `json:"eventId"`
    SomeEventStringPayload uuid.UUID     `json:"someEventStringPayload"` // override_type
    Process                *ProcessPlain `json:"process"`
    ParentEventId          string        `json:"parentEventId"`

    // --- EventData embedded (data) ---
    // platform_event (объединённые поля):
    Path    *string `json:"path,omitempty"`
    PathCRF string  `json:"pathCRF,omitempty"` // CRF - Collision Resolution Field

    OtherEvent *string `json:"otherEvent,omitempty"`

    // non_platform_event (с префиксом, без коллизий):
    NonPlatformEventCustomEvent *string `json:"nonPlatformEventCustomEvent,omitempty"`
    NonPlatformEventPath        *string `json:"nonPlatformEventPath,omitempty"`

    // no_removed_oneof (NOT embedded):
    NoRemovedOneof isEventData_NoRemovedOneof
}
```

---

## CRF (Collision Resolution Field)

Когда поля объединяются из-за коллизии, добавляется поле `{Name}CRF` с EmPath:

```
Источник: FileRename.file.path
EmPath:   "platform_event/file_rename/file"

Источник: FileCreate.file.path
EmPath:   "platform_event/file_create/file"
```

**Формат EmPath:** `{oneof_name}/{field_name}/{nested_field_name}/...`

Маркеры можно включить для дополнительной информации:
```
"platform_event?embed=true/file_rename?embed=true/file?embed=true"
```

---

## Пример использования

```golang
event := EventPlain{
    EventId: 1,
    Path:    ptr("/tmp/test.txt"),
    PathCRF: "platform_event/file_rename/file", // знаем что это FileRename
}

// Проверка источника через CRF:
switch {
case strings.Contains(event.PathCRF, "file_rename"):
    // обработка FileRename
case strings.Contains(event.PathCRF, "file_create"):
    // обработка FileCreate
}
```

---

## Когда добавляется CRF

| Ситуация | CRF добавляется? |
|----------|------------------|
| Поле из embedded oneof, есть коллизия | ✅ Да |
| Поле из embedded oneof, нет коллизии | ❌ Нет (имя уникально) |
| Поле с `embed_with_prefix` | ❌ Нет (префикс делает имя уникальным) |
| Обычное поле (не из oneof) | ❌ Нет |
| Поле из НЕ-embedded oneof | ❌ Нет (oneof остаётся) |
