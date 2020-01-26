package oas

import (
	"encoding/json"
	"fmt"
	copy2 "github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/tjbrockmeyer/oasm"
	"github.com/tjbrockmeyer/vjsonschema"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
)

var specPath = "openapi.json"
var refRegex = regexp.MustCompile(`"\$ref"\s*:\s*"file:/[^"]*/(.*?)\.json"`)
var swaggerUrlRegex = regexp.MustCompile(`url: ?(".*?"|'.*?')`)

type HandlerFunc func(Data) (interface{}, error)
type Middleware func(next HandlerFunc) HandlerFunc
type RouteCreator func(method, path string, handler http.Handler)
type ResponseAndErrorHandler func(Data, Response, error)

type OpenAPI interface {
	// Get the API documentation for reading or modification.
	Doc() *oasm.OpenAPIDoc
	// Function to handle responses and/or errors that come from an endpoint function call.
	// The default implementation prints any errors to stdout.
	SetResponseAndErrorHandler(ResponseAndErrorHandler)
	// Set Indent level of JSON responses. (Default: 2) A level of 0 will print condensed JSON.
	SetDefaultJSONIndent(int)
	// Get Indent level of JSON responses. (Default: 2) A level of 0 will print condensed JSON.
	DefaultJSONIndent() int
	// Create a new endpoint for your API, complete with documentation.
	NewEndpoint(operationId, method, path, summary, description string, tags []string) EndpointDeclaration
	// Get all endpoints mapped by their operation ids.
	Endpoints() map[string]Endpoint
	// Save the spec into the directory
	Save() error
}

type openAPI struct {
	doc                     oasm.OpenAPIDoc
	jsonIndent              int
	responseAndErrorHandler ResponseAndErrorHandler
	dir                     string
	basePathLength          int
	validatorBuilder        vjsonschema.Builder
	validator               vjsonschema.Validator
	middleware              []Middleware
	routeCreator            RouteCreator
	endpoints               map[string]Endpoint
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
//   schemasDir      - A path to a directory of valid JSON Schemas for objects to be used by the API
//   tags            - A list of Tag objects for describing the sections of the API which hold endpoints
//   endpoints       - A list of Endpoints that have been created for this API
//   routeCreator    - A function which can mount an endpoint at an http path
//   middleware      - A list of EndpointMiddleware functions to be run before each endpoint when they are called
//                     * Useful for authorization, header pre-processing, and more.
//                     * Use `nil` or an empty list to have no middleware.
//
// Returns:
//   spec       - The specification object
//   fileServer - The fileServer http.Handler that can be mounted to show a Swagger UI for the API
//   err        - Any error that may have occurred
func NewOpenAPI(
	title, description, serverUrl, version, dir, schemasDir string,
	tags []oasm.Tag, routeCreator RouteCreator,
	middleware []Middleware,
) (spec OpenAPI, fileServer http.Handler, err error) {
	o := &openAPI{
		doc: oasm.OpenAPIDoc{
			OpenApi: "3.0.0",
			Info: &oasm.Info{
				Title:       title,
				Description: description,
				Version:     version,
			},
			Servers: []oasm.Server{{
				Url:         serverUrl,
				Description: title,
			}},
			Tags:       tags,
			Paths:      make(oasm.PathsMap),
			Components: oasm.Components{},
		},
		jsonIndent:       2,
		dir:              dir,
		validatorBuilder: vjsonschema.NewBuilder(),
		middleware:       middleware,
		routeCreator:     routeCreator,
		endpoints:        make(map[string]Endpoint),
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil && !os.IsExist(err) {
		return nil, nil, err
	}

	if parsedUrl, err := url.Parse(serverUrl); err != nil {
		return nil, nil, err
	} else {
		for _, s := range strings.Split(parsedUrl.Path, "/") {
			if len(s) > 0 {
				o.basePathLength += 1
			}
		}
	}

	if err := o.validatorBuilder.AddDir("", schemasDir); err != nil {
		return nil, nil, errors.WithMessage(err, "failed to read the schema directory")
	}

	if o.doc.Components.Schemas == nil {
		o.doc.Components.Schemas = make(map[string]interface{})
	}
	for k, s := range o.validatorBuilder.GetSchemas() {
		o.doc.Components.Schemas[k] = json.RawMessage(vjsonschema.SchemaRefReplace(s, refNameToSwaggerRef))
	}

	// Create Swagger UI
	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		return nil, nil, err
	}
	if err = copy2.Copy(path.Join(path.Dir(filePath), "swagger-dist"), o.dir); err != nil {
		return nil, nil, fmt.Errorf("failed to copy swagger ui distribution: %v", err)
	}
	indexHtml := fmt.Sprintf("%s/index.html", o.dir)
	if contents, err := ioutil.ReadFile(indexHtml); err != nil {
		return nil, nil, fmt.Errorf("could not open 'index.html' in swagger directory: %s", err.Error())
	} else {
		newContents := swaggerUrlRegex.ReplaceAllLiteral(contents, []byte(fmt.Sprintf(`url: "./%s"`, specPath)))
		err := ioutil.WriteFile(indexHtml, newContents, 644)
		if err != nil {
			return nil, nil, err
		}
	}

	return o, http.FileServer(http.Dir(o.dir)), nil
}

func (o *openAPI) Doc() *oasm.OpenAPIDoc {
	return &o.doc
}

func (o *openAPI) SetDefaultJSONIndent(i int) {
	o.jsonIndent = i
}

func (o *openAPI) DefaultJSONIndent() int {
	return o.jsonIndent
}

func (o *openAPI) SetResponseAndErrorHandler(reh ResponseAndErrorHandler) {
	o.responseAndErrorHandler = reh
}

func (o *openAPI) NewEndpoint(operationId, method, path, summary, description string, tags []string) EndpointDeclaration {
	parsedPath := make(map[string]int)
	pathParamRegex := regexp.MustCompile(`{[^/]+}`)
	splitPath := strings.Split(path, "/")
	for i, s := range splitPath {
		if pathParamRegex.MatchString(s) {
			parsedPath[s[1:len(s)-1]] = i
		}
	}
	e := &endpointObject{
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
		options:            make(map[string]interface{}, 3),
		path:               path,
		method:             strings.ToLower(method),
		parsedPath:         parsedPath,
		bodyType:           nil,
		query:              make([]typedParameter, 0, 3),
		params:             make(map[int]typedParameter, 3),
		headers:            make([]typedParameter, 0, 3),
		reqSchemaName:      "endpoint_" + operationId + "_request",
		responseSchemaRefs: make(map[int]string),
		spec:               o,
	}
	o.endpoints[operationId] = e
	return e
}

func (o *openAPI) Endpoints() map[string]Endpoint {
	return o.endpoints
}

func (o *openAPI) Save() error {
	if b, err := json.Marshal(o.doc); err != nil {
		return errors.WithMessage(err, "could not marshal Open API 3 spec: %s")
	} else if err = ioutil.WriteFile(path.Join(o.dir, specPath), b, 0644); err != nil {
		return fmt.Errorf("could not write Open API 3 spec to %s: %s", o.dir, err.Error())
	} else if o.validator, err = o.validatorBuilder.Compile(); err != nil {
		return errors.WithMessage(err, "could not compile jsonschema validator")
	}
	return nil
}
