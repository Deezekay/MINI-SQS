package api

import (
	"encoding/json"
	"net/http"
	"taskqueue/internal/queue"
)

type EnqueueRequest struct {
	ID      string `json:"task_id"`
	Payload string `json:"payload"`
}

func EnqueueHandler(mgr *queue.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req EnqueueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if err := mgr.Enqueue(req.ID, req.Payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
