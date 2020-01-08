package oas

import (
	"encoding/json"
	"fmt"
	"github.com/tjbrockmeyer/oasm"
	"net/http"
	"reflect"
)

// Use to create a reference to a defined schema.
type Ref string

func (r Ref) toSwaggerSchema() json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"$ref":"#/components/schemas/%s"}`, r))
}

func (r Ref) toJSONSchema(schemasDir string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"$ref":"file://%s/%s.json"}`, schemasDir, r))
}

type typedParameter struct {
	kind reflect.Kind
	oasm.Parameter
}

type Data struct {
	// The HTTP Request that called this endpoint.
	Req *http.Request
	// The HTTP Response Writer.
	// When using, be sure to:
	//   return Response{Ignore:true}.
	ResWriter http.ResponseWriter
	// The query parameters passed in the url which are defined in the documentation for this endpoint.
	Query map[string]interface{}
	// The path parameters passed in the url which are defined in the documentation for this endpoint.
	Params map[string]interface{}
	// The headers passed in the request which are defined in the documentation for this endpoint.
	Headers map[string]interface{}
	// The request body, marshaled into the type of object which was set up on this endpoint during initialization.
	Body interface{}
	// The endpoint which was called.
	Endpoint *Endpoint
	// A place to attach any kind of data using middleware.
	Extra map[string]interface{}
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
