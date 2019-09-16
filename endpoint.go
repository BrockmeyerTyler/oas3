package oas

import (
	"encoding/json"
	"fmt"
	"github.com/tjbrockmeyer/oasm"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"
)

type Endpoint struct {
	spec     *OpenAPI
	Settings *EndpointSettings
	Doc      *oasm.OperationDoc
}

type EndpointSettings struct {
	Path             string
	Method           string
	Run              func(d Data) (Response, error)
	Version          int
	Middleware       []func(h http.Handler) http.Handler
	ResponseHandlers []func(req *http.Request, res *Response, err error)
	BodyType         reflect.Type
}

// Create a new endpoint for your API, supplying the mandatory arguments as necessary.
func NewEndpoint(operationId, method, path, summary, description string, tags ...string) *Endpoint {
	return &Endpoint{
		Settings: &EndpointSettings{
			Method:           strings.ToLower(method),
			Path:             path,
			Middleware:       make([]func(http.Handler) http.Handler, 0, 2),
			ResponseHandlers: make([]func(*http.Request, *Response, error), 0, 2),
		},
		Doc: &oasm.OperationDoc{
			Tags:        tags,
			Summary:     summary,
			Description: description,
			OperationId: operationId,
			Parameters:  make([]*oasm.ParameterDoc, 0, 2),
			Responses: &oasm.ResponsesDoc{
				Codes: make(map[int]*oasm.ResponseDoc),
			},
			Security: make([]*oasm.SecurityRequirementDoc, 0, 1),
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
func (e *Endpoint) Parameter(in oasm.InRequest, name, description string, required bool, schema interface{}) *Endpoint {
	e.Doc.Parameters = append(e.Doc.Parameters, &oasm.ParameterDoc{
		Name:        name,
		Description: description,
		In:          in,
		Required:    required,
		Schema:      schema,
	})
	return e
}

// Attach a request body doc.
// `schema` will be used in the documentation, and `object` will be used for reading the body automatically.
func (e *Endpoint) RequestBody(description string, required bool, schema, object interface{}) *Endpoint {
	e.Settings.BodyType = reflect.TypeOf(object)
	e.Doc.RequestBody = &oasm.RequestBodyDoc{
		Description: description,
		Required:    required,
		Content: oasm.MediaTypesDoc{
			oasm.MimeJson: {Schema: schema},
		},
	}
	return e
}

// Attach a response doc. Schema may be nil.
func (e *Endpoint) Response(code int, description string, schema interface{}) *Endpoint {
	r := &oasm.ResponseDoc{
		Description: description,
	}
	if schema != nil {
		r.Content = oasm.MediaTypesDoc{
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
	e.Doc.Security = append(e.Doc.Security, &oasm.SecurityRequirementDoc{
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
func (e *Endpoint) ResponseHandler(rh func(*http.Request, *Response, error)) *Endpoint {
	e.Settings.ResponseHandlers = append(e.Settings.ResponseHandlers, rh)
	return e
}

// Attach a function to run when calling this endpoint
// If an error is caught, run the following: `return oas3.Response{Error: err}`
func (e *Endpoint) Func(f func(d Data) (Response, error)) *Endpoint {
	e.Settings.Run = f
	return e
}

func (e *Endpoint) getRequestBody(r *http.Request) (interface{}, error) {
	if e.Settings.BodyType == nil {
		return nil, nil
	}
	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %v", err)
	}
	err = r.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close request body: %v", err)
	}
	body := reflect.New(e.Settings.BodyType).Interface()
	err = json.Unmarshal(requestBody, body)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal request body: %v", err)
	}
	return body, nil
}

func (e *Endpoint) runFunc(w http.ResponseWriter, r *http.Request) (res Response, err error) {
	var body interface{}
	body, err = e.getRequestBody(r)
	if err != nil {
		return
	}
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			err = fmt.Errorf("a fatal error occurred: %v", panicErr)
			log.Printf("endpoint panic (%s %s): %s\n", e.Settings.Method, e.Settings.Path, panicErr)
			debug.PrintStack()
		}
	}()
	return e.Settings.Run(Data{
		Req:       r,
		ResWriter: w,
		Body:      body,
	})
}

func (e *Endpoint) Run(w http.ResponseWriter, r *http.Request) {
	res, err := e.runFunc(w, r)

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
		rh(r, &res, err)
	}
	if res.Body == nil {
		w.WriteHeader(res.Status)
		return
	}

	var b []byte
	if b, err = json.Marshal(res.Body); err != nil {
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
