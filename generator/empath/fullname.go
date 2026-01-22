package empath

import "strings"

const fullPathSeparator = "."
const nameSeparator = "+"

type Name string

func NewName(s string) Name {
	return Name(s)
}

func (n Name) AddPrefix(prefix string) Name {
	return NewName(strings.Join([]string{prefix, string(n)}, nameSeparator))
}

func (n Name) AddSuffix(suffix string) Name {
	return NewName(strings.Join([]string{string(n), suffix}, nameSeparator))
}

func (n Name) String() string {
	return string(n)
}

type FullName []Name

func ParseFullName(s string) FullName {
	split := strings.Split(s, fullPathSeparator)
	result := make(FullName, len(split))
	for i, part := range split {
		result[i] = NewName(part)
	}
	return result
}

func (fn FullName) String() string {
	result := make([]string, len(fn))
	for i, name := range fn {
		result[i] = string(name)
	}
	return strings.Join(result, fullPathSeparator)
}
