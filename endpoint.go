package oas3

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oas3models"
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
	method           string
	run              func(r *http.Request) *Response
	version          int
	middleware       []mux.MiddlewareFunc
	responseHandlers []func(req *http.Request, res *Response)
}

// Create a new endpoint for your API, supplying the mandatory arguments as necessary.
func NewEndpoint(method string, path, summary, description string, tags ...string) *Endpoint {
	return &Endpoint{
		settings: &endpointSettings{
			method:           strings.ToLower(method),
			path:             path,
			middleware:       make([]mux.MiddlewareFunc, 0, 2),
			responseHandlers: make([]func(req *http.Request, res *Response), 0, 2),
		},
		Doc: &oas3models.OperationDoc{
			Tags:        tags,
			Summary:     summary,
			Description: description,
			OperationId: string(method) + strings.ReplaceAll(path, "/", "_"),
			Parameters:  make([]*oas3models.ParameterDoc, 0, 2),
			Responses: &oas3models.ResponsesDoc{
				Codes: make(map[int]*oas3models.ResponseDoc),
			},
			Security: make([]*oas3models.SecurityRequirementDoc, 0, 1),
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

// Attach a parameter doc.
func (e *Endpoint) Parameter(in oas3models.InRequest, name, description string, required bool, schema interface{}) *Endpoint {
	e.Doc.Parameters = append(e.Doc.Parameters, &oas3models.ParameterDoc{
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
	e.Doc.RequestBody = &oas3models.RequestBodyDoc{
		Description: description,
		Required:    required,
		Content: oas3models.MediaTypesDoc{
			oas3models.MimeJson: {Schema: schema},
		},
	}
	return e
}

// Attach a response doc. Schema may be nil.
func (e *Endpoint) Response(code int, description string, schema interface{}) *Endpoint {
	r := &oas3models.ResponseDoc{
		Description: description,
	}
	if schema != nil {
		r.Content = oas3models.MediaTypesDoc{
			oas3models.MimeJson: {
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
	e.Doc.Security = append(e.Doc.Security, &oas3models.SecurityRequirementDoc{
		Name:   name,
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
func (e *Endpoint) Func(f func(r *http.Request) *Response) *Endpoint {
	e.settings.run = f
	return e
}

func (e *Endpoint) run(w http.ResponseWriter, r *http.Request) {
	res := e.settings.run(r)

	if res.Error != nil {
		res.Body = errorToJSON(res.Error)
		res.Status = 500
	} else if res.Status == 0 {
		res.Status = 200
	}

	for _, rh := range e.settings.responseHandlers {
		rh(r, res)
	}
	if res.Error != nil {
		log.Printf("endpoint error (%s %s) at runtime: %s", e.settings.method, e.settings.path, res.Error)
	}

	var b []byte
	var err error
	if b, err = json.Marshal(res.Body); err != nil {
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
