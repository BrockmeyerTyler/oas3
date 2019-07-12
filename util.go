package oas

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// A reference object
func Ref(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"$ref":%s}`, strconv.Quote(to)))
}

// A reference to a schema in this document
func SchemaRef(to string) json.RawMessage {
	return Ref(fmt.Sprintf("#/components/schemas/%s", to))
}

// A reference to any component in this document
func CompRef(to string) json.RawMessage {
	return Ref(fmt.Sprintf("#/components/%s", to))
}

func errorToJSON(err error) json.RawMessage {
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":%s}`, strconv.Quote(err.Error())))
}

func fileExists(err error) bool {
	return strings.Contains(err.Error(), "file exists")
}
