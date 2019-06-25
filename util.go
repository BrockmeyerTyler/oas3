package oas3

import "fmt"

func errorToJSON(err error) []byte{
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":"%s"}`, err.Error()))
}
