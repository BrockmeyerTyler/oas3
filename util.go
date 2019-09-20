package oas

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Data struct {
	Req       *http.Request
	ResWriter http.ResponseWriter
	Query     url.Values
	Params    map[string]string
	Body      interface{}
}

type Response struct {
	// If set, the Response will be ignored.
	// Set ONLY when handling the response writer manually.
	Ignore  bool
	Status  int
	Body    interface{}
	Headers map[string]string
}

type ValidationError struct {
	errors []string
}

func (v *ValidationError) Add(str string) {
	if v.errors == nil {
		v.errors = make([]string, 1, 5)
		v.errors[0] = str
	} else {
		v.errors = append(v.errors, str)
	}
}

func (v ValidationError) HasErrors() bool {
	return v.errors != nil
}

func (v ValidationError) Error() string {
	return "a validation error has occurred:\n  " + strings.Join(v.errors, "\n  ")
}

// A reference object
func Ref(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"$ref":%s}`, strconv.Quote(to)))
}

// A reference to a schema in this document
func SchemaRef(to string) json.RawMessage {
	return Ref(fmt.Sprintf("#/components/schemas/%s", to))
}

func ArrayOfSchemaRef(to string) json.RawMessage {
	return []byte(fmt.Sprintf(`{"type":"array","items":%s}`, SchemaRef(to)))
}

// A reference to any component in this document
func CompRef(to string) json.RawMessage {
	return Ref(fmt.Sprintf("#/components/%s", to))
}

func errorToJSON(err error) json.RawMessage {
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":%s}`, strconv.Quote(err.Error())))
}
