package test

type VirtualTypePlain struct {
	File *FilePlain `json:"file,omitempty"`
}

type EventPlain struct {
	EventVirtualType            string        `json:"eventVirtualType"`
	EventId                     int32         `json:"eventId"`
	SomeEventStringPayload      string        `json:"someEventStringPayload"`
	Path                        string        `json:"path,omitempty"`
	NonPlatformEventCustomEvent *string       `json:"nonPlatformEventCustomEvent,omitempty"`
	NonPlatformEventPath        string        `json:"nonPlatformEventPath"`
	Process                     *ProcessPlain `json:"process,omitempty"`
	PathCrf                     string        `json:"pathCrf,omitempty"`
	OtherEvent                  *string       `json:"otherEvent,omitempty"`
	ParentEventId               string        `json:"parentEventId"`
}

type EventDataPlain struct {
	Path                        string                     `json:"path,omitempty"`
	PathCrf                     string                     `json:"pathCrf,omitempty"`
	OtherEvent                  *string                    `json:"otherEvent,omitempty"`
	NonPlatformEventCustomEvent *string                    `json:"nonPlatformEventCustomEvent,omitempty"`
	NonPlatformEventPath        string                     `json:"nonPlatformEventPath"`
	NoRemovedOneof              isEventData_NoRemovedOneof `json:"noRemovedOneof"`
}

type FilePlain struct {
	Path string `json:"path"`
}

type FileCreatePlain struct {
	Path string `json:"path"`
}

type FileRenamePlain struct {
	Path string `json:"path"`
}

type ProcessPlain struct {
	File *FilePlain `json:"file,omitempty"`
}
