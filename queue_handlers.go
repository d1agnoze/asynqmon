package asynqmon

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hibiken/asynq"
)

// ****************************************************************************
// This file defines:
//   - http.Handler(s) for queue related endpoints
// ****************************************************************************

func newListQueuesHandlerFunc(inspector *asynq.Inspector, t *transformer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		qnames, err := inspector.Queues()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		snapshots := make([]*QueueStateSnapshot, len(qnames))
		for i, qname := range qnames {
			qinfo, err := inspector.GetQueueInfo(qname)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			snapshots[i] = t.toQueueStateSnapshot(qinfo)
		}
		payload := map[string]interface{}{"queues": snapshots}
		json.NewEncoder(w).Encode(payload)
	}
}

func newGetQueueHandlerFunc(inspector *asynq.Inspector, t *transformer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]

		payload := make(map[string]interface{})
		qinfo, err := inspector.GetQueueInfo(qname)
		if err != nil {
			// TODO: Check for queue not found error.
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		payload["current"] = t.toQueueStateSnapshot(qinfo)

		// TODO: make this n a variable
		data, err := inspector.History(qname, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var dailyStats []*DailyStats
		for _, s := range data {
			dailyStats = append(dailyStats, t.toDailyStats(s))
		}
		payload["history"] = dailyStats
		json.NewEncoder(w).Encode(payload)
	}
}

func newDeleteQueueHandlerFunc(inspector *asynq.Inspector, t *transformer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]
		if err := inspector.DeleteQueue(qname, false); err != nil {
			if errors.Is(err, asynq.ErrQueueNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if errors.Is(err, asynq.ErrQueueNotEmpty) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func newPauseQueueHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]
		if err := inspector.PauseQueue(qname); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func newResumeQueueHandlerFunc(inspector *asynq.Inspector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		qname := vars["qname"]
		if err := inspector.UnpauseQueue(qname); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type ListQueueStatsResponse struct {
	Stats map[string][]*DailyStats `json:"stats"`
}

func newListQueueStatsHandlerFunc(inspector *asynq.Inspector, t *transformer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		qnames, err := inspector.Queues()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := ListQueueStatsResponse{Stats: make(map[string][]*DailyStats)}
		const numdays = 90 // Get stats for the last 90 days.
		for _, qname := range qnames {
			stats, err := inspector.History(qname, numdays)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			resp.Stats[qname] = t.toDailyStatsList(stats)
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
