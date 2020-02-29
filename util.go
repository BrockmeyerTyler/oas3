package oas

import (
	"context"
	"net/http"
)

const (
	JSONIndentHeader = "Oas-Json-Indent"
)

// Utility function for creating reference schemas.
func Ref(ref string) interface{} {
	return struct {
		Ref string `json:"$ref"`
	}{
		Ref: ref,
	}
}

// Utility function for creating basic array schemas that use the itemsSchema for all items.
func ArrayOf(itemsSchema interface{}) interface{} {
	return struct {
		Type  string      `json:"type"`
		Items interface{} `json:"items"`
	}{
		Type:  "array",
		Items: itemsSchema,
	}
}

// Adds the endpoint into the request context for other middleware to consume.
func EndpointAttachingMiddleware(endpoint Endpoint) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), "endpoint", endpoint))
			next.ServeHTTP(w, r)
		})
	}
}

func refNameToSwaggerRef(ref string) string {
	return "#/components/schemas/" + ref
}

func NewData(w http.ResponseWriter, r *http.Request, e Endpoint) Data {
	return Data{
		Req:       r,
		ResWriter: w,
		Query:     make(MapAny),
		Params:    make(MapAny),
		Headers:   make(MapAny),
		Endpoint:  e,
	}
}

type Data struct {
	// The HTTP Request that called this endpoint.
	Req *http.Request
	// The HTTP Response Writer.
	// When using, be sure to:
	//   return Response{Ignore:true}.
	ResWriter http.ResponseWriter
	// The query parameters passed in the url which are defined in the documentation for this endpoint.
	Query MapAny
	// The path parameters passed in the url which are defined in the documentation for this endpoint.
	Params MapAny
	// The headers passed in the request which are defined in the documentation for this endpoint.
	Headers MapAny
	// The request body, marshaled into the type of object which was set up on this endpoint during initialization.
	Body interface{}
	// The endpoint which was called.
	Endpoint Endpoint
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

type MapAny map[string]interface{}

func (m MapAny) GetOrElse(key string, elseValue interface{}) interface{} {
	if item, ok := m[key]; ok {
		return item
	}
	return elseValue
}
