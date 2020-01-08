package oas

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/tjbrockmeyer/oasm"
	"github.com/xeipuuv/gojsonschema"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
)

type Endpoint struct {
	// The operation documentation for this endpoint.
	Doc oasm.Operation

	// Options that can be read by middleware to add items to the request data before it gets to this endpoint.
	Options map[string]interface{}

	// The function defined during endpoint creation via Endpoint.Func().
	// During testing, it may be useful to call the function directly, or
	// to override this value by wrapping with some testing middleware
	UserDefinedFunc HandlerFunc

	path    string
	method  string
	version int

	fullyWrappedFunc HandlerFunc
	spec             *OpenAPI
	parsedPath       map[string]int

	bodyType reflect.Type
	query    []typedParameter
	params   map[int]typedParameter
	headers  []typedParameter

	dataSchema      *gojsonschema.Schema
	responseSchemas map[int]*gojsonschema.Schema
}

// Create a new endpoint for your API, supplying the mandatory arguments as necessary.
func NewEndpoint(operationId, method, path, summary, description string, tags []string) *Endpoint {
	parsedPath := make(map[string]int)
	pathParamRegex := regexp.MustCompile(`{[^/]+}`)
	splitPath := strings.Split(path, "/")
	for i, s := range splitPath {
		if pathParamRegex.MatchString(s) {
			parsedPath[s[1:len(s)-1]] = i
		}
	}
	return &Endpoint{
		Doc: oasm.Operation{
			Tags:        tags,
			Summary:     summary,
			Description: description,
			OperationId: operationId,
			Parameters:  make([]oasm.Parameter, 0, 2),
			Responses: oasm.Responses{
				Codes: make(map[int]oasm.Response),
			},
			Security: make([]oasm.SecurityRequirement, 0, 1),
		},
		Options:         make(map[string]interface{}, 3),
		path:            path,
		method:          strings.ToLower(method),
		parsedPath:      parsedPath,
		bodyType:        nil,
		query:           make([]typedParameter, 0, 3),
		params:          make(map[int]typedParameter, 3),
		headers:         make([]typedParameter, 0, 3),
		dataSchema:      nil,
		responseSchemas: make(map[int]*gojsonschema.Schema),
	}
}

// Returns settings of the endpoint that are not stored explicitly in the operation documentation.
func (e *Endpoint) Settings() (method, path string, version int) {
	return e.method, e.path, e.version
}

// Get a map of Security Requirements for this endpoint to their respective Security Schemes.
// Useful for authentication middleware.
func (e *Endpoint) GetSecuritySchemes() map[*oasm.SecurityRequirement]oasm.SecurityScheme {
	schemes := make(map[*oasm.SecurityRequirement]oasm.SecurityScheme)
	if e.spec != nil && e.spec.Doc.Security != nil {
		for _, requirement := range e.spec.Doc.Security {
			schemes[&requirement] = e.spec.Doc.Components.SecuritySchemes[requirement.Name]
		}
	}
	for _, requirement := range e.Doc.Security {
		schemes[&requirement] = e.spec.Doc.Components.SecuritySchemes[requirement.Name]
	}
	return schemes
}

// Add some options to the endpoint.
// These can be processed by custom middleware via the Endpoint.Options map.
func (e *Endpoint) Option(key string, value interface{}) *Endpoint {
	e.Options[key] = value
	return e
}

// Set the version of this endpoint, updating the path to correspond to it
func (e *Endpoint) Version(version int) *Endpoint {
	if version <= 0 || e.version != 0 {
		return e
	}
	e.Doc.OperationId += fmt.Sprintf("_v%v", version)
	e.path = fmt.Sprintf("/v%v", version) + e.path
	e.version = version
	return e
}

