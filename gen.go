//go:build generate

package main

//go:generate go tool oapi-codegen -config oapi-codegen.yaml docs/openapi.yaml
