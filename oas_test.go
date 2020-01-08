package oas

import (
	"encoding/json"
	"fmt"
	"github.com/tjbrockmeyer/oasm"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestErrorToJSON(t *testing.T) {
	j := errorToJSON(fmt.Errorf(`this is an "error"`))
	expected := `{"message":"Internal Server Error","details":"this is an \"error\""}`
	if string(j) != expected {
		t.Errorf("Expected %s, Got: %s", expected, j)
	}
}

// endpoint.go

func TestNewEndpoint(t *testing.T) {
	operationId := "getAbc"
	method := "GET"
	epPath := "/abc"
	summary := "this is a summary"
	description := "this is a description"

	e := NewEndpoint(operationId, method, epPath, summary, description, []string{"TAG1", "TAG2"})
	if e.UserDefinedFunc != nil {
		t.Errorf("Run func should be nil")
	}
	if e.path != epPath {
		t.Errorf("Expected path (%s) to be equal to %s", e.path, epPath)
	}
	lowerMethod := strings.ToLower(method)
	if e.method != lowerMethod {
		t.Errorf("Expected method (%s) to be equal to %s", e.method, lowerMethod)
	}
	if e.Doc.OperationId != operationId {
		t.Errorf("Expected OperationID (%s) to be equal to %s", e.Doc.OperationId, operationId)
	}
	if e.Doc.Parameters == nil {
		t.Errorf("Doc Parameters should not be nil")
	}
	if e.Doc.Security == nil {
		t.Errorf("Doc Security should not be nil")
	}
	if e.Doc.Tags == nil || e.Doc.Tags[0] != "TAG1" || e.Doc.Tags[1] != "TAG2" {
		t.Errorf("Doc Tags should be equal to [TAG1,TAG2]")
	}
	if e.Doc.Responses.Codes == nil {
		t.Errorf("Doc response codes should not be nil")
	}
	if e.Doc.Description != description {
		t.Errorf("Expected description (%s) to be equal to %s", e.Doc.Description, description)
	}
	if e.Doc.Summary != summary {
		t.Errorf("Expected summary (%s) to be equal to %s", e.Doc.Summary, summary)
	}
}

func newEndpoint() *Endpoint {
	return NewEndpoint("abc", "GET", "/abc", "summary", "description", []string{"TAG1"}).
		Func(func(data Data) (response Response, e error) {
			return Response{
				Status: 200,
			}, nil
		})
}

func TestEndpoint_Deprecate(t *testing.T) {
	comment := "deprecation reason"
	e := newEndpoint()
	e.Deprecate(comment)
	if !e.Doc.Deprecated {
		t.Errorf("Endpoint should be deprecated")
	}
	if !strings.Contains(e.Doc.Description, comment) {
		t.Errorf("Description (%s) should contain deprecation comment: %s", e.Doc.Description, comment)
	}
}

func TestEndpoint_Func(t *testing.T) {
	var funcRan bool
	e := newEndpoint()
	e.Func(func(d Data) (Response, error) {
		funcRan = true
		return Response{}, nil
	})
	if e.UserDefinedFunc == nil {
		t.Errorf("Run func should not be nil")
	}
	_, _ = e.UserDefinedFunc(Data{})
	if !funcRan {
		t.Errorf("Run func did not get called properly")
	}
}

func TestEndpoint_Version(t *testing.T) {
	v := 1
	e := newEndpoint()
	e.Version(v)
	if e.version != v {
		t.Errorf("Version (%v) should be equal to %v", e.version, v)
	}
	versionPath := fmt.Sprintf("/v%v", v)
	if !strings.HasPrefix(e.path, versionPath) {
		t.Errorf("Endpoint Path (%v) should start with the version: %v", e.path, versionPath)
	}
	versionOpId := fmt.Sprintf("_v%v", v)
	if !strings.HasSuffix(e.Doc.OperationId, versionOpId) {
		t.Errorf("Endpoint OperationID (%v) should end with the version: %v", e.Doc.OperationId, versionOpId)
	}
}

func TestEndpoint_Parameter(t *testing.T) {
	const loc = "query"
	name := "param"
	description := "this is a description"
	schema := json.RawMessage(`{"type":"string"}`)
	e := newEndpoint()
	e.Parameter(loc, name, description, true, schema, reflect.String)
	if len(e.Doc.Parameters) != 1 {
		t.Errorf("There should be exactly 1 parameter")
	}
	p := e.Doc.Parameters[0]
	if p.In != loc {
		t.Errorf("Expected parameter location (%s) to be equal to %s", p.In, loc)
	}
	if p.Description != description {
		t.Errorf("Expected parameter description (%s) to be equal to %s", p.Description, description)
	}
	if p.Name != name {
		t.Errorf("Expected parameter name (%s) to be equal to %s", p.Name, name)
	}
	if !p.Required {
		t.Errorf("Expected parameter to be required")
	}
	if string(p.Schema.(json.RawMessage)) != string(schema) {
		t.Errorf("Expected parameter schema (%v) to be equal to %v", p.Schema, schema)
	}
}

func TestEndpoint_Response(t *testing.T) {
	description := "this is a description"
	e := newEndpoint()
	e.Response(200, description, nil)
	if len(e.Doc.Responses.Codes) != 1 {
		t.Errorf("There should be exactly 1 response")
	}
	if response, ok := e.Doc.Responses.Codes[200]; !ok {
		t.Errorf("There should be a response with a code of 200")
	} else {
		if response.Description != description {
			t.Errorf("Expected response description (%s) to be equal to %s", response.Description, description)
		}
		if len(response.Content) > 0 {
			t.Errorf("Expected response to have no content")
		}
	}
	schema := json.RawMessage(`{"type":"object"}`)
	e.Response(409, "this is a conflict description", schema)
	if len(e.Doc.Responses.Codes) != 2 {
		t.Errorf("There should be exactly 2 responses")
	}
	if response, ok := e.Doc.Responses.Codes[409]; !ok {
		t.Errorf("There should be a response with a code of 409")
	} else if content, ok := response.Content["application/json"]; !ok {
		t.Errorf("Expected response to have a content type of 'application/json'")
	} else if string(content.Schema.(json.RawMessage)) != string(schema) {
		t.Errorf("Expected response schema (%v) to equal %v", content.Schema, schema)
	}
}

func TestEndpoint_RequestBody(t *testing.T) {
	type Body struct {
		Abc string `json:"abc"`
	}

	description := "description of the body"
	schema := json.RawMessage(`{"type":"object","properties":{"abc":{"type":"string"}}}`)
	e := newEndpoint()
	e.RequestBody(description, true, schema, Body{})
	if e.Doc.RequestBody.Content == nil {
		t.Errorf("RequestBody should not be nil")
	}
	rb := e.Doc.RequestBody
	if rb.Description != description {
		t.Errorf("Expected request body description (%s) to be equal to %s", rb.Description, description)
	}
	if rb.Required != true {
		t.Errorf("Expected request body to be required")
	}
	if content, ok := rb.Content["application/json"]; !ok {
		t.Errorf("Expected request body to have 'application/json' content type")
	} else if string(content.Schema.(json.RawMessage)) != string(schema) {
		t.Errorf("Expected request body schema (%s) to be equal to %s", content.Schema, schema)
	}

	val := "this is an ABC"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/abc", strings.NewReader(fmt.Sprintf(`{"abc":"%s"}`, val)))
	e.Func(func(d Data) (Response, error) {
		abc := d.Body.(*Body).Abc
		if abc != val {
			t.Errorf("Expected body to have been unmarshaled. Expected Abc=%s, Got Abc=%s", val, abc)
		}
		return Response{}, nil
	})
	e.Call(w, r)
}

func TestEndpoint_Security(t *testing.T) {
	name := "abc"
	e := newEndpoint()
	e.Security(name, nil)
	if len(e.Doc.Security) != 1 {
		t.Errorf("Expected Doc Security to have exactly 1 requirement")
		s := e.Doc.Security[0]
		if s.Name != name {
			t.Errorf("Expected security requirement name (%s) to be equal to %s", s.Name, name)
		}
	}

	e.Security("def", []string{"SCOPE1", "SCOPE2"})
	if len(e.Doc.Security) != 2 {
		t.Errorf("Expected Doc Security to have exactly 2 requirements")
	} else {
		scopes := e.Doc.Security[1].Scopes
		if len(scopes) != 2 || scopes[0] != "SCOPE1" || scopes[1] != "SCOPE2" {
			t.Errorf("Expected security requirement (%v) to be equal to: [SCOPE1,SCOPE2]", scopes)
		}
	}
}

func TestEndpoint_Run(t *testing.T) {
	var funcRan bool
	r := httptest.NewRequest("GET", "/abc", nil)
	e := newEndpoint()

	w := httptest.NewRecorder()
	e.Func(func(d Data) (Response, error) {
		funcRan = true
		return Response{}, nil
	})
	e.Call(w, r)
	if !funcRan {
		t.Errorf("Expected func to have been run")
	}
	if w.Code != 200 {
		t.Errorf("Expected default code to be 200")
	}

	body := `{"message":"Bad Request"}`
	w = httptest.NewRecorder()
	e.Func(func(d Data) (Response, error) {
		return Response{Status: 400, Body: json.RawMessage(body)}, nil
	})
	e.Call(w, r)
	if w.Code != 400 {
		t.Errorf("Expected response to have the given status code of 400")
	}
	b, _ := ioutil.ReadAll(w.Body)
	if string(b) != body {
		t.Errorf("Expected response body %s to be equal to the given body of %s", b, body)
	}

	w = httptest.NewRecorder()
	e.Func(func(d Data) (Response, error) {
		return Response{}, fmt.Errorf(`this is a test "error"`)
	})
	e.Call(w, r)
	if w.Code != 500 {
		t.Errorf("Expected response code during an error to have a status code of 500")
	}
	b, _ = ioutil.ReadAll(w.Body)
	expectedBody := `{"message":"Internal Server Error","details":"this is a test \"error\""}`
	if string(b) != expectedBody {
		t.Errorf("Expected response body during an error (%s) to be equal to %s", b, expectedBody)
	}

	w = httptest.NewRecorder()
	e.Func(func(d Data) (Response, error) {
		return Response{Body: json.RawMessage(`{"message":"bad json"`)}, nil
	})
	e.Call(w, r)
	if w.Code != 500 {
		t.Errorf("Expected response code when response is unable to be marshalled to have status of 500")
	}

	w = httptest.NewRecorder()
	e.Func(func(d Data) (Response, error) {
		panic("test error")
	})
	e.Call(w, r)
	if w.Code != 500 {
		t.Errorf("Expected response code when panic is encountered to have status of 500")
	}

	w = httptest.NewRecorder()
	e.Func(func(d Data) (Response, error) {
		d.ResWriter.WriteHeader(204)
		return Response{Ignore: true}, nil
	})
	e.Call(w, r)
	if w.Code != 204 {
		t.Errorf("Expected response to be ignored in favor of manual response writing. Status should be 204")
	}

	stringSchema := json.RawMessage(`{"type":"string"}`)
	r = httptest.NewRequest("POST", "/abc/123?kind=abc%20Kind", nil)
	r.Header.Set("x-info", "abcInfo")
	w = httptest.NewRecorder()
	e = NewEndpoint("createABC", "POST", "/abc/{id}", "Update an ABC", "<= What he said.", []string{"ABC"}).
		Parameter(oasm.InPath, "id", "The ABC's ID.", true, stringSchema, reflect.String).
		Parameter(oasm.InQuery, "kind", "The ABC's kind", true, stringSchema, reflect.String).
		Parameter(oasm.InHeader, "x-info", "The ABC's info", true, stringSchema, reflect.String).
		Func(func(d Data) (Response, error) {
			if len(d.Query) != 1 || d.Query["kind"] != "abc Kind" {
				t.Errorf("Expected Data.Query to have len()=1 and id=abc Kind. Got %v", d.Query)
			}
			if len(d.Params) != 1 || d.Params["id"] != "123" {
				t.Errorf("Expected Data.Params to have len()=1 and id=123. Got %v", d.Params)
			}
			if len(d.Headers) != 1 || d.Headers["x-info"] != "abcInfo" {
				t.Errorf("Expected Data.Headers to have len()=1 and id=abcInfo. Got %v", d.Headers)
			}
			return Response{}, nil
		})
	e.Call(w, r)
}

// openapi.go

func newApi(
	endpoints []*Endpoint,
	routeCreator func(method, path string, handler http.Handler),
	middleware []Middleware, responseHandler ResponseHandler,
) (*OpenAPI, http.Handler, error) {
	if endpoints == nil {
		endpoints = []*Endpoint{newEndpoint()}
	}
	if routeCreator == nil {
		routeCreator = func(method, path string, handler http.Handler) {
			// some logic here...
		}
	}
	if middleware == nil {
		middleware = make([]Middleware, 0)
	}
	return NewOpenAPI(
		"title", "description", "http://localhost", "1.0.0", "./test/public", "./example/schemas",
		[]oasm.Tag{{Name: "Tag1", Description: "The first tag"}},
		endpoints, routeCreator, middleware, responseHandler)
}

func cleanupApi(o *OpenAPI) {
	_ = os.RemoveAll(o.dir)
}

func TestNewOpenAPI(t *testing.T) {
	title := "title"
	description := "description"
	version := "1.0.0"
	dir := "./test/public"
	url := "http://localhost"
	tags := []oasm.Tag{
		{Name: "Tag1", Description: "This is the first tag"},
	}
	schemaDirPath := "./example/schemas"
	var middlewareRan, responseHandlerRan bool

	e := newEndpoint()
	o, fileServer, err := NewOpenAPI(
		title, description, url, version, dir, schemaDirPath, tags, []*Endpoint{e},
		func(method, path string, handler http.Handler) {
			if method != e.method || path != e.path {
				t.Errorf("Expected path (%s) and method (%s) to be equal to the endpoint's path (%s) and method (%s)",
					path, method, e.path, e.method)
			}
		}, []Middleware{
			func(next HandlerFunc) HandlerFunc {
				return func(data Data) (response Response, e error) {
					middlewareRan = true
					return next(data)
				}
			},
			func(next HandlerFunc) HandlerFunc {
				return func(data Data) (response Response, e error) {
					if !middlewareRan {
						t.Errorf("Expected this middleware to have been run second, not first")
					}
					return next(data)
				}
			},
		}, func(data Data, response Response, e error) Response {
			responseHandlerRan = true
			return response
		})
	defer cleanupApi(o)
	if err != nil {
		t.Error(err)
		return
	}
	if o.dir != dir {
		t.Errorf("Expected API dir (%s) to be equal to %s", o.dir, dir)
	}

	info := o.Doc.Info
	if info.Description != description || info.Version != version || info.Title != title {
		t.Errorf("Expected API doc info (%+v) to have description '%s', version '%s' and title '%s'",
			info, description, version, title)
	}

	if pathItem, ok := o.Doc.Paths[e.path]; !ok {
		t.Errorf("Expected a path item to be created for the endpoint path")
	} else if _, ok := pathItem.Methods[e.method]; !ok {
		t.Errorf("Expected an operation item to be created for the endpoint method")
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/abc", nil)
	e.Call(w, r)
	if !middlewareRan {
		t.Errorf("Expected middleware to have been run")
	}
	if !responseHandlerRan {
		t.Errorf("Expected response handler to have been run")
	}

	_, err = os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		t.Errorf("Expected API directory to exist")
	} else if err != nil {
		t.Errorf("Unexpected error while checking for directory: %v", err)
	}

	_, err = os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		t.Errorf("Expected API spec directory to exist")
	} else if err != nil {
		t.Errorf("Unexpected error while checking for spec directory: %v", err)
	}

	index := "./test/public/index.html"
	_, err = os.Stat(index)
	if err != nil && os.IsNotExist(err) {
		t.Errorf("Expected swagger-ui (index.html) to be copied into ./test/public/")
	} else if err != nil {
		t.Errorf("Unexpected error while checking for index.html: %v", err)
	} else if b, err := ioutil.ReadFile(index); err != nil {
		t.Errorf("Unexpected error while reading from index.html: %v", err)
	} else {
		regex, _ := regexp.Compile(`url: "\./openapi\.json"`)
		if !regex.Match(b) {
			t.Errorf("Expected the index.html API target url to be overwritten with the API's spec url")
		}
	}

	s := httptest.NewServer(fileServer)
	defer s.Close()
	if res, err := http.Get(s.URL); err != nil {
		t.Errorf("Unexpected error while calling the test server: %v", err)
	} else if res.StatusCode != 200 {
		t.Errorf("Expected a 200 status code from the fileserver")
	} else if b, err := ioutil.ReadAll(res.Body); err != nil {
		t.Errorf("Unexpected error while reading from the response body: %v", err)
	} else if !strings.Contains(string(b), "Swagger UI") {
		t.Errorf("Expected the response body to contain something about the 'Swagger UI'")
	}
}

func TestOpenAPI_Save(t *testing.T) {
	o, _, _ := newApi(nil, nil, nil, nil)
	defer cleanupApi(o)

	if err := o.Save(); err != nil {
		t.Errorf("Unexpected error while saving the spec")
	} else {
		spec := "./test/public/openapi.json"
		_, err := os.Stat(spec)
		if err != nil && os.IsNotExist(err) {
			t.Errorf("Expected spec file to have been created at %s", spec)
		} else if err != nil {
			t.Error("Unexpected error while checking for spec")
		} else {
			b, _ := ioutil.ReadFile(spec)
			if !strings.Contains(string(b), `"openapi"`) {
				t.Errorf("Expected spec to at least contain \"openapi\"")
			}
		}
	}
}
