package oas

type Response struct {
	// If set, the Response will be ignored.
	// Set ONLY when handling the response writer manually.
	Ignore  bool
	Status  int
	Body    interface{}
	Headers map[string]string
}
