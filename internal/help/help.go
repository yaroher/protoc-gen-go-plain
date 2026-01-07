package help

import (
	"strings"

	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func GetMessageByFullName(p *protogen.Plugin, fullName string) *protogen.Message {
	for _, file := range p.Files {
		for _, message := range file.Messages {
			if string(message.Desc.FullName()) == fullName {
				return message
			}
		}
	}
	return nil
}

func LowerSnake(s protoreflect.Name) string {
	return strcase.ToSnake(strings.ToLower(string(s)))
}

func StringOrDefault(s string, d string) string {
	if s != "" {
		return s
	}
	return d
}

func ListStringOrDefault(s []string, d []string) []string {
	if len(s) > 0 {
		return s
	}
	return d
}
