package empath

import (
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/generator/marker"
)

const empathSeparator = "/"

type EmPath []marker.StringMarker

func (s EmPath) String() string {
	strs := make([]string, len(s))
	for i, p := range s {
		strs[i] = p.String()
	}
	return strings.Join(strs, empathSeparator)
}

func New(m ...marker.StringMarker) EmPath {
	return m
}

func (s EmPath) Last() marker.StringMarker {
	return s[len(s)-1]
}

func (s EmPath) Copy() EmPath {
	result := make(EmPath, len(s))
	copy(result, s)
	return result
}

func (s EmPath) Append(prefix marker.StringMarker) EmPath {
	return append(s.Copy(), prefix)
}
func (s EmPath) Prepend(prefix marker.StringMarker) EmPath {
	return append([]marker.StringMarker{prefix}, s.Copy()...)
}
func (s EmPath) AppendPath(other EmPath) EmPath {
	return append(s.Copy(), other...)
}

func Parse(s string) EmPath {
	parts := strings.Split(s, empathSeparator)
	result := make(EmPath, len(parts))
	for i, part := range parts {
		result[i] = marker.Parse(part)
	}
	return result
}
