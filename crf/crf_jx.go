package crf

import (
	"github.com/go-faster/jx"
)

// MarshalJX сериализует CRF напрямую в encoder (без аллокаций)
func (c *CRF) MarshalJX(e *jx.Encoder) {
	if c == nil || len(c.Entries) == 0 {
		e.ObjStart()
		e.ObjEnd()
		return
	}
	e.ObjStart()
	e.FieldStart("entries")
	e.ArrStart()
	for i := range c.Entries {
		c.Entries[i].marshalJX(e)
	}
	e.ArrEnd()
	e.ObjEnd()
}

func (entry *Entry) marshalJX(e *jx.Encoder) {
	e.ObjStart()
	if entry.Field != "" {
		e.FieldStart("field")
		e.Str(entry.Field)
	}
	if len(entry.Sources) > 0 {
		e.FieldStart("sources")
		e.ArrStart()
		for i := range entry.Sources {
			entry.Sources[i].marshalJX(e)
		}
		e.ArrEnd()
	}
	e.ObjEnd()
}

func (s *Source) marshalJX(e *jx.Encoder) {
	e.ObjStart()
	if s.Path != "" {
		e.FieldStart("path")
		e.Str(s.Path)
	}
	e.ObjEnd()
}

// UnmarshalJX десериализует CRF напрямую из decoder
func (c *CRF) UnmarshalJX(d *jx.Decoder) error {
	if d.Next() == jx.Null {
		return d.Null()
	}
	return d.Obj(func(d *jx.Decoder, key string) error {
		if key != "entries" {
			return d.Skip()
		}
		c.Entries = c.Entries[:0] // Переиспользуем slice если возможно
		return d.Arr(func(d *jx.Decoder) error {
			if d.Next() == jx.Null {
				_ = d.Null()
				c.Entries = append(c.Entries, Entry{})
				return nil
			}
			var entry Entry
			if err := entry.unmarshalJX(d); err != nil {
				return err
			}
			c.Entries = append(c.Entries, entry)
			return nil
		})
	})
}

func (e *Entry) unmarshalJX(d *jx.Decoder) error {
	return d.Obj(func(d *jx.Decoder, key string) error {
		switch key {
		case "field":
			val, err := d.Str()
			if err != nil {
				return err
			}
			e.Field = val
		case "sources":
			e.Sources = e.Sources[:0]
			return d.Arr(func(d *jx.Decoder) error {
				if d.Next() == jx.Null {
					_ = d.Null()
					e.Sources = append(e.Sources, Source{})
					return nil
				}
				var src Source
				if err := src.unmarshalJX(d); err != nil {
					return err
				}
				e.Sources = append(e.Sources, src)
				return nil
			})
		default:
			return d.Skip()
		}
		return nil
	})
}

func (s *Source) unmarshalJX(d *jx.Decoder) error {
	return d.Obj(func(d *jx.Decoder, key string) error {
		if key == "path" {
			val, err := d.Str()
			if err != nil {
				return err
			}
			s.Path = val
			return nil
		}
		return d.Skip()
	})
}
