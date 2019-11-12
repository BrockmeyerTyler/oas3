# oas - Open API Specification

[![GoDoc](https://godoc.org/github.com/tjbrockmeyer/oas?status.svg)](https://godoc.org/github.com/tjbrockmeyer/oas)
[![Build Status](https://travis-ci.com/tjbrockmeyer/oas.svg?branch=master)](https://travis-ci.com/tjbrockmeyer/oas)
[![codecov](https://codecov.io/gh/tjbrockmeyer/oas/branch/master/graph/badge.svg)](https://codecov.io/gh/tjbrockmeyer/oas)

Golang Open API Specification Version 3 simple API setup package  
Create json endpoint specs inline with your code implementation.  
This package specifically serves and accepts the `application/json` content type.

UI is created using [SwaggerUI.](https://github.com/swagger-api/swagger-ui)

The example below will create an API at http://localhost:5000 that has 1 endpoint, `GET /search` under 2 different tags.

For API documentation, view the [GoDoc Page.](https://godoc.org/github.com/tjbrockmeyer/oas)  

## Example: 
#### `main.go`
```go
package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/tjbrockmeyer/oas"
	"github.com/tjbrockmeyer/oasm"
	"log"
	"net/http"
)

func main() {
	strSchema := json.RawMessage(`{"type":"string"}`)
	intSchema := json.RawMessage(`{"type":"integer"}`)
	endpoints := []*oas.Endpoint{
		oas.NewEndpoint("search", "GET", "/search", "Summary", "Description", []string{"Tag1", "Tag2"}).
			Version(1).
			Parameter("query", "q", "The search query", true, strSchema).
			Parameter("query", "limit", "Limit the amount of returned results", false, intSchema).
			Parameter("query", "skip", "How many results to skip over before returning", false, intSchema).
			Response(200, "Results were found", oas.SchemaRef("SearchResults")).
			Response(204, "No results found", nil).
			Func(func(data oas.Data) (oas.Response, error) {
				// Your search logic here...
				return oas.Response{
					Status: 204,
				}, nil
			}),
	}

	address := "localhost:5000"
	r := mux.NewRouter()
	endpointRouter := r.PathPrefix("/api").Subrouter()

	spec, fileServer, err := oas.NewOpenAPI(
		"API Title", "Description", "http://localhost:5000/api", "1.0.0", "./public", "schemas.json", []oasm.Tag{
			{Name: "Tag1", Description: "This is the first tag."},
			{Name: "Tag2", Description: "This is the second tag."},
		}, endpoints, func(method, path string, handler http.Handler) {
			endpointRouter.Path(path).Methods(method).Handler(handler)
		}, []oas.EndpointMiddleware{
			func(next oas.HandlerFunc) oas.HandlerFunc {
				return func(data oas.Data) (response oas.Response, e error) {
					log.Println("This runs first")
					return next(data)
				}
			},
			func(next oas.HandlerFunc) oas.HandlerFunc {
				return func(data oas.Data) (response oas.Response, e error) {
					log.Println("This runs second")
					return next(data)
				}
			},
		}, func(data oas.Data, response oas.Response, e error) oas.Response {
			log.Println(data.Endpoint.Settings.Method, data.Endpoint.Settings.Path, "| response:", response.Status)
			return response
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
```

#### `./schemas.json`
```json
{
  "definitions": {
    "SearchResults": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["title", "description", "url"],
        "properties": {
          "title": {"type": "string"},
          "description": {"type": "string"},
          "url": {"type": "string"}
        }
      }
    }
  }
}
```