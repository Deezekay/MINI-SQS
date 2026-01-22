package api

import (
	"encoding/json"
	"net/http"
	"taskqueue/internal/queue"
)

type PollRequest struct {
	WorkerID string `json:"worker_id"`
}

func PollHandler(mgr *queue.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PollRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		task, err := mgr.Poll(req.WorkerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if task == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		json.NewEncoder(w).Encode(task)
	}
}
