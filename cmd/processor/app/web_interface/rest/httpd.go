package rest

import (
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
	// used in request context to pass current ID
	idKey = "id"
)

var (
	// Event sources - set by Processor.Start
	srcs []event_source.EventSource
	// Checkpoints for event sources (for start/stop)
	checkpoints = make(map[int]event_source.Checkpoint)
	rtr         *chi.Mux

	badIDError = errors.New("bad ID")
)

// ErrorMessage is JSON for single error
type ErrorMessage struct {
	Title string `json:"title"`
}

// ErrorReply is reply when we have error
type ErrorReply struct {
	Errors []ErrorMessage `json:"errors"`
}

// sendError is like http.Error only in JSON API
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

// DataReply is reply for data
type DataReply struct {
	Data interface{} `json:"data"`
}

// srcData return data for given event source
func srcData(id int, src event_source.EventSource) map[string]interface{} {
	return map[string]interface{}{
		"type":       "event_source",
		"id":         id,
		"attributes": src.Config(),
	}
}

// listEventsHandler return list of all events
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

// Function that fetches data by ID
type FetchFunc func(id int) (interface{}, error)

// objFromReq returns object form ID in request
func objFromReq(r *http.Request, fetch FetchFunc) (interface{}, error) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		return nil, err
	}
	obj, err := fetch(id)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// newHandler creates a handler with specific fetch functions
func newHandler(ff FetchFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := objFromReq(r, ff)
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

// fetchEvent returns event data for id
func fetchEvent(id int) (interface{}, error) {
	if id >= len(srcs) {
		return nil, badIDError
	}

	return srcData(id, srcs[id]), nil
}

// srcStats return statistics for source
func srcStats(id int, src event_source.EventSource) map[string]interface{} {
	return map[string]interface{}{
		"type": "event_statistics",
		"id":   id,
		//"attributes": asMap(src.Stats()),
	}
}

// fetchEventStats return event statistics for ID
func fetchEventStats(id int) (interface{}, error) {
	if id >= len(srcs) {
		return nil, badIDError
	}
	return srcStats(id, srcs[id]), nil
}

// listEventStatsHandler return statistics for all events
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

// asMap translates expvar.Map to map[string]interface{} so it can be sent as
// JSON
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

// postData is data sent by client in POST request
type postData struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
	// TODO: Later switch to map[string]interface{}
	Attrs map[string]bool `json:"attributes"`
}

// postRequest is JSON request by client
type postRequest struct {
	Data postData `json:"data"`
}

// evtPostHandler handles POST request
// (currently start/stop)
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

// workerData return data for worker
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

// listWorkers returns data for all workers
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

// workerStats return statistics for worker
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

// listWorkersStatsHandler return statistics for all workers
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

// fetchWorker is a FetchFunc that return data for worker by id
func fetchWorker(id int) (interface{}, error) {
	wkr := worker.FindWorker(id)
	if wkr == nil {
		return nil, badIDError
	}

	return workerData(id, wkr), nil
}

// fetchWorkerStats is a FetchFunc that return statics for worker by id
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
		r.Get("/", newHandler(fetchEvent))
		r.Post("/", evtPostHandler)
		r.Get("/statistics", newHandler(fetchEventStats))
	})

	ws := chi.NewRouter()
	ws.Get("/", listWorkersHandler)
	ws.Get("/statistics", listWorkersStatsHandler)
	ws.Route("/:id", func(r chi.Router) {
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

// StartHTTPD starts the server on "addr" in a goroutine
func StartHTTPD(addr string, esrcs []event_source.EventSource) {
	srcs = esrcs
	go http.ListenAndServe(addr, rtr)
}
