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

See [this example project.](./example)