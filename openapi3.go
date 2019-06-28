package oas3

import (
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oas3models"
)

type OpenAPI3 struct {
	Doc       *oas3models.OpenAPIDoc
	ApiRouter *mux.Router
}

// Create a new specification for your API
// This will create the endpoints in the documentation and will create a router for them.
func NewOpenAPISpec3(title, description, version, basePath string, endpoints []*Endpoint) *OpenAPI3 {
	spec := &OpenAPI3{
		Doc: &oas3models.OpenAPIDoc{
			OpenApi: "3.0.0",
			Info: &oas3models.InfoDoc{
				Title:       title,
				Description: description,
				Version:     version,
			},
			Servers: make([]*oas3models.ServerDoc, 0, 1),
			Tags:    make([]*oas3models.TagDoc, 0, 3),
			Paths:   make(oas3models.PathsDoc),
		},
		ApiRouter: mux.NewRouter().PathPrefix(basePath).Subrouter().StrictSlash(true),
	}
	for _, e := range endpoints {
		pathItem, ok := spec.Doc.Paths[e.settings.path]
		if !ok {
			pathItem = &oas3models.PathItemDoc{
				Methods: make(map[oas3models.HTTPVerb]*oas3models.OperationDoc)}
			spec.Doc.Paths[e.settings.path] = pathItem
		}
		pathItem.Methods[oas3models.HTTPVerb(e.settings.method)] = e.Doc
		spec.ApiRouter.Path(e.settings.path).Methods(string(e.settings.method)).HandlerFunc(e.run)
	}
	return spec
}

// Add a server to the API
func (o *OpenAPI3) Server(url, description string) *OpenAPI3 {
	o.Doc.Servers = append(o.Doc.Servers, &oas3models.ServerDoc{
		Url:         url,
		Description: description,
	})
	return o
}

// Add a tag to the API with a description
func (o *OpenAPI3) Tag(name, description string) *OpenAPI3 {
	o.Doc.Tags = append(o.Doc.Tags, &oas3models.TagDoc{
		Name:        name,
		Description: description,
	})
	return o
}

// Add global security requirements for the API
func (o *OpenAPI3) Security(name string, scopes ...string) *OpenAPI3 {
	o.Doc.Security = append(o.Doc.Security, &oas3models.SecurityRequirementDoc{
		Name:   name,
		Scopes: scopes,
	})
	return o
}
