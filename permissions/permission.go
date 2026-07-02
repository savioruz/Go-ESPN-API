// Package permissions provides functionality to load and manage permissions from an embedded JSON file.
package permissions

import (
	_ "embed"
	"encoding/json"
	"slices"

	"github.com/rs/zerolog/log"
)

//go:embed permissions.json
var permissionsData []byte

// Permission represents the structure of a single permission entry in the permissions data.
type Permission struct {
	Permissions []string `json:"permissions"`
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Skip        bool     `json:"skip"`
}

// PermissionData represents the structure of the permissions data loaded from the embedded JSON file.
type PermissionData struct {
	Endpoints []Permission `json:"endpoints"`
	Skip      bool         `json:"skip"`
}

// FindPermissions searches for a permission matching the given path and method.
func (r *PermissionData) FindPermissions(path, method string) Permission {
	idx := slices.IndexFunc(r.Endpoints, func(rp Permission) bool {
		return rp.Path == path && rp.Method == method
	})

	if idx == -1 {
		return Permission{}
	}

	return r.Endpoints[idx]
}

// Get loads the embedded permissions data and returns a pointer to a PermissionData struct.
func Get() *PermissionData {
	var permissions PermissionData

	err := json.Unmarshal(permissionsData, &permissions)
	if err != nil {
		log.Err(err).Msg("Failed to decode embedded permissions")

		return nil
	}

	log.Info().Int("endpoints", len(permissions.Endpoints)).Msg("Successfully loaded embedded permissions")

	return &permissions
}
