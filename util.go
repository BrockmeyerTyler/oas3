package oas3

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// A reference object
func Ref(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"$ref": %s}`, strconv.Quote(to)))
}

// A reference to a schema in this document
func SchemaRef(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"$ref": #/components/schemas/%s}`, strconv.Quote(to)))
}

// A reference to any component in this document
func CompRef(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"$ref": #/components/%s`, strconv.Quote(to)))
}

func errorToJSON(err error) json.RawMessage {
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":%s}`, strconv.Quote(err.Error())))
}
