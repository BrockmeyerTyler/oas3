package oas

import (
	"encoding/json"
	"fmt"
	copy2 "github.com/otiai10/copy"
	"github.com/tjbrockmeyer/oasm"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
)

type OpenAPI struct {
	Doc *oasm.OpenAPIDoc
	dir string
}

var specDir = "spec"
var specPath = fmt.Sprintf("%s/openapi.json", specDir)

// Create a new specification for your API
// This will create the endpoints in the documentation and will create routes for them.
//
// dir - A directory for hosting the spec, schemas, and SwaggerUI.
func NewOpenAPI(title, description, version, dir string) *OpenAPI {
	if err := os.MkdirAll(path.Join(dir, specDir), os.ModePerm); err != nil && !fileExists(err) {
		log.Printf("failed to create spec directory: %v\n", err)
	}
	return &OpenAPI{
		Doc: &oasm.OpenAPIDoc{
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
		},
		dir: dir,
	}
}

// Add an amount of endpoints to the API.
func (o *OpenAPI) Endpoints(routeCreator func(method, path string, handler http.HandlerFunc), endpoints ...*Endpoint) *OpenAPI {
	for _, e := range endpoints {
		pathItem, ok := o.Doc.Paths[e.Settings.Path]
		if !ok {
			pathItem = &oasm.PathItemDoc{
				Methods: make(map[oasm.HTTPVerb]*oasm.OperationDoc)}
			o.Doc.Paths[e.Settings.Path] = pathItem
		}
		pathItem.Methods[oasm.HTTPVerb(e.Settings.Method)] = e.Doc
		routeCreator(e.Settings.Method, e.Settings.Path, e.Run)
	}
	return o
}

// Add a server to the API
func (o *OpenAPI) Server(url, description string) *OpenAPI {
	o.Doc.Servers = append(o.Doc.Servers, &oasm.ServerDoc{
		Url:         url,
		Description: description,
	})
	return o
}

// Add a tag to the API with a description
func (o *OpenAPI) Tag(name, description string) *OpenAPI {
	o.Doc.Tags = append(o.Doc.Tags, &oasm.TagDoc{
		Name:        name,
		Description: description,
	})
	return o
}

// Add global security requirements for the API
func (o *OpenAPI) SecurityRequirement(name string, scopes ...string) *OpenAPI {
	o.Doc.Security = append(o.Doc.Security, &oasm.SecurityRequirementDoc{
		Name:   name,
		Scopes: scopes,
	})
	return o
}

// Create a new security scheme of API key
func (o *OpenAPI) NewAPIKey(name, description, headerName string) *OpenAPI {
	if o.Doc.Components.SecuritySchemes == nil {
		o.Doc.Components.SecuritySchemes = make(map[string]*oasm.SecuritySchemeDoc)
	}
	o.Doc.Components.SecuritySchemes[name] = &oasm.SecuritySchemeDoc{
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
	if o.Doc.Components.SecuritySchemes == nil {
		o.Doc.Components.SecuritySchemes = make(map[string]*oasm.SecuritySchemeDoc)
	}
	o.Doc.Components.SecuritySchemes[name] = &oasm.SecuritySchemeDoc{
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
func (o *OpenAPI) AddSchemaFile(schemaFilepath, prefix string) error {
	schemaFilename := filepath.Base(schemaFilepath)
	contents, err := ioutil.ReadFile(schemaFilepath)
	if err != nil {
		return fmt.Errorf("failed to read the schema file: %v", err)
	}
	var file map[string]interface{}
	err = json.Unmarshal(contents, &file)
	if err != nil {
		return fmt.Errorf("failed to unmarshal the schema file: %v", err)
	}
	defs, ok := file["definitions"]
	if !ok {
		return fmt.Errorf("schema files must contain a top-level 'definitions' property")
	}
	defsMap, ok := defs.(map[string]interface{})
	if !ok {
		return fmt.Errorf("'definitions' property of schema files must be an object")
	}
	if o.Doc.Components.Schemas == nil {
		o.Doc.Components.Schemas = make(map[string]interface{})
	}
	for name := range defsMap {
		o.Doc.Components.Schemas[name] = Ref(fmt.Sprintf("%s#/definitions/%s%s", schemaFilename, prefix, name))
	}
	schemaCopy := fmt.Sprintf("%s/%s/%s", o.dir, specDir, schemaFilename)
	_ = os.Remove(schemaCopy)
	if err = os.Link(schemaFilepath, schemaCopy); err != nil {
		return fmt.Errorf("failed to create a link to the schema file: %v", err)
	}
	return nil
}

// Publish your API using a Swagger UI. Writes your spec to the specified file.
// Returns a File Server handler. This should be mounted with a call to http.StripPrefix()
func (o *OpenAPI) CreateSwaggerUI() (fileServer http.Handler, err error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return nil, err
	}
	err = copy2.Copy(path.Join(path.Dir(file), "swagger-dist"), o.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to copy swagger ui distribution: %v", err)
	}
	indexHtml := fmt.Sprintf("%s/index.html", o.dir)
	if contents, err := ioutil.ReadFile(indexHtml); err != nil {
		return nil, fmt.Errorf("could not open 'index.html' in swagger directory: %s", err.Error())
	} else {
		regex, _ := regexp.Compile(`url: ?(".*?"|'.*?')`)
		newContents := regex.ReplaceAllLiteral(contents, []byte(fmt.Sprintf(`url: "./%s"`, specPath)))
		err := ioutil.WriteFile(indexHtml, newContents, 644)
		if err != nil {
			return nil, err
		}
	}
	return http.FileServer(http.Dir(o.dir)), nil
}

// Save the spec into the directory
func (o *OpenAPI) Save() error {
	if b, err := json.Marshal(o.Doc); err != nil {
		return fmt.Errorf("could not marshal Open API 3 spec: %s", err.Error())
	} else if err = ioutil.WriteFile(path.Join(o.dir, specPath), b, 0644); err != nil {
		return fmt.Errorf("could not write Open API 3 spec to %s: %s", o.dir, err.Error())
	}
	return nil
}
