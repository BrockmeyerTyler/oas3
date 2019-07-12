package oas

import (
	"encoding/json"
	"fmt"
	"github.com/tjbrockmeyer/oasm"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

// TODO: cover endpoint middleware operations after composing with NewOpenAPI()
// TODO: cover multiple endpoints not overwriting each other in OpenAPI.Endpoints()

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
	epPath := "/abc"
	summary := "this is a summary"
	description := "this is a description"

	e := NewEndpoint(method, epPath, summary, description, "TAG1", "TAG2")
	if e.Settings.Middleware == nil {
		t.Errorf("Middleware should not be nil")
	}
	if e.Settings.ResponseHandlers == nil {
		t.Errorf("ResponseHandlers should not be nil")
	}
	if e.Settings.Run != nil {
		t.Errorf("Run func should be nil")
	}
	if e.Settings.Path != epPath {
		t.Errorf("Expected path (%s) to be equal to %s", e.Settings.Path, epPath)
	}
	lowerMethod := strings.ToLower(method)
	if e.Settings.Method != lowerMethod {
		t.Errorf("Expected method (%s) to be equal to %s", e.Settings.Method, lowerMethod)
	}
	opId := fmt.Sprintf("%s%s", method, strings.ReplaceAll(epPath, "/", "_"))
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
	e := newEndpoint()
	e.Middleware(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// some middleware logic
		})
	})
	if len(e.Settings.Middleware) != 1 {
		t.Errorf("Endpoint middleware list should have exactly 1 item")
	}
}

