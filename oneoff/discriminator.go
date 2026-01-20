package oneoff

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type EnumDiscriminator string

func (e EnumDiscriminator) String() string {
	return string(e)
}

func NewDiscriminator[T protoreflect.Enum](enumValue T) EnumDiscriminator {
	buf := &bytes.Buffer{}
	buf.WriteString(string(enumValue.Descriptor().FullName()))
	buf.WriteByte(':')
	buf.WriteString(strconv.Itoa(int(enumValue.Number())))
	return EnumDiscriminator(buf.String())
}

func ParseDiscriminator(discriminator EnumDiscriminator) (zero protoreflect.Enum, err error) {
	parts := strings.Split(string(discriminator), ":")
	if len(parts) != 2 {
		return zero, fmt.Errorf("invalid discriminator: %s", discriminator)
	}
	enumType, err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(parts[0]))
	if err != nil {
		if errors.Is(err, protoregistry.NotFound) {
			return zero, fmt.Errorf("unknown enum: %s", parts[0])
		}
		return zero, fmt.Errorf("invalid discriminator: %s", discriminator)
	}
	enumNumber, err := strconv.Atoi(parts[1])
	if err != nil {
		return zero, fmt.Errorf("invalid discriminator: %s", discriminator)
	}
	return enumType.New(protoreflect.EnumNumber(int32(enumNumber))), nil
}
