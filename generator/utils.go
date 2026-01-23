package generator

func stringOrDefault(s string, def string) string {
	if s == "" {
		return def
	}
	return s
}
