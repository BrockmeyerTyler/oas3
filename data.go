package oas

import "net/http"

type Data struct {
	Req       *http.Request
	ResWriter http.ResponseWriter
	Body      interface{}
}
