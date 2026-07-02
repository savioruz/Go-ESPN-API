// Package tools anchors the project's code-generation directives at the module
// root. It carries no build tag (unlike tools.go) so the root forms a valid,
// buildable package — otherwise `go list ./` fails and swag can't resolve the
// package name when generating OpenAPI docs.
package tools

//go:generate go run github.com/swaggo/swag/cmd/swag init -g cmd/app/main.go --parseInternal
//go:generate go run github.com/google/wire/cmd/wire ./di
