package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	response.OK(c, gin.H{"id": "1"})

	require.Equal(t, http.StatusOK, w.Code)
	var body response.SuccessBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "1", body.Data.(map[string]any)["id"])
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	response.Unauthorized(c, "UNAUTHORIZED", "nope")
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandleMapper(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	mapper := func(err error) (int, string, string, bool) {
		if errors.Is(err, errTest) {
			return http.StatusBadRequest, "BAD", "errors.bad", true
		}
		return 0, "", "", false
	}
	response.Handle(c, errTest, mapper, nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

var errTest = errors.New("bad")
