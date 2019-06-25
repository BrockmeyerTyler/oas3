package oas3

import (
	"encoding/json"
	"github.com/brockmeyertyler/oas3models"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strings"
)

type Endpoint struct {
	settings *endpointSettings
	Doc      *oas3models.OperationDoc
}

type endpointSettings struct {
	path             string
	method           oas3models.HTTPVerb
	run              func(r *http.Request) *Response
	version          int
	middleware       []mux.MiddlewareFunc
	responseHandlers []func(req *http.Request, res *Response)
}


func NewEndpoint(method oas3models.HTTPVerb, path, summary, description string, tags []string) *Endpoint {
	return &Endpoint{
		settings: &endpointSettings{
			method:           method,
			path:             path,
			middleware:       make([]mux.MiddlewareFunc, 2),
			responseHandlers: make([]func(req *http.Request, res *Response), 2),
		},
		Doc: &oas3models.OperationDoc{
			Tags: tags,
			Summary: summary,
			Description: description,
			OperationId: string(method) + strings.ReplaceAll(path, "/", "_"),
			Parameters: make([]*oas3models.ParameterDoc, 2),
			Responses: &oas3models.ResponsesDoc{},
			Security: make([]*oas3models.SecurityRequirementDoc, 1),
		},
	}
}

// Set the version of this endpoint, updating the path to correspond to it
func (e *Endpoint) Version(version int) *Endpoint {
	e.Doc.OperationId += "_v" + string(version)
	e.settings.path += "/v" + string(version)
	e.settings.version = version
	return e
}

// Attach a parameter doc
func (e *Endpoint) Parameter(doc *oas3models.ParameterDoc) *Endpoint {
	e.Doc.Parameters = append(e.Doc.Parameters, doc)
	return e
}

// Attach a request body doc
func (e *Endpoint) RequestBody(doc *oas3models.RequestBodyDoc) *Endpoint {
	e.Doc.RequestBody = doc
	return e
}

// Attach a response doc
func (e *Endpoint) Response(code int, doc *oas3models.ResponseDoc) *Endpoint {
	e.Doc.Responses.Responses[code] = doc
	return e
}

func (e *Endpoint) Deprecate(comment string) *Endpoint {
	e.Doc.Deprecated = true
	if comment != "" {
		e.Doc.Description += "<br/>DEPRECATED: " + comment
	}
	return e
}

// Attach a security doc
func (e *Endpoint) Security(name string, scopes ...string) *Endpoint {
	e.Doc.Security = append(e.Doc.Security, &oas3models.SecurityRequirementDoc{
		Name: name,
		Scopes: scopes,
	})
	return e
}

// Attach middleware to this endpoint.
// Middleware is run before the endpoint function is called.
// This is a good place for authorization and logging.
func (e *Endpoint) Middleware(mdw mux.MiddlewareFunc) *Endpoint {
	e.settings.middleware = append(e.settings.middleware, mdw)
	return e
}

// Attach a response handler to this endpoint.
// Response handlers run after the endpoint call is complete.
// This is a good place for logging and metrics.
// They have the ability to view and modify the response before sending it.
// If there was an error, setting `res.Error` to `nil` will keep from printing it out.
func (e *Endpoint) ResponseHandler(rh func(*http.Request, *Response)) *Endpoint {
	e.settings.responseHandlers = append(e.settings.responseHandlers, rh)
	return e
}

// Attach a function to run when calling this endpoint
// If an error is caught, run the following: `return oas3.Response{Error: err}`
func (e *Endpoint) Func(f func(r *http.Request)*Response) *Endpoint {
	e.settings.run = f
	return e
}

func (e *Endpoint) run(w http.ResponseWriter, r *http.Request) {
	res := e.settings.run(r)
	if res.Error != nil {
		res.Body = errorToJSON(res.Error)
	}
	for _, rh := range e.settings.responseHandlers {
		rh(r, res)
	}
	if res.Error != nil {
		log.Printf("endpoint error (%s %s) at run: %s", e.settings.method, e.settings.path, res.Error)
	}

	var b []byte
	var err error
	b, err = json.Marshal(res.Body)
	if err != nil {
		log.Printf("endpoint error (%s %s) at marshal body: %s\n\tobject: %v",
			e.settings.method, e.settings.path, err, res.Body)
		res.Status = 500
		b = errorToJSON(err)
	}
	w.WriteHeader(res.Status)
	if _, err = w.Write(b); err != nil {
		log.Printf("endpoint error (%s %s) at write response: %s", e.settings.method, e.settings.path, err)
	}
}

