package crf

import (
	"fmt"
	"strings"
)

type Source struct {
	Path string `json:"path"`
}

type Entry struct {
	Field   string   `json:"field"`
	Sources []Source `json:"sources"`
}

type CRF struct {
	Entries []Entry `json:"entries"`
}

func (c *CRF) HasEntries() bool {
	return c != nil && len(c.Entries) > 0
}

func (c *CRF) Error(field string) error {
	if c == nil || field == "" {
		return nil
	}
	for _, entry := range c.Entries {
		if entry.Field == field {
			return ErrFieldCollision{Field: field, Sources: entry.sourcePaths()}
		}
	}
	return nil
}

func (e Entry) sourcePaths() []string {
	if len(e.Sources) == 0 {
		return nil
	}
	out := make([]string, 0, len(e.Sources))
	for _, src := range e.Sources {
		if src.Path == "" {
			continue
		}
		out = append(out, src.Path)
	}
	return out
}

type ErrFieldCollision struct {
	Field   string
	Sources []string
}

func (e ErrFieldCollision) Error() string {
	if len(e.Sources) == 0 {
		return fmt.Sprintf("crf collision for %s", e.Field)
	}
	return fmt.Sprintf("crf collision for %s: %s", e.Field, strings.Join(e.Sources, ", "))
}