// Attach a parameter doc.
// Valid 'kind's are String, Int, Float64, and Bool
func (e *Endpoint) Parameter(in, name, description string, required bool, schema interface{}, kind reflect.Kind) *Endpoint {
	param := oasm.Parameter{
		Name:        name,
		Description: description,
		In:          in,
		Required:    required,
		Schema:      schema,
	}
	if kind != reflect.String && kind != reflect.Int && kind != reflect.Float64 && kind != reflect.Bool {
		e.printError(errors.New("kind should be one of String, Int, Float64, Bool"),
			"invalid kind for parameter %s in %s", name, in)
	}
	t := typedParameter{kind, param}
	e.Doc.Parameters = append(e.Doc.Parameters, param)
	switch in {
	case oasm.InQuery:
		e.query = append(e.query, t)
	case oasm.InPath:
		loc, ok := e.parsedPath[name]
		if !ok {
			e.printError(errors.New("path parameter provided in docs, but not provided in route"), "")
		} else {
			e.params[loc] = t
		}
	case oasm.InHeader:
		e.headers = append(e.headers, t)
	}
	return e
}

// Attach a request body doc.
// `schema` will be used in the documentation, and `object` will be used for reading the body automatically.
func (e *Endpoint) RequestBody(description string, required bool, schema, object interface{}) *Endpoint {
	e.bodyType = reflect.TypeOf(object)
	e.Doc.RequestBody = &oasm.RequestBody{
		Description: description,
		Required:    required,
		Content: oasm.MediaTypesMap{
			oasm.MimeJson: {
				Schema: schema,
			},
		},
	}
	return e
}

// Attach a response doc. Schema may be nil.
func (e *Endpoint) Response(code int, description string, schema interface{}) *Endpoint {
	r := oasm.Response{
		Description: description,
	}
	if schema != nil {
		r.Content = oasm.MediaTypesMap{
			oasm.MimeJson: {
				Schema: schema,
			},
		}
	}
	e.Doc.Responses.Codes[code] = r
	return e
}

// Deprecate this endpoint.
func (e *Endpoint) Deprecate(comment string) *Endpoint {
	e.Doc.Deprecated = true
	if comment != "" {
		e.Doc.Description += "<br/>DEPRECATED: " + comment
	}
	return e
}

// Attach a security doc.
func (e *Endpoint) Security(name string, scopes []string) *Endpoint {
	e.Doc.Security = append(e.Doc.Security, oasm.SecurityRequirement{
		Name:   name,
		Scopes: scopes,
	})
	return e
}

// Attach a function to run when calling this endpoint
func (e *Endpoint) Func(f HandlerFunc) *Endpoint {
	e.UserDefinedFunc = f
	return e
}

// Call this endpoint manually
// `Call` should only be used for testing purposes mostly.
func (e *Endpoint) Call(w http.ResponseWriter, r *http.Request) {
	data := Data{
		Req:       r,
		ResWriter: w,
		Query:     make(map[string]interface{}, len(e.query)),
		Params:    make(map[string]interface{}, len(e.params)),
		Headers:   make(map[string]interface{}, len(e.headers)),
		Endpoint:  e,
		Extra:     make(map[string]interface{}),
	}
	var res Response

	err := e.parseRequest(&data)
	if err == nil {
		res, err = e.runUserDefinedFunc(data)
	}

	if err != nil {
		if valErr, ok := err.(JSONValidationError); ok {
			res = Response{
				Body:   valErr,
				Status: 400,
			}
		} else if err == malformedJSONError {
			res = Response{
				Body:   malformedJSONError.Error(),
				Status: 400,
			}
		} else {
			res = Response{
				Body:   errorToJSON(err),
				Status: 500,
			}
		}
	} else if res.Ignore {
		return
	} else if res.Status == 0 {
		res.Status = 200
	}

	if e.spec != nil && e.spec.responseHandler != nil {
		e.spec.responseHandler(data, res, err)
	}

	if schema, ok := e.responseSchemas[res.Status]; ok {
		result, err := schema.Validate(gojsonschema.NewGoLoader(res.Body))
		if err != nil {
			e.printError(err, "response body contains malformed json")
		} else if !result.Valid() {
			e.printError(NewJSONValidationError(result), "response body failed validation for status %v", res.Status)
		}
	}

	if res.Body == nil {
		w.WriteHeader(res.Status)
		return
	}

	var b []byte

	if e.spec != nil && e.spec.JSONIndent > 0 {
		b, err = json.MarshalIndent(res.Body, "", strings.Repeat(" ", e.spec.JSONIndent))
	} else {
		b, err = json.Marshal(res.Body)
	}
	if err != nil {
		e.printError(err, "failed to marshal body (%s)", res.Body)
		res.Status = 500
		b = errorToJSON(err)
	}

	w.WriteHeader(res.Status)
	if _, err = w.Write(b); err != nil {
		e.printError(err, "error occurred while writing the response body")
	}
}

