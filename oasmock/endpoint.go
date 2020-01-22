package oasmock

import (
	"github.com/pkg/errors"
	"github.com/tjbrockmeyer/oas"
	"github.com/tjbrockmeyer/oasm"
	"net/http"
	"reflect"
)

type endpoint struct {
	doc      oasm.Operation
	function oas.HandlerFunc
}

func (e *endpoint) Option(string, interface{}) oas.EndpointDeclaration {
	return e
}

func (e *endpoint) Version(int) oas.EndpointDeclaration {
	return e
}

func (e *endpoint) Parameter(in string, name string, description string, required bool, schema interface{}, kind reflect.Kind) oas.EndpointDeclaration {
	return e
}

func (e *endpoint) RequestBody(description string, required bool, schema interface{}, object interface{}) oas.EndpointDeclaration {
	return e
}

func (e *endpoint) Response(code int, description string, schema interface{}) oas.EndpointDeclaration {
	return e
}

func (e *endpoint) Deprecate(comment string) oas.EndpointDeclaration {
	return e
}

func (e *endpoint) Security(name string, scopes []string) oas.EndpointDeclaration {
	return e
}

func (e *endpoint) Define(f oas.HandlerFunc) (oas.Endpoint, error) {
	e.function = f
	return e, nil
}

func (e *endpoint) MustDefine(f oas.HandlerFunc) oas.Endpoint {
	_, _ = e.Define(f)
	return e
}

func (e *endpoint) Doc() *oasm.Operation {
	return &e.doc
}

func (e *endpoint) Options() map[string]interface{} {
	return map[string]interface{}{}
}

func (e *endpoint) Settings() (method, path string, version int) {
	return "", "", 0
}

func (e *endpoint) SecurityMapping() map[*oasm.SecurityRequirement]oasm.SecurityScheme {
	return map[*oasm.SecurityRequirement]oasm.SecurityScheme{}
}

func (e *endpoint) UserDefinedFunc(oas.Data) (interface{}, error) {
	if e.function == nil {
		return nil, errors.New("endpoint has not been Define()d: " + e.doc.OperationId)
	}
}

func (e *endpoint) Call(w http.ResponseWriter, r *http.Request) {
	panic("implement me")
}
