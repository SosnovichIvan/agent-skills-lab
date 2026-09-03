package app

import (
	"net/http"

	"benchmark.local/iam/internal/httpapi"
)

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *App) metrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	httpapi.WriteData(w, http.StatusOK, httpapi.MetricsData(a.Metrics))
}
