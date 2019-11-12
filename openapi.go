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
	"regexp"
	"runtime"
)

var specPath = "openapi.json"

type HandlerFunc func(Data) (Response, error)
type Middleware func(next HandlerFunc) HandlerFunc
type ResponseHandler func(Data, Response, error) Response

type OpenAPI struct {
	Doc oasm.OpenAPIDoc
	// Indent level of JSON responses. (Default: 2) A level of 0 will print condensed JSON.
	JSONIndent      int
	responseHandler ResponseHandler
	dir             string
}

// Create a new OpenAPI Specification with JSON Schemas and a Swagger UI.
//
// This will:
//   - Generate an OpenAPI Specification for the API inside the provided directory.
//   - Create documentation and routes (via param:routeCreator) for all endpoints passed as arguments.
//   - Add all definitions from a provided JSON Schema file into the generated spec.
//   - Generate a Swagger UI in the target directory, returning a handler which can be used to mount the file server.
//   - Add middleware for authorization or other needs to every endpoint, with the endpoint itself as context.
//   - Add response handling middleware to every endpoint, for logging or other needs, after the endpoint has run.
//
// Parameters:
//   title           - API title
//   description     - API description
//   url             - API URL location
//   version         - API version in the format of (MAJOR.MINOR.PATCH)
//   dir             - A directory for hosting the spec, schemas, and SwaggerUI - typically a folder like ./public
//   schemaFilepath  - A filepath to a valid JSON Schema which contains `definitions` for objects to be used by the API
//   tags            - A list of Tag objects for describing the sections of the API which hold endpoints
//   endpoints       - A list of Endpoints that have been created for this API
//   routeCreator    - A function which can mount an endpoint at an http path
//   middleware      - A list of EndpointMiddleware functions to be run before each endpoint when they are called
//                     * Useful for authorization, header pre-processing, and more.
//                     * Use `nil` or an empty list to have no middleware.
//   responseHandler - A function to read and/or modify the response after an endpoint call
//                     * Useful for logging, metrics, notifications and more.
//                     * Use `nil` to have no response handler.
//
// Returns:
//   spec       - The specification object
//   fileServer - The fileServer http.Handler that can be mounted to show a Swagger UI for the API
//   err        - Any error that may have occurred
func NewOpenAPI(
	title, description, url, version, dir, schemaFilepath string,
	tags []oasm.Tag, endpoints []*Endpoint, routeCreator func(method, path string, handler http.Handler),
	middleware []Middleware, responseHandler ResponseHandler,
) (spec *OpenAPI, fileServer http.Handler, err error) {
	spec = &OpenAPI{
		Doc: oasm.OpenAPIDoc{
			OpenApi: "3.0.0",
			Info: &oasm.Info{
				Title:       title,
				Description: description,
				Version:     version,
			},
			Servers: []oasm.Server{{
				Url:         url,
				Description: title,
			}},
			Tags:       tags,
			Paths:      make(oasm.PathsMap),
			Components: oasm.Components{},
		},
		JSONIndent:      2,
		responseHandler: responseHandler,
		dir:             dir,
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil && !os.IsExist(err) {
		return nil, nil, err
	}

	// Create routes and docs for all endpoints
	for _, e := range endpoints {
		pathItem, ok := spec.Doc.Paths[e.Settings.Path]
		if !ok {
			pathItem = oasm.PathItem{
				Methods: make(map[string]oasm.Operation)}
			spec.Doc.Paths[e.Settings.Path] = pathItem
		}
		pathItem.Methods[e.Settings.Method] = e.Doc
		handler := e.userDefinedFunc
		if middleware != nil {
			for i := len(middleware) - 1; i >= 0; i-- {
				handler = middleware[i](handler)
			}
		}
		routeCreator(e.Settings.Method, e.Settings.Path, http.HandlerFunc(e.Call))
		e.fullyWrappedFunc = handler
		e.spec = spec
	}

	// Copy schemas from schema file into spec.
	contents, err := ioutil.ReadFile(schemaFilepath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read the schema file: %v", err)
	}
	var file map[string]interface{}
	err = json.Unmarshal(contents, &file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal the schema file: %v", err)
	}
	defs, ok := file["definitions"]
	if !ok {
		return nil, nil, fmt.Errorf("schema files must contain a top-level 'definitions' property")
	}
	defsMap, ok := defs.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("'definitions' property of schema files must be an object")
	}
	if spec.Doc.Components.Schemas == nil {
		spec.Doc.Components.Schemas = make(map[string]interface{})
	}
	for name, value := range defsMap {
		valueJson, _ := json.Marshal(value)
		spec.Doc.Components.Schemas[name] = json.RawMessage(valueJson)
	}

	// Create Swagger UI
	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		return nil, nil, err
	}
	if err = copy2.Copy(path.Join(path.Dir(filePath), "swagger-dist"), spec.dir); err != nil {
		return nil, nil, fmt.Errorf("failed to copy swagger ui distribution: %v", err)
	}
	indexHtml := fmt.Sprintf("%s/index.html", spec.dir)
	if contents, err := ioutil.ReadFile(indexHtml); err != nil {
		return nil, nil, fmt.Errorf("could not open 'index.html' in swagger directory: %s", err.Error())
	} else {
		regex, _ := regexp.Compile(`url: ?(".*?"|'.*?')`)
		newContents := regex.ReplaceAllLiteral(contents, []byte(fmt.Sprintf(`url: "./%s"`, specPath)))
		err := ioutil.WriteFile(indexHtml, newContents, 644)
		if err != nil {
			return nil, nil, err
		}
	}

	log.Println(spec.dir)
	return spec, http.FileServer(http.Dir(spec.dir)), nil
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
