// Package httpapi holds the HTTP contract generated from api/openapi.yaml
// (models and the strict server interface) plus small helpers shared by the
// bounded contexts' HTTP adapters.
package httpapi

//go:generate go tool oapi-codegen -config oapi-codegen.yaml ../../../../../api/openapi.yaml
