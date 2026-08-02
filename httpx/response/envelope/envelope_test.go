package envelope_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/httpx/response/envelope"
	"github.com/stretchr/testify/require"
)

func TestSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("X-Trace-ID", "trace-1")
	c.Set("locale", "en")

	body := envelope.Success(c, []string{"a"})
	require.Equal(t, envelope.StatusSuccess, body.Status)
	require.Equal(t, "trace-1", body.Meta.RequestID)
	require.Equal(t, "en", body.Meta.Locale)
	require.NotNil(t, body.Data)
	require.Empty(t, body.Errors)
}

func TestSuccessWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	p := response.NewPagination(1, 10, 25)
	payload := envelope.SuccessWithPagination(c, []int{1, 2}, p)
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, "success", decoded["status"])

	meta := decoded["meta"].(map[string]any)
	require.EqualValues(t, 1, meta["page"])
	require.EqualValues(t, 10, meta["perPage"])
	require.EqualValues(t, 25, meta["totalCount"])
	require.EqualValues(t, 3, meta["totalPages"])

	nested := meta["pagination"].(map[string]any)
	require.EqualValues(t, 3, nested["totalPages"])
}

func TestWithErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	body := envelope.WithErrors(c, envelope.NewErrorDetail("boom", "err.code"))
	require.Equal(t, envelope.StatusError, body.Status)
	require.Nil(t, body.Data)
	require.Len(t, body.Errors, 1)
	require.Equal(t, "err.code", body.Errors[0].Code)
}
