package api

import (
	"encoding/json"
	"net/http"
	"taskqueue/internal/queue"
)

type MetricsResponse struct {
	Total     int `json:"total_tasks"`
	Pending   int `json:"pending_tasks"`
	InFlight  int `json:"in_flight_tasks"`
	Completed int `json:"completed_tasks"`
	Retries   int `json:"total_retries"`
}

func MetricsHandler(mgr *queue.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		total, pending, inFlight, done, retries := mgr.GetStats()

		resp := MetricsResponse{
			Total:     total,
			Pending:   pending,
			InFlight:  inFlight,
			Completed: done,
			Retries:   retries,
		}

		json.NewEncoder(w).Encode(resp)
	}
}
