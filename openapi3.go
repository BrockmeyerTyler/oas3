package oas3

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oas3models"
	"io/ioutil"
	"net/http"
	"regexp"
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
	return o
}

// Add a schema file to your documentation.
// The schema file should have a top-most "definitions" property,
// and the names contained within will be added directly into #/components/schemas.
// They will be prepended by 'prefix' before creation.
//
// - specsDir should be the directory that will contain your generated spec.
func (o *OpenAPI3) AddSchemaFile(specsDir, filename, prefix string) error {
	contents, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", specsDir, filename))
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
		o.Components.Schemas[name] = Ref(fmt.Sprintf("%s#/definitions/%s%s", filename, prefix, name))
	}
	return nil
}

// Publish your API using a Swagger UI. Writes your spec to the specified file.
//
// - specPath should be the path to where your spec will be generated from within your public swagger directory.
//
// This method should be called LAST!
func (o *OpenAPI3) PublishSwaggerUI(route *mux.Route, publicSwaggerDir, specPath string) error {
	path, err := route.GetPathTemplate()
	if err != nil {
		return fmt.Errorf("failed to get path for provided Swagger route: %s", err.Error())
	}
	route.Handler(http.StripPrefix(path, http.FileServer(http.Dir(publicSwaggerDir))))

	if b, err := json.Marshal(o); err != nil {
		return fmt.Errorf("could not marshal Open API 3 spec: %s", err.Error())
	} else if err = ioutil.WriteFile(fmt.Sprintf("%s/%s", publicSwaggerDir, specPath), b, 0644); err != nil {
		return fmt.Errorf("could not write Open API 3 spec to %s: %s", publicSwaggerDir, err.Error())
	}

	indexHtml := fmt.Sprintf("%s/index.html", publicSwaggerDir)
	if contents, err := ioutil.ReadFile(indexHtml); err != nil {
		return fmt.Errorf("could not open 'index.html' in swagger directory: %s", err.Error())
	} else {
		regex, _ := regexp.Compile(`url: ?(".*?"|'.*?')`)
		newContents := regex.ReplaceAllLiteral(contents, []byte(fmt.Sprintf(`url: "./%s"`, specPath)))
		err := ioutil.WriteFile(indexHtml, newContents, 644)
		if err != nil {
			return err
		}
	}
	return nil
}
