package response_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"go-espn-api/transport/http/response"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decode(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))

	return out
}

func TestDRFList_EmptyResultsIsArrayNotNull(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/api/v1/sports/", nil)
	w := httptest.NewRecorder()

	// Non-nil empty slice must serialize as [].
	results := []string{}
	response.DRFList(w, r, 0, 1, 25, results)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"results":[]`)

	out := decode(t, w.Body.Bytes())
	assert.EqualValues(t, 0, out["count"])
	assert.Nil(t, out["next"])
	assert.Nil(t, out["previous"])
	assert.NotNil(t, out["results"])
}

func TestDRFList_FirstPageNextSetPreviousNull(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/api/v1/news/", nil)
	w := httptest.NewRecorder()

	// 60 items, page 1, size 25 -> next present, previous null.
	response.DRFList(w, r, 60, 1, 25, []int{})

	out := decode(t, w.Body.Bytes())
	assert.EqualValues(t, 60, out["count"])
	require.NotNil(t, out["next"])
	assert.Contains(t, out["next"].(string), "page=2")
	assert.Nil(t, out["previous"])
}

func TestDRFList_LastPagePreviousSetNextNull(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/api/v1/news/?page=3", nil)
	w := httptest.NewRecorder()

	// 60 items, page 3 (last), size 25 -> next null, previous present.
	response.DRFList(w, r, 60, 3, 25, []int{})

	out := decode(t, w.Body.Bytes())
	assert.Nil(t, out["next"])
	require.NotNil(t, out["previous"])
	assert.Contains(t, out["previous"].(string), "page=2")
}

func TestDRFList_PreviousToPageOneOmitsPageParam(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/api/v1/news/?page=2", nil)
	w := httptest.NewRecorder()

	response.DRFList(w, r, 60, 2, 25, []int{})

	out := decode(t, w.Body.Bytes())
	require.NotNil(t, out["previous"])
	// DRF omits the page param entirely when linking back to page 1.
	assert.NotContains(t, out["previous"].(string), "page=")
}

func TestParsePage(t *testing.T) {
	cases := map[string]int{
		"http://x/?page=5":   5,
		"http://x/":          1,
		"http://x/?page=0":   1,
		"http://x/?page=-3":  1,
		"http://x/?page=abc": 1,
	}

	for u, want := range cases {
		r := httptest.NewRequest("GET", u, nil)
		assert.Equal(t, want, response.ParsePage(r), u)
	}
}
