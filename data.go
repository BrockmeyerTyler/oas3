package oas

import "net/http"

type Data struct {
	R    *http.Request
	W    http.ResponseWriter
	Body interface{}
}
