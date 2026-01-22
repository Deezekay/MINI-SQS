package api

import (
	"encoding/json"
	"net/http"
	"taskqueue/internal/queue"
)

type AckRequest struct {
	TaskID   string `json:"task_id"`
	WorkerID string `json:"worker_id"`
}

func AckHandler(mgr *queue.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if err := mgr.Ack(req.TaskID, req.WorkerID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
