package oas

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// util.go

func TestRef(t *testing.T) {
	ref := Ref("abc")
	expected := `{"$ref":"abc"}`
	if string(ref) != expected {
		t.Errorf("Expected %s, Got: %s", expected, ref)
	}
}

func TestSchemaRef(t *testing.T) {
	ref := SchemaRef("abc")
	expected := `{"$ref":"#/components/schemas/abc"}`
	if string(ref) != expected {
		t.Errorf("Expected %s, Got: %s", expected, ref)
	}
}

func TestCompRef(t *testing.T) {
	ref := CompRef("abc")
	expected := `{"$ref":"#/components/abc"}`
	if string(ref) != expected {
		t.Errorf("Expected %s, Got: %s", expected, ref)
	}
}

func TestErrorToJSON(t *testing.T) {
	j := errorToJSON(fmt.Errorf(`this is an "error"`))
	expected := `{"message":"Internal Server Error","details":"this is an \"error\""}`
	if string(j) != expected {
		t.Errorf("Expected %s, Got: %s", expected, j)
	}
}

// endpoint.go

func TestNewEndpoint(t *testing.T) {
	method := "GET"
	path := "/abc"
	summary := "this is a summary"
	description := "this is a description"

	e := NewEndpoint(method, path, summary, description, "TAG1", "TAG2")
	if e.Settings.Middleware == nil {
		t.Errorf("Middleware should not be nil")
	}
	if e.Settings.ResponseHandlers == nil {
		t.Errorf("ResponseHandlers should not be nil")
	}
	if e.Settings.Run != nil {
		t.Errorf("Run func should be nil")
	}
	if e.Settings.Path != path {
		t.Errorf("Expected path (%s) to be equal to %s", e.Settings.Path, path)
	}
	lowerMethod := strings.ToLower(method)
	if e.Settings.Method != lowerMethod {
		t.Errorf("Expected method (%s) to be equal to %s", e.Settings.Method, lowerMethod)
	}
	opId := fmt.Sprintf("%s%s", method, strings.ReplaceAll(path, "/", "_"))
	if e.Doc.OperationId != opId {
		t.Errorf("Expected OperationID (%s) to be equal to %s", e.Doc.OperationId, opId)
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
	return NewEndpoint("GET", "/abc", "summary", "description", "TAG1")
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
	e.Func(func(r *http.Request) *Response {
		funcRan = true
		return &Response{}
	})
	if e.Settings.Run == nil {
		t.Errorf("Run func should not be nil")
	}
	e.Settings.Run(nil)
	if !funcRan {
		t.Errorf("Run func did not get called properly")
	}
}

func TestEndpoint_Version(t *testing.T) {
	v := 1
	e := newEndpoint()
	e.Version(v)
	if e.Settings.Version != v {
		t.Errorf("Version (%v) should be equal to %v", e.Settings.Version, v)
	}
	versionPath := fmt.Sprintf("/v%v", v)
	if !strings.HasSuffix(e.Settings.Path, versionPath) {
		t.Errorf("Endpoint Path (%v) should end with the version: %v", e.Settings.Path, versionPath)
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
	e.Parameter(loc, name, description, true, schema)
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
	description := "description of the body"
	schema := json.RawMessage(`{"type":"object"}`)
	e := newEndpoint()
	e.RequestBody(description, true, schema)
	if e.Doc.RequestBody == nil {
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
}

func TestEndpoint_Security(t *testing.T) {
	name := "abc"
	e := newEndpoint()
	e.Security(name)
	if len(e.Doc.Security) != 1 {
		t.Errorf("Expected Doc Security to have exactly 1 requirement")
		s := e.Doc.Security[0]
		if s.Name != name {
			t.Errorf("Expected security requirement name (%s) to be equal to %s", s.Name, name)
		}
	}

	e.Security("def", "SCOPE1", "SCOPE2")
	if len(e.Doc.Security) != 2 {
		t.Errorf("Expected Doc Security to have exactly 2 requirements")
	} else {
		scopes := e.Doc.Security[1].Scopes
		if len(scopes) != 2 || scopes[0] != "SCOPE1" || scopes[1] != "SCOPE2" {
			t.Errorf("Expected security requirement (%v) to be equal to: [SCOPE1,SCOPE2]", scopes)
		}
	}
}

func TestEndpoint_Middleware(t *testing.T) {

}

func TestEndpoint_ResponseHandler(t *testing.T) {

}

func TestEndpoint_Run(t *testing.T) {
	var funcRan bool
	r := httptest.NewRequest("GET", "/abc", nil)
	e := newEndpoint()

	w := httptest.NewRecorder()
	e.Func(func(r *http.Request) *Response {
		funcRan = true
		return &Response{}
	})
	e.Run(w, r)
	if !funcRan {
		t.Errorf("Expected func to have been run")
	}
	if w.Code != 200 {
		t.Errorf("Expected default code to be 200")
	}

	body := `{"message":"Bad Request"}`
	w = httptest.NewRecorder()
	e.Func(func(r *http.Request) *Response {
		return &Response{Status: 400, Body: json.RawMessage(body)}
	})
	e.Run(w, r)
	if w.Code != 400 {
		t.Errorf("Expected response to have the given status code of 400")
	}
	b, _ := ioutil.ReadAll(w.Body)
	if string(b) != body {
		t.Errorf("Expected response body %s to be equal to the given body of %s", b, body)
	}

	w = httptest.NewRecorder()
	e.Func(func(r *http.Request) *Response {
		return &Response{Error: fmt.Errorf(`this is a test "error"`)}
	})
	e.Run(w, r)
	if w.Code != 500 {
		t.Errorf("Expected response code during an error to have a status code of 500")
	}
	b, _ = ioutil.ReadAll(w.Body)
	expectedBody := `{"message":"Internal Server Error","details":"this is a test \"error\""}`
	if string(b) != expectedBody {
		t.Errorf("Expected response body during an error (%s) to be equal to %s", b, expectedBody)
	}
}

// openapi.go

func TestNewOpenAPI(t *testing.T) {

}

func TestOpenAPI_Tag(t *testing.T) {

}

func TestOpenAPI_Server(t *testing.T) {

}

func TestOpenAPI_SecurityRequirement(t *testing.T) {

}

func TestOpenAPI_NewAPIKey(t *testing.T) {

}

func TestOpenAPI_NewClientCredentialsOAuth(t *testing.T) {

}

func TestOpenAPI_AddSchemaFile(t *testing.T) {

}

func TestOpenAPI_PublishSwaggerUI(t *testing.T) {

}
