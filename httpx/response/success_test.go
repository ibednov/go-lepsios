package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	response.Created(c, gin.H{"id": "1"})
	require.Equal(t, http.StatusCreated, w.Code)

	var body response.SuccessBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data)
}

func TestOKWithMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	response.OKWithMeta(c, []string{"a"}, gin.H{"total": 1, "limit": 50, "offset": 0})
	require.Equal(t, http.StatusOK, w.Code)

	var body response.SuccessBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data)
	require.NotNil(t, body.Meta)
}

func TestNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	response.NoContent(c)
	require.Equal(t, http.StatusNoContent, w.Code)
}
