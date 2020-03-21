package oas

import (
	"encoding/json"
	"fmt"
	"github.com/tjbrockmeyer/oasm"
	"github.com/xeipuuv/gojsonschema"
	"strconv"
	"strings"
)

func newMalformedJSONError(err error) malformedJSONError {
	return malformedJSONError(fmt.Sprint("request contains malformed JSON: ", err.Error()))
}

type malformedJSONError string

func (err malformedJSONError) Error() string {
	return string(err)
}

func newJSONValidationError(result *gojsonschema.Result) jsonValidationError {
	errorList := make([]string, 0, len(result.Errors()))
	for _, e := range result.Errors() {
		errorList = append(errorList, fmt.Sprintf("At %s: %s", e.Context().String(), e.Description()))
	}
	return jsonValidationError{
		Type:   "JSONValidationError",
		Errors: errorList,
	}
}

func newParameterTypeError(param oasm.Parameter, expectedType, found string) jsonValidationError {
	return jsonValidationError{
		Type: "ParameterTypeError",
		Errors: []string{
			fmt.Sprintf("%s.%s: expected (%s) to be convertible to type %s", param.In, param.Name, found, expectedType),
		},
	}
}

type jsonValidationError struct {
	Type   string   `json:"type"`
	Errors []string `json:"errors"`
}

func (err jsonValidationError) Error() string {
	return "JSONValidationError:\n\t" + strings.Join(err.Errors, "\n\t")
}

func errorToJSON(err error) json.RawMessage {
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":%s}`, strconv.Quote(err.Error())))
}
