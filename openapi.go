package oas

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oasm"
	"io/ioutil"
	"net/http"
	"regexp"
)

type OpenAPI oasm.OpenAPIDoc

// Create a new specification for your API
// This will create the endpoints in the documentation and will create routes for them.
func NewOpenAPI(title, description, version string) *OpenAPI {
	return &OpenAPI{
		OpenApi: "3.0.0",
		Info: &oasm.InfoDoc{
			Title:       title,
			Description: description,
			Version:     version,
		},
		Servers:    make([]*oasm.ServerDoc, 0, 1),
		Tags:       make([]*oasm.TagDoc, 0, 3),
		Paths:      make(oasm.PathsDoc),
		Components: &oasm.ComponentsDoc{},
	}
}

func (o *OpenAPI) Endpoints(apiSubRouter *mux.Router, endpoints ...*Endpoint) *OpenAPI {
	for _, e := range endpoints {
		pathItem, ok := o.Paths[e.Settings.Path]
		if !ok {
			pathItem = &oasm.PathItemDoc{
				Methods: make(map[oasm.HTTPVerb]*oasm.OperationDoc)}
			o.Paths[e.Settings.Path] = pathItem
		}
		pathItem.Methods[oasm.HTTPVerb(e.Settings.Method)] = e.Doc
		apiSubRouter.Path(e.Settings.Path).Methods(string(e.Settings.Method)).HandlerFunc(e.Run)
	}
	return o
}

// Add a server to the API
func (o *OpenAPI) Server(url, description string) *OpenAPI {
	o.Servers = append(o.Servers, &oasm.ServerDoc{
		Url:         url,
		Description: description,
	})
	return o
}

// Add a tag to the API with a description
func (o *OpenAPI) Tag(name, description string) *OpenAPI {
	o.Tags = append(o.Tags, &oasm.TagDoc{
		Name:        name,
		Description: description,
	})
	return o
}

// Add global security requirements for the API
func (o *OpenAPI) SecurityRequirement(name string, scopes ...string) *OpenAPI {
	o.Security = append(o.Security, &oasm.SecurityRequirementDoc{
		Name:   name,
		Scopes: scopes,
	})
	return o
}

// Create a new security scheme of API key
func (o *OpenAPI) NewAPIKey(name, description, headerName string) *OpenAPI {
	if o.Components.SecuritySchemes == nil {
		o.Components.SecuritySchemes = make(map[string]*oasm.SecuritySchemeDoc)
	}
	o.Components.SecuritySchemes[name] = &oasm.SecuritySchemeDoc{
		Type: "apiKey",
		In:   "header",
		Name: headerName,
	}
	return o
}

// Create a new security scheme of clientCredentials.
func (o *OpenAPI) NewClientCredentialsOAuth(
	name, description, tokenUrl, refreshUrl string,
	scopes map[string]string) *OpenAPI {
	if o.Components.SecuritySchemes == nil {
		o.Components.SecuritySchemes = make(map[string]*oasm.SecuritySchemeDoc)
	}
	o.Components.SecuritySchemes[name] = &oasm.SecuritySchemeDoc{
		Type: "oauth2",
		Flows: map[oasm.OAuthFlowType]*oasm.OAuthFlowDoc{
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
func (o *OpenAPI) AddSchemaFile(specsDir, filename, prefix string) error {
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
func (o *OpenAPI) PublishSwaggerUI(route *mux.Route, publicSwaggerDir, specPath string) error {
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
