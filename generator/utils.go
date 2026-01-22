package generator

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/typepb"
)

func stringOrDefault(s string, def string) string {
	if s == "" {
		return def
	}
	return s
}

func removeOneoffs(msg *typepb.Type, names []string) {
	oldOneofs := msg.Oneofs
	newOneofs := lo.Filter(oldOneofs, func(oneof string, _ int) bool {
		return !lo.Contains(names, oneof)
	})
	msg.Oneofs = newOneofs
}

func pickOneFromMarkers(maps map[string]string, keys ...string) map[string]string {
	for ks := range maps {
		for _, key := range keys {
			if value, ok := maps[ks]; ok {
				return map[string]string{key: value}
			}
		}
	}
	return nil
}
