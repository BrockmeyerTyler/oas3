package oasmock

import (
	"github.com/tjbrockmeyer/oas"
	"github.com/tjbrockmeyer/oasm"
	"strings"
)

type openAPI struct {
	doc       oasm.OpenAPIDoc
	endpoints map[string]oas.Endpoint
}

func (o *openAPI) Doc() *oasm.OpenAPIDoc {
	return &o.doc
}

func (o *openAPI) SetDefaultJSONIndent(int) {}

func (o *openAPI) DefaultJSONIndent() int {
	return 0
}

func (o *openAPI) SetResponseAndErrorHandler(oas.ResponseAndErrorHandler) {}

func (o *openAPI) NewEndpoint(operationId, method, path, summary, description string, tags []string) oas.EndpointDeclaration {
	e := &endpoint{
		doc: oasm.Operation{
			Tags:        tags,
			Summary:     summary,
			Description: strings.ReplaceAll(description, "\n", "<br/>"),
			OperationId: operationId,
			Parameters:  make([]oasm.Parameter, 0, 2),
			Responses: oasm.Responses{
				Codes: make(map[int]oasm.Response),
			},
			Security: make([]oasm.SecurityRequirement, 0, 1),
		},
	}
	o.endpoints[operationId] = e
	return e
}

func (o *openAPI) Endpoints() map[string]oas.Endpoint {
	return o.endpoints
}

func (o *openAPI) Save() error {
	return nil
}

func NewOpenAPI() oas.OpenAPI {
	return &openAPI{
		doc: oasm.OpenAPIDoc{
			OpenApi: "3.0.0",
			Info: &oasm.Info{
				Title:       "test",
				Description: "description",
				Version:     "version",
			},
			Paths:      make(oasm.PathsMap),
			Components: oasm.Components{},
		},
		endpoints: make(map[string]oas.Endpoint),
	}
}
