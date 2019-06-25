package oas3

import (
	"github.com/brockmeyertyler/oas3models"
	"github.com/gorilla/mux"
)

type OpenAPI3 struct {
	Doc *oas3models.OpenAPIDoc
	DocRouter *mux.Router
	ApiRouter *mux.Router
}

func NewOpenAPISpec3(title, description, version string, endpoints []*Endpoint) *OpenAPI3 {
	paths := make(oas3models.PathsDoc)
	for _, e := range endpoints {
		pathItem, ok := paths[e.settings.path]
		if !ok {
			pathItem = new(oas3models.PathItemDoc)
			paths[e.settings.path] = pathItem
		}
		pathItem.Methods[e.settings.method] = e.Doc
	}
	return &OpenAPI3{
		Doc: &oas3models.OpenAPIDoc{
			OpenApi: "3.0.0",
			Info: &oas3models.InfoDoc{
				Title: title,
				Description: description,
				Version: version,
			},
			Servers: make([]*oas3models.ServerDoc, 1),
			Tags: make([]*oas3models.TagDoc, 3),
		},
	}
}

func (o *OpenAPI3) Server(url, description string) *OpenAPI3 {
	o.Doc.Servers = append(o.Doc.Servers, &oas3models.ServerDoc{
		Url: url,
		Description: description,
	})
	return o
}

func (o *OpenAPI3) Tag(name, description string) *OpenAPI3 {
	o.Doc.Tags = append(o.Doc.Tags, &oas3models.TagDoc{
		Name: name,
		Description: description,
	})
	return o
}

func (o *OpenAPI3) Security(name string, scopes ...string) *OpenAPI3 {
	o.Doc.Security = append(o.Doc.Security, &oas3models.SecurityRequirementDoc{
		Name: name,
		Scopes: scopes,
	})
	return o
}
