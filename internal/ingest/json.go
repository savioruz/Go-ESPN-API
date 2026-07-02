package ingest

import (
	"encoding/json"
	"strconv"
)

// The ESPN payloads are decoded into map[string]any trees and navigated with
// these lenient accessors, mirroring the Python `.get(...)` style. Missing or
// wrong-typed values yield the zero/default rather than panicking.

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)

	return s
}

// mmap returns m[key] as a map (nil when absent or not an object).
func mmap(m map[string]any, key string) map[string]any {
	return asMap(m[key])
}

// mslice returns m[key] as a slice (nil when absent or not an array).
func mslice(m map[string]any, key string) []any {
	return asSlice(m[key])
}

// mstr returns m[key] as a string ("" when absent or not a string).
func mstr(m map[string]any, key string) string {
	s, _ := m[key].(string)

	return s
}

// mstrOr returns the first non-empty string among the given keys, else "".
func mstrOr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}

	return ""
}

// mid returns m[key] rendered as a string id, accepting either a JSON string or
// number (ESPN ids appear in both forms). Empty when absent.
func mid(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

// mscalarStr returns m[key] as a string, coercing a JSON number to its textual
// form (used for competitor scores which may arrive as string or number).
func mscalarStr(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// mbool returns m[key] as a bool, defaulting to def when absent/not a bool.
func mbool(m map[string]any, key string, def bool) bool {
	b, ok := m[key].(bool)
	if !ok {
		return def
	}

	return b
}

// mboolPtr returns a pointer to m[key] as a bool, or nil when absent.
func mboolPtr(m map[string]any, key string) *bool {
	b, ok := m[key].(bool)
	if !ok {
		return nil
	}

	return &b
}

// mint returns m[key] as an int, defaulting to def when absent/not numeric.
func mint(m map[string]any, key string, def int) int {
	f, ok := m[key].(float64)
	if !ok {
		return def
	}

	return int(f)
}

// mintPtr returns a pointer to m[key] as an int, or nil when absent/not numeric.
func mintPtr(m map[string]any, key string) *int {
	f, ok := m[key].(float64)
	if !ok {
		return nil
	}

	i := int(f)

	return &i
}

// rawObj marshals v back to JSON bytes; on failure (or nil) it returns def.
func rawObj(v any, def string) json.RawMessage {
	if v == nil {
		return json.RawMessage(def)
	}

	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(def)
	}

	return b
}
