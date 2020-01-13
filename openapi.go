package oas

import (
	"encoding/json"
	"fmt"
	copy2 "github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/tjbrockmeyer/oasm"
	"github.com/xeipuuv/gojsonschema"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var specPath = "openapi.json"
var refRegex = regexp.MustCompile(`"\$ref"\s*:\s*"file:/[^"]*/(.*?)\.json"`)
var swaggerUrlRegex = regexp.MustCompile(`url: ?(".*?"|'.*?')`)

type HandlerFunc func(Data) (Response, error)
type Middleware func(next HandlerFunc) HandlerFunc
type ResponseHandler func(Data, Response, error) Response

type OpenAPI struct {
	Doc oasm.OpenAPIDoc
	// Indent level of JSON responses. (Default: 2) A level of 0 will print condensed JSON.
	JSONIndent      int
	responseHandler ResponseHandler
	dir             string
	basePathLength  int
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
//   responseHandler - A function to read and/or modify the response after an endpoint call
//                     * Useful for logging, metrics, notifications and more.
//                     * Use `nil` to have no response handler.
//
// Returns:
//   spec       - The specification object
//   fileServer - The fileServer http.Handler that can be mounted to show a Swagger UI for the API
//   err        - Any error that may have occurred
func NewOpenAPI(
	title, description, serverUrl, version, dir, schemasDir string,
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
				Url:         serverUrl,
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

	if parsedUrl, err := url.Parse(serverUrl); err != nil {
		return nil, nil, err
	} else {
		for _, s := range strings.Split(parsedUrl.Path, "/") {
			if len(s) > 0 {
				spec.basePathLength += 1
			}
		}
	}

	if spec.Doc.Components.Schemas == nil {
		spec.Doc.Components.Schemas = make(map[string]interface{})
	}

	// Gather all schemas, creating separate swagger schemas.
	err = filepath.Walk(schemasDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".json") {
			return nil
		}
		typeName := name[:len(name)-5]
		contents, err := ioutil.ReadFile(path)
		swaggerSchemaContents := refRegex.ReplaceAll(contents, []byte(`"$$ref":"#/components/schemas/$1"`))
		spec.Doc.Components.Schemas[typeName] = json.RawMessage(swaggerSchemaContents)
		if err != nil {
			return errors.WithMessage(err, "failed to parse json schema for "+typeName)
		}
		return nil
	})
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to read the schema directory")
	}

	for _, e := range endpoints {

		// Create a schema for the data object.
		querySchema := map[string]interface{}{
			"type":       "object",
			"required":   make([]string, 0, 3),
			"properties": make(map[string]interface{}),
		}
		paramsSchema := map[string]interface{}{
			"type":       "object",
			"required":   make([]string, 0, 3),
			"properties": make(map[string]interface{}),
		}
		headersSchema := map[string]interface{}{
			"type":       "object",
			"required":   make([]string, 0, 3),
			"properties": make(map[string]interface{}),
		}

		dataSchema := map[string]interface{}{
			"type": "object",
			"required": []string{
				"Query",
				"Params",
				"Headers",
			},
			"properties": map[string]interface{}{
				"Query":   querySchema,
				"Params":  paramsSchema,
				"Headers": headersSchema,
			},
		}

		// Create schema for request body.
		if e.Doc.RequestBody != nil {
			b := e.Doc.RequestBody
			bodyContent := b.Content["application/json"]

			// Resolve references
			if ref, ok := bodyContent.Schema.(Ref); ok {
				bodyContent.Schema = ref.toSwaggerSchema()
				e.Doc.RequestBody.Content["application/json"] = bodyContent
				dataSchema["properties"].(map[string]interface{})["Body"] = ref.toJSONSchema(schemasDir)
			} else {
				dataSchema["properties"].(map[string]interface{})["Body"] = bodyContent.Schema
			}

			// Set schema attributes.
			if b.Required {
				dataSchema["required"] = append(dataSchema["required"].([]string), "Body")
			}
		}

		// Create schemas for parameters.
		for i, p := range e.Doc.Parameters {
			var addToSchema *map[string]interface{}
			var jsonSchema interface{}

			switch p.In {
			case oasm.InQuery:
				addToSchema = &querySchema
			case oasm.InPath:
				addToSchema = &paramsSchema
			case oasm.InHeader:
				addToSchema = &headersSchema
			default:
				return nil, nil, errors.New("invalid value for 'in' on parameter: " + p.Name)
			}

			// Resolve references.
			if ref, ok := p.Schema.(Ref); ok {
				e.Doc.Parameters[i].Schema = ref.toSwaggerSchema()
				jsonSchema = ref.toJSONSchema(schemasDir)
			} else {
				jsonSchema = p.Schema
			}

			// Set schema attributes.
			(*addToSchema)["properties"].(map[string]interface{})[p.Name] = jsonSchema
			if p.Required {
				(*addToSchema)["required"] = append((*addToSchema)["required"].([]string), p.Name)
			}
		}

		// Save the schema to the endpoint.
		e.dataSchema, err = gojsonschema.NewSchema(gojsonschema.NewGoLoader(dataSchema))
		if err != nil {
			return nil, nil, errors.WithMessage(err, "failed to load dataSchema for endpoint: "+e.Doc.OperationId)
		}

		// Create schemas for response codes
		for code, response := range e.Doc.Responses.Codes {
			var jsonSchemaLoader interface{}
			responseContent := response.Content["application/json"]
			if responseContent.Schema == nil {
				continue
			}
			if ref, ok := responseContent.Schema.(Ref); ok {
				responseContent.Schema = ref.toSwaggerSchema()
				response.Content["application/json"] = responseContent
				e.Doc.Responses.Codes[code] = response
				jsonSchemaLoader = ref.toJSONSchema(schemasDir)
			} else {
				jsonSchemaLoader = responseContent.Schema
			}

			e.responseSchemas[code], err = gojsonschema.NewSchema(gojsonschema.NewGoLoader(jsonSchemaLoader))
			if err != nil {
				return nil, nil, errors.WithMessagef(
					err, "failed to load response schema: (%s, %v)", e.Doc.OperationId, code)
			}
		}

		// Create routes and docs for all endpoints
		pathItem, ok := spec.Doc.Paths[e.path]
		if !ok {
			pathItem = oasm.PathItem{
				Methods: make(map[string]oasm.Operation)}
			spec.Doc.Paths[e.path] = pathItem
		}
		pathItem.Methods[e.method] = e.Doc
		handler := e.UserDefinedFunc
		if middleware != nil {
			for i := len(middleware) - 1; i >= 0; i-- {
				handler = middleware[i](handler)
			}
		}
		routeCreator(e.method, e.path, http.HandlerFunc(e.Call))
		e.fullyWrappedFunc = handler
		e.spec = spec
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
		newContents := swaggerUrlRegex.ReplaceAllLiteral(contents, []byte(fmt.Sprintf(`url: "./%s"`, specPath)))
		err := ioutil.WriteFile(indexHtml, newContents, 644)
		if err != nil {
			return nil, nil, err
		}
	}

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
