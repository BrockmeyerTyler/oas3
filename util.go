package oas

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Data struct {
	// The HTTP Request that called this endpoint.
	Req *http.Request
	// The HTTP Response Writer.
	// When using, be sure to:
	//   return Response{Ignore:true}.
	ResWriter http.ResponseWriter
	// The query parameters passed in the url which are defined in the documentation for this endpoint.
	Query map[string]string
	// The path parameters passed in the url which are defined in the documentation for this endpoint.
	Params map[string]string
	// The headers passed in the request which are defined in the documentation for this endpoint.
	Headers map[string]string
	// The request body, marshaled into the type of object which was set up on this endpoint during initialization.
	Body interface{}
}

type Response struct {
	// If set, the Response will be ignored.
	// Set ONLY when handling the response writer manually.
	Ignore bool
	// The status code of the response.
	Status int
	// The body to send back in the response. If nil, no body will be sent.
	Body interface{}
	// The headers to send back in the response.
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
