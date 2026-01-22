package test

import (
	"github.com/yaroher/protoc-gen-go-plain/into"
	"strings"
)

func (x *EventPlain) IntoPb() *Event {
	if x == nil {
		return nil
	}
	out := &Event{}
	_pathEventVirtualType := []string{"event_virtual_type"}
	into.SetString(out, _pathEventVirtualType, x.EventVirtualType)
	_pathPath := []string{"data", "file_rename", "file", "path"}
	if x.PathCRF != "" {
		_pathPath = into.ParseCRFPath(x.PathCRF)
	}
	if x.Path != nil {
		into.SetString(out, _pathPath, *x.Path)
	}
	_pathOtherEvent := []string{"data", "other_event"}
	if x.OtherEvent != nil {
		into.SetString(out, _pathOtherEvent, *x.OtherEvent)
	}
	_pathNonPlatformEventCustomEvent := []string{"data", "custom_event"}
	if x.NonPlatformEventCustomEvent != nil {
		into.SetString(out, _pathNonPlatformEventCustomEvent, *x.NonPlatformEventCustomEvent)
	}
	_pathNonPlatformEventPath := []string{"data", "file", "path"}
	into.SetString(out, _pathNonPlatformEventPath, x.NonPlatformEventPath)
	_pathParentEventId := []string{"parent_event_id"}
	into.SetString(out, _pathParentEventId, x.ParentEventId)
	_pathEventId := []string{"event_id"}
	into.SetInt32(out, _pathEventId, x.EventId)
	_pathSomeEventStringPayload := []string{"some_event_string_payload"}
	into.SetString(out, _pathSomeEventStringPayload, x.SomeEventStringPayload)
	_pathProcess := []string{"process"}
	if x.Process != nil {
		into.SetMessage(out, _pathProcess, x.Process.IntoPb())
	}
	_pathPathCrf := []string{"data", "pathCRF"}
	into.SetString(out, _pathPathCrf, x.PathCRF)
	return out
}

func (x *Event) IntoPlain() *EventPlain {
	if x == nil {
		return nil
	}
	out := &EventPlain{}
	_pathEventVirtualType := []string{"event_virtual_type"}
	if v, ok := into.GetString(x, _pathEventVirtualType); ok {
		out.EventVirtualType = v
	}
	_pathPath := []string{"data", "file_rename", "file", "path"}
	if v, ok := into.GetString(x, _pathPath); ok {
		out.Path = &v
		out.PathCRF = strings.Join(_pathPath, "/")
	}
	_pathOtherEvent := []string{"data", "other_event"}
	if v, ok := into.GetString(x, _pathOtherEvent); ok {
		out.OtherEvent = &v
	}
	_pathNonPlatformEventCustomEvent := []string{"data", "custom_event"}
	if v, ok := into.GetString(x, _pathNonPlatformEventCustomEvent); ok {
		out.NonPlatformEventCustomEvent = &v
	}
	_pathNonPlatformEventPath := []string{"data", "file", "path"}
	if v, ok := into.GetString(x, _pathNonPlatformEventPath); ok {
		out.NonPlatformEventPath = v
	}
	_pathParentEventId := []string{"parent_event_id"}
	if v, ok := into.GetString(x, _pathParentEventId); ok {
		out.ParentEventId = v
	}
	_pathEventId := []string{"event_id"}
	if v, ok := into.GetInt32(x, _pathEventId); ok {
		out.EventId = v
	}
	_pathSomeEventStringPayload := []string{"some_event_string_payload"}
	if v, ok := into.GetString(x, _pathSomeEventStringPayload); ok {
		out.SomeEventStringPayload = v
	}
	_pathProcess := []string{"process"}
	if v, ok := into.GetMessage(x, _pathProcess); ok {
		if mv, ok := v.(*Process); ok {
			out.Process = mv.IntoPlain()
		}
	}
	_pathPathCrf := []string{"data", "pathCRF"}
	if v, ok := into.GetString(x, _pathPathCrf); ok {
		out.PathCRF = v
	}
	return out
}