func TestEndpoint_ResponseHandler(t *testing.T) {
	e := newEndpoint()
	e.ResponseHandler(func(req *http.Request, res *Response) {
		if res.Error != nil && strings.Contains(res.Error.Error(), "this is a test") {
			res.Status = 204
			res.Body = nil
			res.Error = nil
		}
	})

	e.Func(func(r *http.Request) *Response {
		return &Response{}
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/abc", nil)
	e.Run(w, r)
	if w.Code != 200 {
		t.Errorf("Expected status code to be returned unmodified")
	}

	e.Func(func(r *http.Request) *Response {
		return &Response{Error: fmt.Errorf("this is a test error")}
	})
	w = httptest.NewRecorder()
	e.Run(w, r)
	if w.Code != 204 {
		t.Errorf("Expected status code to have been handled by the response handler")
	}
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

	w = httptest.NewRecorder()
	e.Func(func(r *http.Request) *Response {
		return &Response{Body: json.RawMessage(`{"message":"bad json"`)}
	})
	e.Run(w, r)
	if w.Code != 500 {
		t.Errorf("Expected response code when response is unable to be marshalled to have status of 500")
	}
}

// openapi.go

func newApi() *OpenAPI {

	return NewOpenAPI("title", "description", "1.0.0", "./test/public")
}

func cleanupApi(o *OpenAPI) {
	_ = os.RemoveAll(o.dir)
}

func TestNewOpenAPI(t *testing.T) {
	title := "title"
	description := "description"
	version := "1.0.0"
	dir := "./test/public"
	//noinspection ALL
	defer os.RemoveAll("./test/public")

	o := NewOpenAPI(title, description, version, dir)
	if o.dir != dir {
		t.Errorf("Expected API dir (%s) to be equal to %s", o.dir, dir)
	}

	if o.Doc == nil {
		t.Errorf("Expected API doc to not be nil")
	}
	if o.Doc.Info == nil {
		t.Errorf("Expected API doc info to not be nil")
	}
	info := o.Doc.Info
	if info.Description != description || info.Version != version || info.Title != title {
		t.Errorf("Expected API doc info (%+v) to have description '%s', version '%s' and title '%s'",
			info, description, version, title)
	}
	if o.Doc.Servers == nil {
		t.Errorf("Expected API doc servers to not be nil")
	}
	if o.Doc.Tags == nil {
		t.Errorf("Expected API doc tags to not be nil")
	}
	if o.Doc.Paths == nil {
		t.Errorf("Expected API doc paths to not be nil")
	}
	if o.Doc.Components == nil {
		t.Errorf("Expected API doc components to not be nil")
	}

	_, err := os.Stat(dir)
	if err != nil && os.IsNotExist(err) {
		t.Errorf("Expected API directory to exist")
	} else if err != nil {
		t.Errorf("Unexpected error while checking for directory: %v", err)
	}

	_, err = os.Stat(path.Join(dir, "spec"))
	if err != nil && os.IsNotExist(err) {
		t.Errorf("Expected API spec directory to exist")
	} else if err != nil {
		t.Errorf("Unexpected error while checking for spec directory: %v", err)
	}

}

func TestOpenAPI_Endpoints(t *testing.T) {
	o := newApi()
	defer cleanupApi(o)
	e := newEndpoint()
	o.Endpoints(func(method, path string, handler http.HandlerFunc) {
		if method != e.Settings.Method || path != e.Settings.Path {
			t.Errorf("Expected path (%s) and method (%s) to be equal to the endpoint's path (%s) and method (%s)",
				path, method, e.Settings.Path, e.Settings.Method)
		}
	}, e)
	if pathItem, ok := o.Doc.Paths[e.Settings.Path]; !ok {
		t.Errorf("Expected a path item to be created for the endpoint path")
	} else if operation, ok := pathItem.Methods[oasm.HTTPVerb(e.Settings.Method)]; !ok {
		t.Errorf("Expected an operation item to be created for the endpoint method")
	} else if operation != e.Doc {
		t.Errorf("Expected the operation item to be qual to the endpoint doc")
	}
}

func TestOpenAPI_Server(t *testing.T) {
	url := "http://localhost:5000"
	description := "d"
	o := newApi()
	defer cleanupApi(o)
	o.Server(url, description)
	if len(o.Doc.Servers) != 1 {
		t.Errorf("Expected there to be exactly 1 server")
	}
	s := o.Doc.Servers[0]
	if s.Url != url || s.Description != description {
		t.Errorf("Expected server (%+v) to have a URL and description equal to %s and %s",
			s, url, description)
	}
}

func TestOpenAPI_Tag(t *testing.T) {
	name := "tag1"
	description := "tag1 description"
	o := newApi()
	defer cleanupApi(o)
	o.Tag(name, description)
	if len(o.Doc.Tags) != 1 {
		t.Errorf("Expected there to be exactly 1 tag")
	}
	tag := o.Doc.Tags[0]
	if tag.Name != name || tag.Description != description {
		t.Errorf("Expected tag (%+v) name and description to be equal to %s and %s",
			tag, name, description)
	}
}

func TestOpenAPI_SecurityRequirement(t *testing.T) {
	name := "secure"
	o := newApi()
	defer cleanupApi(o)
	o.SecurityRequirement(name, "SCOPE1", "SCOPE2")
	if len(o.Doc.Security) != 1 {
		t.Errorf("Expected there to be exactly 1 security requirement")
	}
	s := o.Doc.Security[0]
	if s.Name != name {
		t.Errorf("Expected security requirement (%s) to be named %s", s.Name, name)
	}
	if len(s.Scopes) != 2 {
		t.Errorf("Expected there to be exactly 2 scopes in the security requirement")
	}
	if s.Scopes[0] != "SCOPE1" || s.Scopes[1] != "SCOPE2" {
		t.Errorf("Expected the security requirement scopes (%v) to be equal to [SCOPE1,SCOPE2]", s.Scopes)
	}
}

func TestOpenAPI_NewAPIKey(t *testing.T) {
	name := "key"
	in := oasm.SecurityInHeader
	description := "description"
	paramName := "x-access-key"
	o := newApi()
	defer cleanupApi(o)
	o.NewAPIKey(in, name, description, paramName)

	if o.Doc.Components.SecuritySchemes == nil {
		t.Errorf("Expected security schemes to not be nil")
	} else if s, ok := o.Doc.Components.SecuritySchemes[name]; !ok {
		t.Errorf("Expected in the API components (%+v) a defined security scheme for %s",
			o.Doc.Components.SecuritySchemes, name)
	} else if s.Name != paramName || s.Type != "apiKey" || s.In != in {
		t.Errorf("Expected the security scheme (%+v) to have a name %s, a type %s, and in %s",
			s, name, "apiKey", in)
	}
}

func TestOpenAPI_NewClientCredentialsOAuth(t *testing.T) {
	name := "ccoauth"
	description := "description"
	url := "token.url.com/oauth"
	refreshUrl := "token.url.com/refresh"
	scopes := map[string]string{
		"SCOPE1": "description1",
	}
	o := newApi()
	defer cleanupApi(o)
	o.NewClientCredentialsOAuth(name, description, url, refreshUrl, scopes)

	if o.Doc.Components.SecuritySchemes == nil {
		t.Errorf("Expected security schemes to not be nil")
	} else if s, ok := o.Doc.Components.SecuritySchemes[name]; !ok {
		t.Errorf("Expected in the API components (%+v) a defined security scheme for %s",
			o.Doc.Components.SecuritySchemes, name)
	} else {
		if s.Type != "oauth2" {
			t.Errorf("Expected the security scheme (%s) to have a type of oauth2", s.Type)
		}
		if s.Flows == nil {
			t.Errorf("Expected the security scheme to have non-nil flows")
		} else if flow, ok := s.Flows["clientCredentials"]; !ok {
			t.Errorf("Expected a flow to have been defined for clientCredentials")
		} else {
			if d, ok := flow.Scopes["SCOPE1"]; !ok || d != "description1" {
				t.Errorf("Expected flow scopes (%v) to match %v", flow.Scopes, scopes)
			}
			if flow.RefreshUrl != refreshUrl || flow.TokenUrl != url {
				t.Errorf("Expected flow tokenUrl (%s) and refreshUrl (%s) to equal %s and %s",
					flow.TokenUrl, flow.RefreshUrl, url, refreshUrl)
			}
		}
	}
}

func TestOpenAPI_AddSchemaFile(t *testing.T) {
	o := newApi()
	defer cleanupApi(o)
	err := o.AddSchemaFile("./test/schemas.json", "prefix_")
	if err != nil {
		t.Errorf("Error while adding schema file: %v", err)
	}
	if o.Doc.Components.Schemas == nil {
		t.Errorf("Expected schemas to not be nil")
	}
	ref := `{"$ref":"schemas.json#/definitions/SearchResults"}`
	if searchResults, ok := o.Doc.Components.Schemas["prefix_SearchResults"]; !ok {
		t.Errorf("Expected 'SearchResults' schema (%+v) to exist with prefix_", o.Doc.Components.Schemas)
	} else if sr, ok := searchResults.(json.RawMessage); !ok {
		t.Errorf("Expected 'SearchResults' to be convertable to a json.RawMessage")
	} else if string(sr) != ref {
		t.Errorf("Expected value of 'SearchResults' (%s) to be equal to %s", sr, ref)
	}

	_, err = os.Stat("./test/public/spec/schemas.json")
	if err != nil && os.IsNotExist(err) {
		t.Errorf("Expected schema to be copied into ./test/public/spec/")
	} else if err != nil {
		t.Errorf("Unexpected error while checking for schema copy: %v", err)
	}

	err = o.AddSchemaFile("./test/badSchema1.json", "")
	if err == nil || !strings.Contains(err.Error(), "must contain") {
		t.Error("Expected an error about a missing field: 'definitions'")
	}

	err = o.AddSchemaFile("./test/badSchema2.json", "")
	if err == nil || !strings.Contains(err.Error(), "must be an object") {
		t.Error("Expected an error about a field that must be an object: 'definitions'")
	}

	err = o.AddSchemaFile("./test/badSchema3.json", "")
	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal") {
		t.Error("Expected an error about the file being malformed json")
	}

}

func TestOpenAPI_CreateSwaggerUI(t *testing.T) {
	o := newApi()
	defer cleanupApi(o)
	fileServer, err := o.CreateSwaggerUI()
	if err != nil {
		t.Errorf("Unexpected error while creating SwaggerUI: %v", err)
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
		regex, _ := regexp.Compile(`url: "\./spec/openapi\.json"`)
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
	o := newApi()
	defer cleanupApi(o)

	if err := o.Save(); err != nil {
		t.Errorf("Unexpected error while saving the spec")
	} else {
		spec := "./test/public/spec/openapi.json"
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