func (e *Endpoint) parseRequest(data *Data) error {
	var err error
	var requestBody []byte

	convertParamType := func(param typedParameter, item string) (interface{}, error) {
		switch param.kind {
		case reflect.String:
			return item, nil
		case reflect.Int:
			return strconv.Atoi(item)
		case reflect.Float64:
			return strconv.ParseFloat(item, 64)
		case reflect.Bool:
			return strconv.ParseBool(item)
		default:
			return nil, errors.New("bad reflection type for converting parameter from string")
		}
	}

	if e.bodyType != nil {
		requestBody, err = ioutil.ReadAll(data.Req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %v", err)
		}
		err = data.Req.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to close request body: %v", err)
		}
	}

	if len(e.query) > 0 {
		getQueryParam := data.Req.URL.Query().Get
		for _, param := range e.query {
			name := param.Name
			query := getQueryParam(name)
			if query == "" {
				continue
			}
			data.Query[name], err = convertParamType(param, query)
			if err != nil {
				return errors.WithMessage(err, "failed to convert query parameter "+name)
			}
		}
	}

	if len(e.params) > 0 {
		var basePathLength int
		if e.spec != nil {
			basePathLength = e.spec.basePathLength
		}
		if e.version != 0 {
			basePathLength += 1
		}
		splitPath := strings.Split(data.Req.URL.Path, "/")
		for loc, param := range e.params {
			if len(splitPath) <= loc {
				continue
			}
			data.Params[param.Name], err = convertParamType(param, splitPath[basePathLength+loc])
			if err != nil {
				return errors.WithMessage(err, "failed to convert path parameter "+param.Name)
			}
		}
	}

	if len(e.headers) > 0 {
		getHeader := data.Req.Header.Get
		for _, param := range e.headers {
			name := param.Name
			header := getHeader(name)
			if header == "" {
				continue
			}
			data.Headers[name], err = convertParamType(param, header)
			if err != nil {
				return errors.WithMessage(err, "failed to conver header parameter "+name)
			}
		}
	}

	if e.dataSchema != nil {
		dataJson := map[string]interface{}{
			"Query":   data.Query,
			"Params":  data.Params,
			"Headers": data.Headers,
		}
		if e.bodyType != nil {
			dataJson["Body"] = json.RawMessage(requestBody)
		}
		loader := gojsonschema.NewGoLoader(dataJson)
		result, err := e.dataSchema.Validate(loader)
		if err != nil {
			return malformedJSONError
		}
		if !result.Valid() {
			return NewJSONValidationError(result)
		}
		log.Println(result, err)
	}

	if e.bodyType != nil {
		data.Body = reflect.New(e.bodyType).Interface()
		err = json.Unmarshal(requestBody, data.Body)
		if err != nil {
			return malformedJSONError
		}
	}

	return nil
}

func (e *Endpoint) runUserDefinedFunc(data Data) (res Response, err error) {
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			err = fmt.Errorf("a fatal error occurred: %v", panicErr)
			log.Printf("endpoint panic (%s %s): %s\n", e.method, e.path, panicErr)
			debug.PrintStack()
		}
	}()
	if e.UserDefinedFunc == nil {
		return res, fmt.Errorf("endpoint function is not defined")
	}
	if e.spec == nil {
		return e.UserDefinedFunc(data)
	}
	return e.fullyWrappedFunc(data)
}

func (e *Endpoint) printError(err error, format string, args ...interface{}) {
	log.Printf("endpoint error (%s): %s: %s", e.Doc.OperationId, fmt.Sprintf(format, args...), err.Error())
}
