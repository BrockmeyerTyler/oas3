package oas

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oasm"
	"log"
	"net/http"
	"strings"
)

type Endpoint struct {
	Settings *endpointSettings
	Doc      *oasm.OperationDoc
}

type endpointSettings struct {
	Path             string
	Method           string
	Run              func(r *http.Request) *Response
	Version          int
	Middleware       []mux.MiddlewareFunc
	ResponseHandlers []func(req *http.Request, res *Response)
}

// Create a new endpoint for your API, supplying the mandatory arguments as necessary.
func NewEndpoint(method string, path, summary, description string, tags ...string) *Endpoint {
	return &Endpoint{
		Settings: &endpointSettings{
			Method:           strings.ToLower(method),
			Path:             path,
			Middleware:       make([]mux.MiddlewareFunc, 0, 2),
			ResponseHandlers: make([]func(req *http.Request, res *Response), 0, 2),
		},
		Doc: &oasm.OperationDoc{
			Tags:        tags,
			Summary:     summary,
			Description: description,
			OperationId: fmt.Sprintf("%s%s", method, strings.ReplaceAll(path, "/", "_")),
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
func (e *Endpoint) RequestBody(description string, required bool, schema interface{}) *Endpoint {
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
func (e *Endpoint) Middleware(mdw mux.MiddlewareFunc) *Endpoint {
	e.Settings.Middleware = append(e.Settings.Middleware, mdw)
	return e
}

// Attach a response handler to this endpoint.
// Response handlers run after the endpoint call is complete.
// This is a good place for logging and metrics.
// They have the ability to view and modify the response before sending it.
// If there was an error, setting `res.Error` to `nil` will keep from printing it out.
func (e *Endpoint) ResponseHandler(rh func(*http.Request, *Response)) *Endpoint {
	e.Settings.ResponseHandlers = append(e.Settings.ResponseHandlers, rh)
	return e
}

// Attach a function to run when calling this endpoint
// If an error is caught, run the following: `return oas3.Response{Error: err}`
func (e *Endpoint) Func(f func(r *http.Request) *Response) *Endpoint {
	e.Settings.Run = f
	return e
}

func (e *Endpoint) Run(w http.ResponseWriter, r *http.Request) {
	res := e.Settings.Run(r)

	if res.Error != nil {
		res.Body = errorToJSON(res.Error)
		res.Status = 500
	} else if res.Status == 0 {
		res.Status = 200
	}

	for _, rh := range e.Settings.ResponseHandlers {
		rh(r, res)
	}
	if res.Error != nil {
		log.Printf("endpoint error (%s %s) at runtime: %s", e.Settings.Method, e.Settings.Path, res.Error)
	}
	if res.Body == nil {
		w.WriteHeader(res.Status)
		return
	}

	var b []byte
	var err error
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
