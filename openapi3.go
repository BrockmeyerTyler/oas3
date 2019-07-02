package oas3

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oas3models"
	"io/ioutil"
	"net/http"
)

type OpenAPI3 oas3models.OpenAPIDoc

// Create a new specification for your API
// This will create the endpoints in the documentation and will create routes for them.
func NewOpenAPISpec3(title, description, version string, endpoints []*Endpoint, apiSubRouter *mux.Router) *OpenAPI3 {
	spec := &OpenAPI3{
		OpenApi: "3.0.0",
		Info: &oas3models.InfoDoc{
			Title:       title,
			Description: description,
			Version:     version,
		},
		Servers:    make([]*oas3models.ServerDoc, 0, 1),
		Tags:       make([]*oas3models.TagDoc, 0, 3),
		Paths:      make(oas3models.PathsDoc),
		Components: &oas3models.ComponentsDoc{},
	}
	for _, e := range endpoints {
		pathItem, ok := spec.Paths[e.settings.path]
		if !ok {
			pathItem = &oas3models.PathItemDoc{
				Methods: make(map[oas3models.HTTPVerb]*oas3models.OperationDoc)}
			spec.Paths[e.settings.path] = pathItem
		}
		pathItem.Methods[oas3models.HTTPVerb(e.settings.method)] = e.Doc
		apiSubRouter.Path(e.settings.path).Methods(string(e.settings.method)).HandlerFunc(e.run)
	}
	return spec
}

// Add a server to the API
func (o *OpenAPI3) Server(url, description string) *OpenAPI3 {
	o.Servers = append(o.Servers, &oas3models.ServerDoc{
		Url:         url,
		Description: description,
	})
	return o
}

// Add a tag to the API with a description
func (o *OpenAPI3) Tag(name, description string) *OpenAPI3 {
	o.Tags = append(o.Tags, &oas3models.TagDoc{
		Name:        name,
		Description: description,
	})
	return o
}

// Add global security requirements for the API
func (o *OpenAPI3) SecurityRequirement(name string, scopes ...string) *OpenAPI3 {
	o.Security = append(o.Security, &oas3models.SecurityRequirementDoc{
		Name:   name,
		Scopes: scopes,
	})
	return o
}

// Create a new security scheme of API key
func (o *OpenAPI3) NewAPIKey(name, description, headerName string) *OpenAPI3 {
	if o.Components.SecuritySchemes == nil {
		o.Components.SecuritySchemes = make(map[string]*oas3models.SecuritySchemeDoc)
	}
	o.Components.SecuritySchemes[name] = &oas3models.SecuritySchemeDoc{
		Type: "apiKey",
		In:   "header",
		Name: headerName,
	}
	return o
}

// Create a new security scheme of clientCredentials.
func (o *OpenAPI3) NewClientCredentialsOAuth(
	name, description, tokenUrl, refreshUrl string,
	scopes map[string]string) *OpenAPI3 {
	if o.Components.SecuritySchemes == nil {
		o.Components.SecuritySchemes = make(map[string]*oas3models.SecuritySchemeDoc)
	}
	o.Components.SecuritySchemes[name] = &oas3models.SecuritySchemeDoc{
		Type: "oauth2",
		Flows: map[oas3models.OAuthFlowType]*oas3models.OAuthFlowDoc{
			"clientCredentials": {
				TokenUrl:   tokenUrl,
				RefreshUrl: refreshUrl,
				Scopes:     scopes,
			},
		},
	}
	fmt.Println(o.Components.SecuritySchemes[name])
	return o
}

// Mount your local Swagger UI at the route specified by 'route'.
func (o *OpenAPI3) SwaggerDocs(route *mux.Route, projectSwaggerDir string) error {
	path, err := route.GetPathTemplate()
	if err != nil {
		return fmt.Errorf("failed to get path for provided Swagger route: %s", err.Error())
	}
	route.Handler(http.StripPrefix(path, http.FileServer(http.Dir(projectSwaggerDir))))

	if b, err := json.Marshal(o); err != nil {
		return fmt.Errorf("could not marshal Open API 3 spec: %s", err.Error())
	} else if err = ioutil.WriteFile(projectSwaggerDir+"/spec.json", b, 0644); err != nil {
		return fmt.Errorf("could not write Open API 3 spec to %s: %s", projectSwaggerDir, err.Error())
	}
	return nil
}

// Add a schema file to your documentation.
// The schema file should have a top-most "definitions" property,
// and the names contained within will be added directly into #/components/schemas.
// They will be prepended by 'prefix' before creation.
func (o *OpenAPI3) AddSchemaFile(filepath, prefix string) error {
	contents, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}

	var file map[string]interface{}
	err = json.Unmarshal(contents, &file)
	if err != nil {
		return err
	}

	defs, ok := file["definitions"]
	if !ok {
		return fmt.Errorf("schema files must contain a top-level 'definitions' property")
	}

	defsMap, ok := defs.(map[string]interface{})
	if !ok {
		return fmt.Errorf("'definitions' property of schema files must be an object")
	}

	if o.Components.Schemas == nil {
		o.Components.Schemas = make(map[string]interface{})
	}
	for name := range defsMap {
		o.Components.Schemas[name] = Ref(fmt.Sprintf("%s#/definitions/%s%s", filepath, prefix, name))
	}
	return nil
}
