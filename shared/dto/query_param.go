package dto

import (
	"go-espn-api/shared/constant"
	"net/http"
	"strconv"
	"strings"
)

const (
	// SortDirAsc and SortDirDesc are constants for sorting direction
	SortDirAsc = "ASC"
	// SortDirDesc is used to indicate descending sort order
	SortDirDesc = "DESC"
)

// QueryParams represents common query parameters for pagination and sorting
type QueryParams struct {
	Page    int    `json:"page"     validate:"omitempty"`
	Limit   int    `json:"limit"    validate:"omitempty"`
	SortBy  string `json:"sort_by"  validate:"omitempty"`
	SortDir string `json:"sort_dir" validate:"omitempty,oneof=ASC DESC"`
}

// FromRequest populates QueryParams from the HTTP request.
// It's recommended to call this method with `defaultRequest` set to true if data is large
// Example:
//
//	q := &dto.QueryParams{}
//	q.FromRequest(req, true)
//
// This will set default values for Page, Limit, SortBy, and SortDir if they are not provided in the request.
// If `defaultRequest` is false, it will only populate the fields that are present in the request.
func (q *QueryParams) FromRequest(r *http.Request, defaultRequest bool) {
	queryParams := r.URL.Query()

	if page := queryParams.Get(constant.RequestParamPage); page != "" {
		if pageInt, err := strconv.Atoi(page); err == nil && pageInt > 0 {
			q.Page = pageInt
		}
	}

	if limit := queryParams.Get(constant.RequestParamLimit); limit != "" {
		if limitInt, err := strconv.Atoi(limit); err == nil && limitInt > 0 {
			q.Limit = limitInt
		}
	}

	if sortBy := queryParams.Get(constant.RequestParamSortBy); sortBy != "" {
		q.SortBy = sortBy
	}

	if sortDir := queryParams.Get(constant.RequestParamSortDir); strings.ToUpper(sortDir) == SortDirAsc || strings.ToUpper(sortDir) == SortDirDesc {
		q.SortDir = strings.ToUpper(sortDir)
	}

	if defaultRequest {
		if q.Page == 0 {
			q.Page = constant.DefaultValuePage
		}

		if q.Limit == 0 {
			q.Limit = constant.DefaultValueLimit
		}
	}
}
