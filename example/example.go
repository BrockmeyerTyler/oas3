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
	strSchema := json.RawMessage(`{"type":"string"}`)
	intSchema := json.RawMessage(`{"type":"integer"}`)
	endpoints := []*oas.Endpoint{
		oas.NewEndpoint("search", "GET", "/search", "Summary", "Description", []string{"Tag1", "Tag2"}).
			Version(1).
			Parameter("query", "q", "The search query", true, strSchema, reflect.String).
			Parameter("query", "limit", "Limit the amount of returned results", false, intSchema, reflect.Int).
			Parameter("query", "skip", "How many results to skip over before returning", false, intSchema, reflect.Int).
			Response(200, "Results were found", oas.Ref("SearchResults")).
			Response(204, "No results found", nil).
			Func(func(data oas.Data) (interface{}, error) {
				// Your search logic here...
				return oas.Response{Status: 204}, nil
			}),
		oas.NewEndpoint("getItem", "GET", "/item/{item}", "Get an Item", "Like, really get an Item if you want it", []string{"Tag1"}).
			Version(2).
			Parameter("path", "item", "the item to get", true, strSchema, reflect.String).
			Response(200, "Results were found", oas.Ref("SearchResults")).
			Response(204, "Item does not exist", nil).
			Func(func(data oas.Data) (interface{}, error) {
				return json.RawMessage(fmt.Sprintf(`"got item: '%s'"`, data.Params["item"])), nil
			}),
		oas.NewEndpoint("putItem", "PUT", "/item/{item}", "Put an Item", "Like, really put an Item if you want to", []string{"Tag2"}).
			Version(1).
			Parameter("path", "item", "the item to put", true, strSchema, reflect.String).
			RequestBody("Item details", true, oas.Ref("Result"), Result{}).
			Response(201, "Created/Updated", nil).
			Func(func(data oas.Data) (interface{}, error) {
				return json.RawMessage(fmt.Sprintf(`"put item: '%s'"`, data.Params["item"])), nil
			}),
	}

	address := "localhost:5000"
	r := mux.NewRouter()
	endpointRouter := r.PathPrefix("/api").Subrouter()

	spec, fileServer, err := oas.NewOpenAPI(
		"API Title", "Description", "http://localhost:5000/api", "1.0.0", "./public", "schemas", []oasm.Tag{
			{Name: "Tag1", Description: "This is the first tag."},
			{Name: "Tag2", Description: "This is the second tag."},
		}, endpoints, func(method, path string, handler http.Handler) {
			endpointRouter.Path(path).Methods(method).Handler(handler)
		}, []oas.Middleware{
			func(next oas.HandlerFunc) oas.HandlerFunc {
				return func(data oas.Data) (response interface{}, e error) {
					log.Println("This runs first")
					return next(data)
				}
			},
			func(next oas.HandlerFunc) oas.HandlerFunc {
				return func(data oas.Data) (response interface{}, e error) {
					log.Println("This runs second")
					return next(data)
				}
			},
		}, func(data oas.Data, response oas.Response, e error) {
			method, path, version := data.Endpoint.Settings()
			log.Println(method, path, version, "| response:", response.Status)
		})
	if err != nil {
		panic(err)
	}

	// Mount the file server at the desired URL.
	endpointRouter.Path("/docs").Handler(http.RedirectHandler("/api/docs/", http.StatusMovedPermanently))
	endpointRouter.PathPrefix("/docs/").Handler(http.StripPrefix("/api/docs/", fileServer))

	// Make any changes desired to the spec.
	spec.JSONIndent = 2
	spec.Doc.Components.SecuritySchemes = map[string]oasm.SecurityScheme{
		"Api Key": {Type: "apiKey", Name: "x-access-key", In: "header"},
		"Client": {Type: "oauth2", Flows: oasm.OAuthFlowsMap{
			"clientCredentials": {TokenUrl: "https://oauth2.my-site.com/token", Scopes: map[string]string{
				"read:email": "View the user's email address",
				"read:name":  "View the user's name",
			}},
		}},
	}

	// Save the spec.
	err = spec.Save()
	if err != nil {
		panic(err)
	}

	// Run the server.
	log.Printf("Swagger Docs at \"http://%s/api/docs/\".\n", address)
	log.Fatal(http.ListenAndServe(address, r))
}
