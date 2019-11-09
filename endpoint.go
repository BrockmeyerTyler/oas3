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

	spec       *OpenAPI
	parsedPath map[string]int

	bodyType reflect.Type
	query    []oasm.Parameter
	params   map[int]oasm.Parameter
	headers  []oasm.Parameter
}

type EndpointSettings struct {
	Path             string
	Method           string
	Run              func(d Data) (Response, error)
	Version          int
	Middleware       []func(h http.Handler) http.Handler
	ResponseHandlers []func(d Data, res *Response, err error)
}

// Create a new endpoint for your API, supplying the mandatory arguments as necessary.
func NewEndpoint(operationId, method, path, summary, description string, tags ...string) *Endpoint {
	parsedPath := make(map[string]int)
	pathParamRegex := regexp.MustCompile(`{[^/]+}`)
	splitPath := strings.Split(path, "/")
	for i, s := range splitPath {
		if pathParamRegex.MatchString(s) {
			parsedPath[s[1:len(s)-1]] = i
		}
	}
	return &Endpoint{
		parsedPath: parsedPath,
		Settings: &EndpointSettings{
			Method:           strings.ToLower(method),
			Path:             path,
			Middleware:       make([]func(http.Handler) http.Handler, 0, 2),
			ResponseHandlers: make([]func(Data, *Response, error), 0, 2),
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
	}
}

// Set the version of this endpoint, updating the path to correspond to it
func (e *Endpoint) Version(version int) *Endpoint {
	e.Doc.OperationId += fmt.Sprintf("_v%v", version)
	e.Settings.Path += fmt.Sprintf("/v%v", version)
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
	case oasm.InQuery:
		if e.query == nil {
			e.query = make([]oasm.Parameter, 1, 3)
			e.query[0] = param
		} else {
			e.query = append(e.query, param)
		}
	case oasm.InPath:
		loc, ok := e.parsedPath[name]
		if ok {
			if e.params == nil {
				e.params = make(map[int]oasm.Parameter, 3)
			}
			e.params[loc] = param
		}
	case oasm.InHeader:
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
	e.Doc.RequestBody = oasm.RequestBody{
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
func (e *Endpoint) Security(name string, scopes ...string) *Endpoint {
	e.Doc.Security = append(e.Doc.Security, oasm.SecurityRequirement{
		Name:   name,
		Scopes: scopes,
	})
	return e
}

// Attach middleware to this endpoint.
// Middleware is run before the endpoint function is called.
// This is a good place for authorization and logging.
func (e *Endpoint) Middleware(mdw func(http.Handler) http.Handler) *Endpoint {
	e.Settings.Middleware = append(e.Settings.Middleware, mdw)
	return e
}

// Attach a response handler to this endpoint.
// Response handlers run after the endpoint call is complete.
// This is a good place for logging and metrics.
// They have the ability to view and modify the response before sending it.
// If there was an error, setting `res.Error` to `nil` will keep from printing it out.
func (e *Endpoint) ResponseHandler(rh func(Data, *Response, error)) *Endpoint {
	e.Settings.ResponseHandlers = append(e.Settings.ResponseHandlers, rh)
	return e
}

// Attach a function to run when calling this endpoint
// If an error is caught, run the following: `return oas3.Response{Error: err}`
func (e *Endpoint) Func(f func(d Data) (Response, error)) *Endpoint {
	e.Settings.Run = f
	return e
}

func (e *Endpoint) runFunc(w http.ResponseWriter, r *http.Request) (data Data, res Response, err error) {
	data = Data{
		Req:       r,
		ResWriter: w,
		Endpoint:  e,
	}

	if e.bodyType != nil {
		var requestBody []byte
		requestBody, err = ioutil.ReadAll(r.Body)
		if err != nil {
			return data, res, fmt.Errorf("failed to read request body: %v", err)
		}
		err = r.Body.Close()
		if err != nil {
			return data, res, fmt.Errorf("failed to close request body: %v", err)
		}
		data.Body = reflect.New(e.bodyType).Interface()
		err = json.Unmarshal(requestBody, data.Body)
		if err != nil {
			return data, res, fmt.Errorf("failed to unmarshal request body: %v", err)
		}
	}

	if e.query == nil {
		data.Query = make(map[string]string, 0)
	} else {
		data.Query = make(map[string]string, len(e.query))
		getQueryParam := r.URL.Query().Get
		for _, param := range e.query {
			name := param.Name
			data.Query[name] = getQueryParam(name)
		}
	}

	if e.params == nil {
		data.Params = make(map[string]string, 0)
	} else {
		splitPath := strings.Split(r.URL.Path, "/")
		data.Params = make(map[string]string, len(e.params))
		for loc, param := range e.params {
			if len(splitPath) <= loc {
				continue
			}
			data.Params[param.Name] = splitPath[loc]
		}
	}

	if e.headers == nil {
		data.Headers = make(map[string]string, 0)
	} else {
		data.Headers = make(map[string]string, len(r.Header))
		setHeader := r.Header.Get
		for _, param := range e.headers {
			name := param.Name
			data.Headers[name] = setHeader(name)
		}
	}

	defer func() {
		panicErr := recover()
		if panicErr != nil {
			err = fmt.Errorf("a fatal error occurred: %v", panicErr)
			log.Printf("endpoint panic (%s %s): %s\n", e.Settings.Method, e.Settings.Path, panicErr)
			debug.PrintStack()
		}
	}()

	res, err = e.Settings.Run(data)
	return
}

func (e *Endpoint) Run(w http.ResponseWriter, r *http.Request) {
	data, res, err := e.runFunc(w, r)

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

	for _, rh := range e.Settings.ResponseHandlers {
		rh(data, &res, err)
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
