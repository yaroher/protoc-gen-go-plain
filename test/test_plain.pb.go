package test

import (
	"github.com/google/uuid"
)

type VirtualTypePlain struct {
	File *FilePlain `json:"file,omitempty"`
}

type EventPlain struct {
	EventId                     int32         `json:"eventId"`
	SomeEventStringPayload      uuid.UUID     `json:"someEventStringPayload"`
	Process                     *ProcessPlain `json:"process,omitempty"`
	NonPlatformEventPath        string        `json:"nonPlatformEventPath"`
	OtherEvent                  *string       `json:"otherEvent,omitempty"`
	ParentEventId               string        `json:"parentEventId"`
	EventVirtualType            string        `json:"eventVirtualType"`
	Path                        *string       `json:"path,omitempty"`
	PathCRF                     string        `json:"pathCRF"`
	NonPlatformEventCustomEvent *string       `json:"nonPlatformEventCustomEvent,omitempty"`
}

type EventDataPlain struct {
	NonPlatformEventPath        string                     `json:"nonPlatformEventPath"`
	Path                        *string                    `json:"path,omitempty"`
	PathCRF                     string                     `json:"pathCRF"`
	OtherEvent                  *string                    `json:"otherEvent,omitempty"`
	NonPlatformEventCustomEvent *string                    `json:"nonPlatformEventCustomEvent,omitempty"`
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
