// Package validator provides validation utilities and error message mappings.
package validator

var (
	// messages maps validation tags to their corresponding error keys
	// These keys should be translated by the frontend to localized messages
	messages = map[string]string{
		"required":    "validation.required",
		"gte":         "validation.gte",
		"lte":         "validation.lte",
		"oneof":       "validation.oneof",
		"max":         "validation.max",
		"min":         "validation.min",
		"email":       "validation.email",
		"url":         "validation.url",
		"dive":        "validation.dive",
		"mimetypes":   "validation.mimetypes",
		"maxfilesize": "validation.maxfilesize",
		"empty":       "validation.empty",
	}
)
