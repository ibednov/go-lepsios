package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/ibednov/go-lepsios/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleMappedWithLocalizer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bundle, err := i18n.NewBundle("en")
	require.NoError(t, err)
	require.NoError(t, bundle.LoadMessages("en", []byte(`[{"id":"errors.bad","translation":"Bad"}]`)))
	loc := bundle.Localizer("en")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	mapper := func(err error) (int, string, string, bool) {
		if errors.Is(err, errTest) {
			return http.StatusBadRequest, "BAD", "errors.bad", true
		}
		return 0, "", "", false
	}
	response.Handle(c, errTest, mapper, loc)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var body response.ErrorBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "Bad", body.Message)
}

func TestErrorHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		fn   func(*gin.Context)
		code int
	}{
		{"notfound", func(c *gin.Context) { response.NotFound(c, "NF", "x") }, 404},
		{"conflict", func(c *gin.Context) { response.Conflict(c, "CF", "x") }, 409},
		{"internal", func(c *gin.Context) { response.Internal(c, "x") }, 500},
		{"badrequest", func(c *gin.Context) { response.BadRequest(c, "BR", "x") }, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			tt.fn(c)
			require.Equal(t, tt.code, w.Code)
		})
	}
}

func TestHandleNilError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	response.Handle(c, nil, nil, nil)
	require.Equal(t, 200, w.Code)
}