func (x *EventDataPlain) IntoPb() *EventData {
	if x == nil {
		return nil
	}
	out := &EventData{}
	_pathPath := []string{"file_rename", "file", "path"}
	if x.PathCRF != "" {
		_pathPath = into.ParseCRFPath(x.PathCRF)
	}
	if x.Path != nil {
		into.SetString(out, _pathPath, *x.Path)
	}
	_pathPathCrf := []string{"pathCRF"}
	into.SetString(out, _pathPathCrf, x.PathCRF)
	_pathOtherEvent := []string{"other_event"}
	if x.OtherEvent != nil {
		into.SetString(out, _pathOtherEvent, *x.OtherEvent)
	}
	_pathNonPlatformEventCustomEvent := []string{"custom_event"}
	if x.NonPlatformEventCustomEvent != nil {
		into.SetString(out, _pathNonPlatformEventCustomEvent, *x.NonPlatformEventCustomEvent)
	}
	_pathNonPlatformEventPath := []string{"file", "path"}
	into.SetString(out, _pathNonPlatformEventPath, x.NonPlatformEventPath)
	out.NoRemovedOneof = x.NoRemovedOneof
	return out
}

func (x *EventData) IntoPlain() *EventDataPlain {
	if x == nil {
		return nil
	}
	out := &EventDataPlain{}
	_pathPath := []string{"file_rename", "file", "path"}
	if v, ok := into.GetString(x, _pathPath); ok {
		out.Path = &v
		out.PathCRF = strings.Join(_pathPath, "/")
	}
	_pathPathCrf := []string{"pathCRF"}
	if v, ok := into.GetString(x, _pathPathCrf); ok {
		out.PathCRF = v
	}
	_pathOtherEvent := []string{"other_event"}
	if v, ok := into.GetString(x, _pathOtherEvent); ok {
		out.OtherEvent = &v
	}
	_pathNonPlatformEventCustomEvent := []string{"custom_event"}
	if v, ok := into.GetString(x, _pathNonPlatformEventCustomEvent); ok {
		out.NonPlatformEventCustomEvent = &v
	}
	_pathNonPlatformEventPath := []string{"file", "path"}
	if v, ok := into.GetString(x, _pathNonPlatformEventPath); ok {
		out.NonPlatformEventPath = v
	}
	out.NoRemovedOneof = x.NoRemovedOneof
	return out
}

func (x *FilePlain) IntoPb() *File {
	if x == nil {
		return nil
	}
	out := &File{}
	_pathPath := []string{"path"}
	into.SetString(out, _pathPath, x.Path)
	return out
}

func (x *File) IntoPlain() *FilePlain {
	if x == nil {
		return nil
	}
	out := &FilePlain{}
	_pathPath := []string{"path"}
	if v, ok := into.GetString(x, _pathPath); ok {
		out.Path = v
	}
	return out
}

func (x *FileCreatePlain) IntoPb() *FileCreate {
	if x == nil {
		return nil
	}
	out := &FileCreate{}
	_pathPath := []string{"file", "path"}
	into.SetString(out, _pathPath, x.Path)
	return out
}

func (x *FileCreate) IntoPlain() *FileCreatePlain {
	if x == nil {
		return nil
	}
	out := &FileCreatePlain{}
	_pathPath := []string{"file", "path"}
	if v, ok := into.GetString(x, _pathPath); ok {
		out.Path = v
	}
	return out
}

func (x *FileRenamePlain) IntoPb() *FileRename {
	if x == nil {
		return nil
	}
	out := &FileRename{}
	_pathPath := []string{"file", "path"}
	into.SetString(out, _pathPath, x.Path)
	return out
}

func (x *FileRename) IntoPlain() *FileRenamePlain {
	if x == nil {
		return nil
	}
	out := &FileRenamePlain{}
	_pathPath := []string{"file", "path"}
	if v, ok := into.GetString(x, _pathPath); ok {
		out.Path = v
	}
	return out
}

func (x *ProcessPlain) IntoPb() *Process {
	if x == nil {
		return nil
	}
	out := &Process{}
	_pathFile := []string{"file"}
	if x.File != nil {
		into.SetMessage(out, _pathFile, x.File.IntoPb())
	}
	return out
}

func (x *Process) IntoPlain() *ProcessPlain {
	if x == nil {
		return nil
	}
	out := &ProcessPlain{}
	_pathFile := []string{"file"}
	if v, ok := into.GetMessage(x, _pathFile); ok {
		if mv, ok := v.(*File); ok {
			out.File = mv.IntoPlain()
		}
	}
	return out
}
