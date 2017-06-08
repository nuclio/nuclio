package rest

import (
	"context"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
)

const (
	idKey = "id"
)

var (
	srcs        []event_source.EventSource
	checkpoints = make(map[int]event_source.Checkpoint)
	rtr         *chi.Mux

	badIDError = errors.New("bad ID")
)

type ErrorMessage struct {
	Title string `json:"title"`
}

type ErrorReply struct {
	Errors []ErrorMessage `json:"errors"`
}

func sendError(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	err := &ErrorReply{
		Errors: []ErrorMessage{
			{Title: msg},
		},
	}
	enc.Encode(err)
}

type DataReply struct {
	Data interface{} `json:"data"`
}

func srcData(id int, src event_source.EventSource) map[string]interface{} {
	return map[string]interface{}{
		"type":       "event_source",
		"id":         id,
		"attributes": src.Config(),
	}
}

func listEventsHandler(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	if srcs == nil {
		sendError(w, "nuclio not initialized", http.StatusInternalServerError)
		return
	}
	data := make([]map[string]interface{}, len(srcs))
	for id, src := range srcs {
		data[id] = srcData(id, src)
	}
	reply := DataReply{Data: data}
	enc.Encode(reply)
}

func idCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if srcs == nil {
			sendError(w, "nuclio not initialized", http.StatusInternalServerError)
			return
		}

		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			sendError(w, "bad id", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), idKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type FetchFunc func(id int) (interface{}, error)

func newHandler(ff FetchFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(idKey).(int)
		data, err := ff(id)
		if err != nil {
			sendError(w, err.Error(), http.StatusBadRequest)
			return
		}
		reply := DataReply{
			Data: data,
		}
		json.NewEncoder(w).Encode(reply)
	})
}

func fetchEvent(id int) (interface{}, error) {
	if id >= len(srcs) {
		return nil, badIDError
	}

	return srcData(id, srcs[id]), nil
}

func srcStats(id int, src event_source.EventSource) map[string]interface{} {
	return map[string]interface{}{
		"type":       "event_statistics",
		"id":         id,
		"attributes": asMap(src.Stats()),
	}
}

func fetchEventStats(id int) (interface{}, error) {
	if id >= len(srcs) {
		return nil, badIDError
	}
	return srcStats(id, srcs[id]), nil
}

func listEventStatsHandler(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	if srcs == nil {
		sendError(w, "nuclio not initialized", http.StatusInternalServerError)
		return
	}
	data := make([]map[string]interface{}, len(srcs))
	for id, src := range srcs {
		data[id] = srcStats(id, src)
	}
	reply := DataReply{Data: data}
	enc.Encode(reply)
}

// SetCtype is a middleware that set content type to JSON API content type
func SetCtype(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func asMap(m *expvar.Map) map[string]interface{} {
	out := make(map[string]interface{})
	m.Do(func(kv expvar.KeyValue) {
		switch kv.Value.(type) {
		case *expvar.Int:
			out[kv.Key] = kv.Value.(*expvar.Int).Value()
		case *expvar.Float:
			out[kv.Key] = kv.Value.(*expvar.Float).Value()
		case *expvar.String:
			out[kv.Key] = kv.Value.(*expvar.String).Value()
		}
	})

	return out
}

type postData struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
	// TODO: Later switch to map[string]interface{}
	Attrs map[string]bool `json:"attributes"`
}

type postRequest struct {
	Data postData `json:"data"`
}

