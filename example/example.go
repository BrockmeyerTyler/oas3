package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oas"
	"github.com/tjbrockmeyer/oasm"
	"log"
	"net/http"
	"reflect"
)

type Result struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Url         string `json:"url"`
}

func main() {

	var (
		address        = "localhost:5876"
		r              = mux.NewRouter()
		endpointRouter = r.PathPrefix("/api").Subrouter()
	)

	spec, fileServer := defineSpec(endpointRouter, address)
	defineEndpoints(spec)

	// Mount the file server at the desired URL.
	endpointRouter.Path("/docs").Handler(http.RedirectHandler("/api/docs/", http.StatusMovedPermanently))
	endpointRouter.PathPrefix("/docs/").Handler(http.StripPrefix("/api/docs/", fileServer))

	// Run the server.
	log.Printf("Swagger Docs at \"http://%s/api/docs/\".\n", address)
	log.Fatal(http.ListenAndServe(address, r))
}

func myAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e := r.Context().Value("endpoint").(oas.Endpoint)
		log.Println("hello, from myAuthMiddleware!: " + e.Doc().OperationId)
		for _, mapping := range e.SecurityMapping() {
			log.Println("security requirement:")
			for name, scheme := range mapping {
				log.Println(name, scheme.Type, scheme.Name)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func defineSpec(endpointRouter *mux.Router, address string) (oas.OpenAPI, http.Handler) {
	spec, fileServer, err := oas.NewOpenAPI(
		"API Title", "Description", "http://"+address+"/api", "1.0.0", "schemas", []oasm.Tag{
			{Name: "Tag1", Description: "This is the first tag."},
			{Name: "Tag2", Description: "This is the second tag."},
		}, func(endpoint oas.Endpoint, handler http.Handler) {
			method, path, _ := endpoint.Settings()
			log.Println(method, path)
			endpointRouter.Path(path).Methods(method).Name(endpoint.Doc().OperationId).Handler(
				oas.EndpointAttachingMiddleware(endpoint)(
					myAuthMiddleware(
						handler)))
		})
	if err != nil {
		panic(err)
	}

	// Make any changes desired to the spec.
	spec.SetResponseAndErrorHandler(func(data oas.Data, response oas.Response, e error) {
		if e != nil {
			log.Println("error: ", e)
		}
		method, path, version := data.Endpoint.Settings()
		log.Println(method, path, version, "| response:", response.Status)
	})
	spec.SetDefaultJSONIndent(2)
	spec.Doc().Components.SecuritySchemes = map[string]oasm.SecurityScheme{
		"Api Key": {Type: "apiKey", Name: "x-access-key", In: "header"},
		"Client": {Type: "oauth2", Flows: oasm.OAuthFlowsMap{
			"clientCredentials": {TokenUrl: "https://oauth2.my-site.com/token", Scopes: map[string]string{
				"read:email": "View the user's email address",
				"read:name":  "View the user's name",
			}},
		}},
	}

	return spec, fileServer
}

func defineEndpoints(spec oas.OpenAPI) {
	strSchema := json.RawMessage(`{"type":"string"}`)
	intSchema := json.RawMessage(`{"type":"integer"}`)

	spec.NewEndpoint("search", "GET", "/search", "Summary", "Description", []string{"Tag1", "Tag2"}).
		Version(1).
		Parameter("query", "q", "The search query", true, strSchema, reflect.String).
		Parameter("query", "limit", "Limit the amount of returned results", false, intSchema, reflect.Int).
		Parameter("query", "skip", "How many results to skip over before returning", false, intSchema, reflect.Int).
		Response(200, "Results were found", oas.Ref("{SearchResults}")).
		Response(204, "No results found", nil).
		MustDefine(func(data oas.Data) (interface{}, error) {
			// Your search logic here...
			return oas.Response{Status: 204}, nil
		})

	spec.NewEndpoint("getItem", "GET", "/item/{item}", "Get an Item", "Like, really get an Item if you want it", []string{"Tag1"}).
		Version(2).
		Parameter("path", "item", "the item to get", true, strSchema, reflect.String).
		Response(200, "Results were found", oas.Ref("{SearchResults}")).
		Response(204, "Item does not exist", nil).
		MustDefine(func(data oas.Data) (interface{}, error) {
			return json.RawMessage(fmt.Sprintf(`"got item: '%s'"`, data.Params["item"])), nil
		})

	spec.NewEndpoint("putItem", "PUT", "/item/{item}", "Put an Item", "Like, really put an Item if you want to", []string{"Tag2"}).
		Version(1).
		Parameter("path", "item", "the item to put", true, strSchema, reflect.String).
		RequestBody("Item details", true, oas.Ref("{Result}"), Result{}).
		Response(201, "Created/Updated", nil).
		MustDefine(func(data oas.Data) (interface{}, error) {
			return json.RawMessage(fmt.Sprintf(`"put item: '%s'"`, data.Params["item"])), nil
		})

	spec.NewEndpoint("postAbc", "POST", "/series/{id}/abc/{path:.*}", "Create an ABC", "", []string{"Tag2"}).
		Parameter("path", "id", "id", true, intSchema, reflect.Int).
		Parameter("path", "path", "Path to the abc in the series", true, strSchema, reflect.String).
		MustDefine(func(data oas.Data) (interface{}, error) {
			return data.Params.GetOrElse("path", "no path").(string), nil
		})
}
