package marker

import (
	"strings"

	"github.com/samber/lo"
)

const MarkerSeparator = "?"
const kvSeparator = "="
const valuesSeparator = ";"

type StringMarker struct {
	value   string
	markers map[string]string
}

func New(s string, mp map[string]string) StringMarker {
	return StringMarker{value: s, markers: mp}
}

func (sm StringMarker) Markers() map[string]string {
	return sm.markers
}

func (sm StringMarker) Value() string {
	return sm.value
}

func (sm StringMarker) HasMarker(key string) bool {
	_, ok := sm.markers[key]
	return ok
}

func (sm StringMarker) Copy() StringMarker {
	return StringMarker{
		value:   sm.value,
		markers: lo.Assign(map[string]string{}, sm.markers),
	}
}

func (sm StringMarker) ClearMarkers() StringMarker {
	sm.markers = make(map[string]string)
	return sm
}

func (sm StringMarker) Merge(other StringMarker) StringMarker {
	sm.value += other.Value()
	for k, v := range other.markers {
		sm.markers[k] = v
	}
	return sm
}

func (sm StringMarker) GetMarker(key string) string {
	return sm.markers[key]
}

func (sm StringMarker) RemoveMarker(key string) StringMarker {
	delete(sm.markers, key)
	return sm
}

func (sm StringMarker) AddMarker(key, value string) StringMarker {
	sm.markers[key] = value
	return sm
}

func (sm StringMarker) AddMarkers(mp map[string]string) StringMarker {
	for k, v := range mp {
		if k == "" || v == "" {
			continue
		}
		sm.markers[k] = v
	}
	return sm
}

func (sm StringMarker) String() string {
	if len(sm.markers) == 0 {
		return sm.Value()
	}
	var kvs []string
	for k, v := range sm.markers {
		kvs = append(kvs, k+"="+v)
	}
	return strings.Join([]string{sm.Value(), strings.Join(kvs, valuesSeparator)}, MarkerSeparator)
}

func Parse(s string) StringMarker {
	markers := make(map[string]string)
	parts := strings.Split(s, MarkerSeparator)
	if len(parts) == 0 {
		panic("empty string")
	}
	if len(parts) == 1 {
		return StringMarker{
			value:   parts[0],
			markers: markers,
		}
	}
	for _, maker := range strings.Split(parts[1], valuesSeparator) {
		pts := strings.Split(maker, kvSeparator)
		if len(pts) != 2 {
			continue
		}
		markers[pts[0]] = pts[1]
	}
	return StringMarker{
		value:   parts[0],
		markers: markers,
	}
}
