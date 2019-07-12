# oas - Open API Specification
Golang Open API Specification Version 3 simple API setup package  
Create json endpoint specs inline with your code implementation.  
This package specifically serves and accepts the `application/json` content type.

**This project is still in development, and may receive breaking changes without notification**

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
	"log"
	"net/http"
)

func main() {
	strSchema := json.RawMessage(`{"type":"string"}`)
	intSchema := json.RawMessage(`{"type":"integer"}`)
	ep := oas.NewEndpoint("GET", "/search", "Summary", "Description", "Tag1", "Tag2").
		Version(1).
		Parameter("query", "q", "The search query", true, strSchema).
		Parameter("query", "limit", "Limit the amount of returned results", true, intSchema).
		Parameter("query", "skip", "How many results to skip over before returning", true, intSchema).
		Response(200, "Results were found", oas.SchemaRef("SearchResults")).
		Response(204, "No results found", nil).
		Func(func(r *http.Request) *oas.Response {
			// Your search logic here...
			return &oas.Response{Status: 204}
		})
	// ep2 := oas.NewEndpoint(...)
	// ep3 := oas.NewEndpoint(...)
	// endpoints := []*oas.Endpoint{...}

	address := "localhost:5000"
	r := mux.NewRouter().StrictSlash(true)
	endpointRouter := r.PathPrefix("/api").Subrouter()

	spec := oas.NewOpenAPI("API Title", "Description", "1.0.0", "./public").
		Endpoints(func(method, path string, handler http.HandlerFunc) {
			endpointRouter.Path(path).Methods(method).HandlerFunc(handler)
		}, ep, /* ep2, ep3, ...endpoints, etc */).
		Server(address, "The local API").
		Tag("Tag1", "The first tag").
		Tag("Tag2", "The second tag")
	if fileServer, err := spec.CreateSwaggerUI(); err != nil {
		panic(err)
	} else {
		endpointRouter.PathPrefix("/docs").Handler(http.StripPrefix("/api/docs/", fileServer))
	}
	if err := spec.AddSchemaFile("schemas.json", ""); err != nil {
		panic(err)
	}
	if err := spec.Save(); err != nil {
		panic(err)
	}

	log.Printf("Listening at \"http://%s\".\n", address)
	log.Fatal(http.ListenAndServe(address, r))
}
```

#### `./schemas.json`
```json
{
  "definitions": {
    "SearchResults": {
      "type": "object"
    }
  }
}
```