func evtPostHandler(w http.ResponseWriter, r *http.Request) {
	id := r.Context().Value(idKey).(int)

	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	var req postRequest
	if err := dec.Decode(&req); err != nil {
		sendError(w, "bad JSON", http.StatusBadRequest)
		return
	}

	if id != req.Data.ID {
		sendError(w, "id mismatch", http.StatusBadRequest)
		return
	}

	enabled, ok := req.Data.Attrs["enabled"]
	if !ok {
		sendError(w, "missing 'enabled' field", http.StatusBadRequest)
		return
	}

	var err error
	if enabled {
		cp := checkpoints[id]
		err = srcs[id].Start(cp)
	} else {
		var cp event_source.Checkpoint
		cp, err = srcs[id].Stop(false)
		if err != nil {
			checkpoints[id] = cp
		}
	}
	if err != nil {
		// TODO: Check if out fault?
		var verb string
		if enabled {
			verb = "start"
		} else {
			verb = "stop"
		}
		sendError(w, fmt.Sprintf("can't %s - %s", verb, err), http.StatusBadRequest)
		return
	}

	reply := DataReply{Data: map[string]bool{"ok": true}}
	json.NewEncoder(w).Encode(reply)
}

func workerData(id int, wrk *worker.Worker) map[string]interface{} {
	out := make(map[string]interface{})
	out["id"] = id
	out["type"] = "worker"
	ctx := wrk.Context()
	attrs := make(map[string]interface{})
	attrs["function_name"] = ctx.FunctionName
	attrs["function_version"] = ctx.FunctionVersion
	attrs["event_id"] = ctx.EventID
	out["attributes"] = attrs

	return out
}

func listWorkersHandler(w http.ResponseWriter, r *http.Request) {
	workers := worker.AllWorkers()
	data := make([]map[string]interface{}, len(workers))
	i := 0
	for id, worker := range workers {
		data[i] = workerData(id, worker)
		i++
	}
	reply := DataReply{Data: data}
	json.NewEncoder(w).Encode(reply)
}

func workerStats(id int, wrk *worker.Worker) map[string]interface{} {
	stats := wrk.Statistics()
	out := make(map[string]interface{})
	out["iterations"] = stats.Iterations
	out["items"] = stats.Items
	out["succeeded"] = stats.Succeeded
	out["failed"] = stats.Failed
	out["retry"] = stats.Retry
	out["duration"] = stats.Duration
	out["queued"] = stats.Queued
	out["start_time"] = stats.StartTime

	return out
}

func listWorkersStatsHandler(w http.ResponseWriter, r *http.Request) {
	workers := worker.AllWorkers()
	data := make([]map[string]interface{}, len(workers))
	i := 0
	for id, worker := range workers {
		data[i] = workerStats(id, worker)
		i++
	}
	reply := DataReply{Data: data}
	json.NewEncoder(w).Encode(reply)
}

func fetchWorker(id int) (interface{}, error) {
	wkr := worker.FindWorker(id)
	if wkr == nil {
		return nil, badIDError
	}

	return workerData(id, wkr), nil
}

func fetchWorkerStats(id int) (interface{}, error) {
	wkr := worker.FindWorker(id)
	if wkr == nil {
		return nil, badIDError
	}

	return workerStats(id, wkr), nil
}

func init() {
	es := chi.NewRouter()
	es.Get("/", listEventsHandler)
	es.Get("/statistics", listEventStatsHandler)
	es.Route("/:id", func(r chi.Router) {
		r.Use(idCtx)
		r.Get("/", newHandler(fetchEvent))
		r.Post("/", evtPostHandler)
		r.Get("/statistics", newHandler(fetchEventStats))
	})

	ws := chi.NewRouter()
	ws.Get("/", listWorkersHandler)
	ws.Get("/statistics", listWorkersStatsHandler)
	ws.Route("/:id", func(r chi.Router) {
		r.Use(idCtx)
		r.Get("/", newHandler(fetchWorker))
		r.Get("/statistics", newHandler(fetchWorkerStats))
	})

	rtr = chi.NewRouter()
	rtr.Use(middleware.Recoverer)
	rtr.Use(middleware.StripSlashes)
	rtr.Use(SetCtype)

	rtr.Mount("/event_sources", es)
	rtr.Mount("/workers", ws)
}

func StartHTTPD(addr string, esrcs []event_source.EventSource) {
	srcs = esrcs
	go http.ListenAndServe(addr, rtr)
}
