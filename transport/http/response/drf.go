package response

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"go-espn-api/shared/constant"
	"go-espn-api/shared/logger"
)

// Paginated is the Django REST Framework PageNumberPagination envelope.
type Paginated struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Results  any     `json:"results"`
}

// ParsePage extracts the ?page query parameter (default 1, minimum 1, junk ignored).
func ParsePage(r *http.Request) int {
	raw := r.URL.Query().Get(constant.RequestParamPage)
	if raw == "" {
		return constant.DefaultValuePage
	}

	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 {
		return constant.DefaultValuePage
	}

	return page
}

// DRFList writes a 200 response using the raw DRF pagination envelope (no data
// wrapper). results must be a non-nil slice so that it marshals as [] rather
// than null when empty.
func DRFList(w http.ResponseWriter, r *http.Request, count, page, pageSize int, results any) {
	next := pageURL(r, page+1)
	if page*pageSize >= count {
		next = nil
	}

	var previous *string
	if page > 1 {
		previous = pageURL(r, page-1)
	}

	writeRaw(w, http.StatusOK, Paginated{
		Count:    count,
		Next:     next,
		Previous: previous,
		Results:  results,
	})
}

// DRFObject writes obj directly (no data wrapper) at the provided status code.
func DRFObject(w http.ResponseWriter, code int, obj any) {
	writeRaw(w, code, obj)
}

// DRFNotFound writes {"error": msg} at 404, matching Django's Response({"error": ...}, 404).
func DRFNotFound(w http.ResponseWriter, msg string) {
	writeRaw(w, http.StatusNotFound, map[string]string{"error": msg})
}

// pageURL builds an absolute URL for the current request with the page query
// param set to the given page. DRF omits the page param entirely for page 1.
func pageURL(r *http.Request, page int) *string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	if proto := r.Header.Get(constant.RequestHeaderForwardedProto); proto != "" {
		scheme = proto
	}

	query := r.URL.Query()
	if page <= 1 {
		query.Del(constant.RequestParamPage)
	} else {
		query.Set(constant.RequestParamPage, strconv.Itoa(page))
	}

	u := url.URL{
		Scheme:   scheme,
		Host:     r.Host,
		Path:     r.URL.Path,
		RawQuery: query.Encode(),
	}

	out := u.String()

	return &out
}

func writeRaw(w http.ResponseWriter, code int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		logger.ErrorWithStack(err)

		return
	}

	w.Header().Set(constant.RequestHeaderContentType, constant.ContentTypeJSON)
	w.WriteHeader(code)

	if _, err := w.Write(body); err != nil {
		logger.ErrorWithStack(err)
	}
}
