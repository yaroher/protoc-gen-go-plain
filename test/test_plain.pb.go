package test

type VirtualTypePlain struct {
	File *FilePlain `json:"file,omitempty"`
}

type EventPlain struct {
	EventVirtualType            string        `json:"eventVirtualType"`
	Path                        *string       `json:"path,omitempty"`
	OtherEvent                  *string       `json:"otherEvent,omitempty"`
	NonPlatformEventCustomEvent *string       `json:"nonPlatformEventCustomEvent,omitempty"`
	NonPlatformEventPath        string        `json:"nonPlatformEventPath"`
	ParentEventId               string        `json:"parentEventId"`
	EventId                     int32         `json:"eventId"`
	SomeEventStringPayload      string        `json:"someEventStringPayload"`
	Process                     *ProcessPlain `json:"process,omitempty"`
	PathCRF                     string        `json:"pathCRF"`
}

type EventDataPlain struct {
	Path                        *string                    `json:"path,omitempty"`
	PathCRF                     string                     `json:"pathCRF"`
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
