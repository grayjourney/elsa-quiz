package handler

import (
	"net/http"

	"github.com/gray/elsa-quiz/web"
)

// handleIndex serves the embedded single-page web client. The "GET /" route is a
// catch-all, so anything other than the exact root is a 404 (the SPA has no
// server-side routes of its own).
func (a *API) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(web.Index)
}
