package oas

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/tjbrockmeyer/oasm"
	"github.com/tjbrockmeyer/vjsonschema"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strings"
)

var pathRegex = regexp.MustCompile(`/(?:[^{][^/]*|{(\w+)(?::(.*?[^\\]))?})`)

type HandlerFunc func(Data) (interface{}, error)
type Middleware func(next HandlerFunc) HandlerFunc
type RouteCreator func(Endpoint, http.Handler)
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
}

type openAPI struct {
	doc                     oasm.OpenAPIDoc
	jsonIndent              int
	responseAndErrorHandler ResponseAndErrorHandler
	validatorBuilder        vjsonschema.Builder
	validator               vjsonschema.Validator
	routeCreator            RouteCreator
	endpoints               map[string]Endpoint
	fileServer              *customFileServer
	url                     *url.URL
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
//   title        - API title
//   description  - API description
//   serverUrl    - API URL location
//   version      - API version in the format of (MAJOR.MINOR.PATCH)
//   dir          - A directory for hosting the spec, schemas, and SwaggerUI - typically a folder like ./public
//   schemasDir   - A path to a directory of valid JSON Schemas for objects to be used by the API
//   tags         - A list of Tag objects for describing the sections of the API which hold endpoints
//   routeCreator - A function which can add middleware and mount an endpoint at an http path
//
// Returns:
//   spec       - The specification object
//   fileServer - The fileServer http.Handler that can be mounted to show a Swagger UI for the API
//   err        - Any error that may have occurred
func NewOpenAPI(
	title, description, serverUrl, version, schemasDir string,
	tags []oasm.Tag, routeCreator RouteCreator,
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
		validatorBuilder: vjsonschema.NewBuilder(),
		routeCreator:     routeCreator,
		endpoints:        make(map[string]Endpoint),
	}
	if parsedUrl, err := url.Parse(serverUrl); err != nil {
		return nil, nil, err
	} else {
		o.url = parsedUrl
	}

	if err := o.validatorBuilder.AddDir(schemasDir); err != nil {
		return nil, nil, errors.WithMessage(err, "failed to read the schema directory")
	}

	if o.doc.Components.Schemas == nil {
		o.doc.Components.Schemas = make(map[string]interface{})
	}
	for k, s := range o.validatorBuilder.GetSchemas() {
		o.doc.Components.Schemas[k] = json.RawMessage(vjsonschema.SchemaRefReplace(s, refNameToSwaggerRef))
	}

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		return nil, nil, err
	}
	swaggerDist := path.Join(path.Dir(filePath), "swagger-dist")
	d := http.Dir(swaggerDist)
	fileServer = &customFileServer{
		dir:        d,
		fileServer: http.FileServer(d),
		o:          o,
	}
	return o, fileServer, nil
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
		path:               path,
		method:             strings.ToLower(method),
		bodyType:           nil,
		query:              make([]typedParameter, 0, 3),
		params:             make(map[int]typedParameter, 3),
		headers:            make([]typedParameter, 0, 3),
		reqSchemaName:      "endpoint_" + operationId + "_request",
		responseSchemaRefs: make(map[int]string),
		spec:               o,
	}
	if _, ok := o.endpoints[operationId]; ok {
		e.err = errors.New("duplicate endpoint definition for operationId: " + operationId)
	} else {
		o.endpoints[operationId] = e
	}
	e.parsePath()
	return e
}

func (o *openAPI) Endpoints() map[string]Endpoint {
	return o.endpoints
}

func (o *openAPI) buildValidator() (err error) {
	if o.validator, err = o.validatorBuilder.Compile(); err != nil {
		return errors.WithMessage(err, "could not compile jsonschema validator")
	}
	return nil
}
