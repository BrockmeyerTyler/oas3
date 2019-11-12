package oas

import (
	"encoding/json"
	"fmt"
	"github.com/tjbrockmeyer/oasm"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
)

type Endpoint struct {
	Settings *EndpointSettings
	Doc      oasm.Operation

	fullyWrappedFunc HandlerFunc
	userDefinedFunc  HandlerFunc
	spec             *OpenAPI
	parsedPath       map[string]int

	bodyType reflect.Type
	query    []oasm.Parameter
	params   map[int]oasm.Parameter
	headers  []oasm.Parameter
}

type EndpointSettings struct {
	Path    string
	Method  string
	Version int
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
		Settings: &EndpointSettings{
			Method: strings.ToLower(method),
			Path:   path,
		},
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
		parsedPath: parsedPath,
		query:      make([]oasm.Parameter, 0, 3),
		params:     make(map[int]oasm.Parameter, 3),
		headers:    make([]oasm.Parameter, 0, 3),
	}
}

// Get a map of Security Requirements for this endpoint to their respective Security Schemes.
// Useful for authentication middleware.
func (e *Endpoint) GetSecuritySchemes() map[*oasm.SecurityRequirement]oasm.SecurityScheme {
	schemes := make(map[*oasm.SecurityRequirement]oasm.SecurityScheme)
	for _, requirement := range e.Doc.Security {
		schemes[&requirement] = e.spec.Doc.Components.SecuritySchemes[requirement.Name]
	}
	return schemes
}

// Set the version of this endpoint, updating the path to correspond to it
func (e *Endpoint) Version(version int) *Endpoint {
	if version <= 0 || e.Settings.Version != 0 {
		return e
	}
	e.Doc.OperationId += fmt.Sprintf("_v%v", version)
	e.Settings.Path = fmt.Sprintf("/v%v", version) + e.Settings.Path
	e.Settings.Version = version
	return e
}

// Attach a parameter doc.
func (e *Endpoint) Parameter(in, name, description string, required bool, schema interface{}) *Endpoint {
	param := oasm.Parameter{
		Name:        name,
		Description: description,
		In:          in,
		Required:    required,
		Schema:      schema,
	}
	e.Doc.Parameters = append(e.Doc.Parameters, param)
	switch in {
	case "query":
		e.query = append(e.query, param)
	case "path":
		loc, ok := e.parsedPath[name]
		if !ok {
			log.Printf("ERROR: path parameter %s provided in %s %s docs, but not provided in route",
				name, e.Settings.Method, e.Settings.Path)
		} else {
			e.params[loc] = param
		}
	case "header":
		if e.headers == nil {
			e.headers = make([]oasm.Parameter, 1, 3)
			e.headers[0] = param
		} else {
			e.headers = append(e.headers, param)
		}
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
	e.userDefinedFunc = f
	return e
}

// Call this endpoint manually
// `Call` should only be used for testing purposes mostly.
func (e *Endpoint) Call(w http.ResponseWriter, r *http.Request) {
	data := Data{
		Req:       r,
		ResWriter: w,
		Query:     make(map[string]string, len(e.query)),
		Params:    make(map[string]string, len(e.params)),
		Headers:   make(map[string]string, len(e.headers)),
		Endpoint:  e,
		Extra:     make(map[string]interface{}),
	}
	var res Response

	err := e.parseRequest(&data)
	if err == nil {
		res, err = e.runUserDefinedFunc(data)
	}

	if err != nil {
		res = Response{
			Body:   errorToJSON(err),
			Status: 500,
		}
	} else if res.Ignore {
		return
	} else if res.Status == 0 {
		res.Status = 200
	}

	if e.spec != nil && e.spec.responseHandler != nil {
		e.spec.responseHandler(data, res, err)
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
		log.Printf("endpoint error (%s %s) at marshal body: %s\n\tobject: %v",
			e.Settings.Method, e.Settings.Path, err, res.Body)
		res.Status = 500
		b = errorToJSON(err)
	}

	w.WriteHeader(res.Status)
	if _, err = w.Write(b); err != nil {
		log.Printf("endpoint error (%s %s) at write response: %s", e.Settings.Method, e.Settings.Path, err)
	}
}

func (e *Endpoint) parseRequest(data *Data) error {
	var err error
	if e.bodyType != nil {
		var requestBody []byte
		requestBody, err = ioutil.ReadAll(data.Req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %v", err)
		}
		err = data.Req.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to close request body: %v", err)
		}
		data.Body = reflect.New(e.bodyType).Interface()
		err = json.Unmarshal(requestBody, data.Body)
		if err != nil {
			return fmt.Errorf("failed to unmarshal request body: %v", err)
		}
	}

	if len(e.query) > 0 {
		getQueryParam := data.Req.URL.Query().Get
		for _, param := range e.query {
			name := param.Name
			data.Query[name] = getQueryParam(name)
		}
	}

	if len(e.params) > 0 {
		splitPath := strings.Split(data.Req.URL.Path, "/")
		log.Println("splitPath:", splitPath)
		for loc, param := range e.params {
			if len(splitPath) <= loc {
				continue
			}
			data.Params[param.Name] = splitPath[loc]
		}
	}

	if len(e.headers) > 0 {
		setHeader := data.Req.Header.Get
		for _, param := range e.headers {
			name := param.Name
			data.Headers[name] = setHeader(name)
		}
	}
	return nil
}

func (e *Endpoint) runUserDefinedFunc(data Data) (res Response, err error) {
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			err = fmt.Errorf("a fatal error occurred: %v", panicErr)
			log.Printf("endpoint panic (%s %s): %s\n", e.Settings.Method, e.Settings.Path, panicErr)
			debug.PrintStack()
		}
	}()
	if e.userDefinedFunc == nil {
		return res, fmt.Errorf("endpoint function is not defined")
	}
	if e.spec == nil {
		return e.userDefinedFunc(data)
	}
	return e.fullyWrappedFunc(data)
}
