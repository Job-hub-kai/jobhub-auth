package health

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type response struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

func RunHTTP(log *zap.Logger) {
	mux := http.NewServeMux()

	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response{
			Status:    "ok",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response{
			Status:    "ready",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	})

	log.Info("health server listening", zap.String("addr", ":8080"))
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Error("health server error", zap.Error(err))
	}

}
