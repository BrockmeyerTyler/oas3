package oas3

import (
	"fmt"
	"encoding/json"
)

func Ref(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"$ref": "%s"}`, to))
}

func errorToJSON(err error) []byte{
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":"%s"}`, err.Error()))
}
