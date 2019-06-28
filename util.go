package oas3

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func Ref(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"$ref": %s}`, strconv.Quote(to)))
}

func errorToJSON(err error) json.RawMessage {
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":%s}`, strconv.Quote(err.Error())))
}
