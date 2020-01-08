package oas

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
	"strconv"
	"strings"
)

var (
	malformedJSONError = errors.New("request contains malformed JSON")
)

func NewJSONValidationError(result *gojsonschema.Result) JSONValidationError {
	errorList := make([]string, 0, len(result.Errors()))
	for _, e := range result.Errors() {
		errorList = append(errorList, fmt.Sprintf("At %s: %s", e.Context().String(), e.Description()))
	}
	return JSONValidationError{
		Type:   "JSONValidationError",
		Errors: errorList,
	}
}

type JSONValidationError struct {
	Type   string   `json:"type"`
	Errors []string `json:"errors"`
}

func (err JSONValidationError) Error() string {
	return "JSONValidationError:\n\t" + strings.Join(err.Errors, "\n\t")
}

func errorToJSON(err error) json.RawMessage {
	return []byte(fmt.Sprintf(`{"message":"Internal Server Error","details":%s}`, strconv.Quote(err.Error())))
}
