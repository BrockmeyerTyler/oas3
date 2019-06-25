package oas3

type Response struct {
	Status int
	Body interface{}
	Headers map[string]string
	Error error
}
