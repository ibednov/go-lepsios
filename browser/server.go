package browser

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type pageFetcher interface {
	PageFetcher
	JSONCapturer
}

// Server exposes /health and internal fetch/capture endpoints.
type Server struct {
	pages pageFetcher
	token string
}

func NewServer(pages pageFetcher, token string) *Server {
	return &Server{
		pages: pages,
		token: strings.TrimSpace(token),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /internal/v1/fetch-in-page", s.handleFetchInPage)
	mux.HandleFunc("POST /internal/v1/capture-json", s.handleCaptureJSON)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type fetchRequest struct {
	WarmupURL  string `json:"warmup_url"`
	RequestURL string `json:"request_url"`
}

type fetchResponse struct {
	Body []byte `json:"body"`
	HTML []byte `json:"html"`
}

type captureRequest struct {
	PageURL string `json:"page_url"`
	URLGlob string `json:"url_glob"`
}

type captureResponse struct {
	Body []byte `json:"body"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) handleFetchInPage(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	var req fetchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := CheckAllowedURL(req.WarmupURL); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if err := CheckAllowedURL(req.RequestURL); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	snap, err := s.pages.FetchInPage(r.Context(), req.WarmupURL, req.RequestURL)
	if err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fetchResponse{Body: snap.Body, HTML: snap.HTML})
}

func (s *Server) handleCaptureJSON(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	var req captureRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := CheckAllowedURL(req.PageURL); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	body, err := s.pages.CaptureJSON(r.Context(), req.PageURL, req.URLGlob)
	if err != nil {
		writeError(w, statusFor(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, captureResponse{Body: body})
}

func (s *Server) authorized(w http.ResponseWriter, r *http.Request) bool {
	if s.token == "" {
		return true
	}
	got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
	if got != s.token {
		writeError(w, http.StatusUnauthorized, ErrUnauthorized.Error())
		return false
	}
	return true
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrHostNotAllowed):
		return http.StatusForbidden
	case errors.Is(err, ErrEmptyPageURL),
		errors.Is(err, ErrEmptyRequestURL),
		errors.Is(err, ErrEmptyGlob):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dest)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
