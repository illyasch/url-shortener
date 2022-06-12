// Package handlers manages the different versions of the API.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/illyasch/url-shortener/pkg/business/shortener"
	"github.com/illyasch/url-shortener/pkg/data/database"
)

// APIConfig contains all the mandatory systems required by handlers.
type APIConfig struct {
	Log *zap.SugaredLogger
	DB  *sqlx.DB
}

type errorResponse struct {
	Error string `json:"error"`
}

// Router constructs a http.Handler with all application routes defined.
func (cfg APIConfig) Router() http.Handler {
	store := shortener.New(cfg.DB)

	router := mux.NewRouter()
	router.HandleFunc("/{code}", cfg.handleExpand(store)).Methods(http.MethodGet)
	router.HandleFunc("/shorten", cfg.handleShorten(store)).Methods(http.MethodPost)
	router.HandleFunc("/readiness", cfg.handleReadiness).Methods(http.MethodGet)
	router.HandleFunc("/liveness", cfg.handleLiveness).Methods(http.MethodGet)

	return router
}

func (cfg APIConfig) handleShorten(store shortener.Engine) http.HandlerFunc {
	const URLMinLen = 9
	type shortenResponse struct {
		Code string `json:"code"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		url := r.FormValue("url")
		if len(url) < URLMinLen {
			err := errors.New("input URL is incorrect")

			cfg.respond(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			cfg.Log.Errorw("shorten", "ERROR", fmt.Errorf("validation url(%s): %w", url, err))
			return
		}

		code, err := store.Shorten(r.Context(), url)
		if err != nil {
			cfg.respond(w, http.StatusInternalServerError, errorResponse{
				Error: http.StatusText(http.StatusInternalServerError),
			})
			cfg.Log.Errorw("shorten", "ERROR", fmt.Errorf("shortening: %w", err))
			return
		}

		cfg.respond(w, http.StatusOK, shortenResponse{Code: code})
		cfg.Log.Infow("shorten", "statusCode", http.StatusOK, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
	}
}

func (cfg APIConfig) handleExpand(store shortener.Engine) func(w http.ResponseWriter, r *http.Request) {
	const CodeMinLen = 6
	type expandResponse struct {
		URL string `json:"url"`
	}
	inputErr := errors.New("input URL code is incorrect")

	return func(w http.ResponseWriter, r *http.Request) {
		code, ok := mux.Vars(r)["code"]
		if !ok || len(code) < CodeMinLen {
			cfg.respond(w, http.StatusBadRequest, errorResponse{Error: inputErr.Error()})
			cfg.Log.Errorw("expand", "ERROR", fmt.Errorf("validation code(%s): %w", code, inputErr))
			return
		}

		url, err := store.Expand(r.Context(), code)
		if err == nil {
			cfg.respond(w, http.StatusOK, expandResponse{URL: url})
			cfg.Log.Infow("expand", "statusCode", http.StatusOK, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
			return
		}

		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, shortener.DecodeErr):
			status = http.StatusBadRequest
			err = fmt.Errorf("validation code(%s): %w", code, inputErr)

		case errors.Is(err, sql.ErrNoRows):
			status = http.StatusNotFound
			err = fmt.Errorf("not found code(%s)", code)
		}

		cfg.respond(w, status, errorResponse{
			Error: http.StatusText(status),
		})
		cfg.Log.Errorw("expand", "ERROR", fmt.Errorf("shortening: %w", err))
		return
	}
}

// handleReadiness checks if the database is ready and if not will return a 500 status.
func (cfg APIConfig) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	status := "ok"
	statusCode := http.StatusOK
	if err := database.StatusCheck(ctx, cfg.DB); err != nil {
		status = "db not ready"
		statusCode = http.StatusInternalServerError
		cfg.Log.Errorw("readiness", "ERROR", fmt.Errorf("status check: %w", err))
	}

	data := struct {
		Status string `json:"status"`
	}{
		Status: status,
	}

	cfg.respond(w, statusCode, data)
	cfg.Log.Infow("readiness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
}

// handleLiveness returns simple status info if the service is alive. If the
// app is deployed to a Kubernetes cluster, it will also return pod, node, and
// namespace details via the Downward API. The Kubernetes environment variables
// need to be set within your Pod/Deployment manifest.
func (cfg APIConfig) handleLiveness(w http.ResponseWriter, r *http.Request) {
	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	data := struct {
		Status    string `json:"status,omitempty"`
		Build     string `json:"build,omitempty"`
		Host      string `json:"host,omitempty"`
		Pod       string `json:"pod,omitempty"`
		PodIP     string `json:"podIP,omitempty"`
		Node      string `json:"node,omitempty"`
		Namespace string `json:"namespace,omitempty"`
	}{
		Status:    "up",
		Host:      host,
		Pod:       os.Getenv("KUBERNETES_PODNAME"),
		PodIP:     os.Getenv("KUBERNETES_NAMESPACE_POD_IP"),
		Node:      os.Getenv("KUBERNETES_NODENAME"),
		Namespace: os.Getenv("KUBERNETES_NAMESPACE"),
	}

	statusCode := http.StatusOK
	cfg.respond(w, statusCode, data)
	cfg.Log.Infow("liveness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)
}

func (cfg APIConfig) respond(w http.ResponseWriter, statusCode int, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		cfg.Log.Errorw("respond", "ERROR", fmt.Errorf("json marshal: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if _, err := w.Write(jsonData); err != nil {
		cfg.Log.Errorw("respond", "ERROR", fmt.Errorf("write output: %w", err))
		return
	}
}
