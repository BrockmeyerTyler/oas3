package oas

type Response struct {
	Status  int
	Body    interface{}
	Headers map[string]string
	Error   error
}
