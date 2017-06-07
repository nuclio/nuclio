package rest

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/pressly/chi"
	"github.com/pressly/chi/docgen"
	"github.com/pressly/chi/middleware"
)

const (
	articleKey = "article_id"
)

var (
	srcs        []event_source.EventSource
	checkpoints = make(map[int]event_source.Checkpoint)
	rtr         *chi.Mux
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

func listHandler(w http.ResponseWriter, r *http.Request) {
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

func getID(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

func evtCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("PATH: %s\n", r.URL.Path)
		if srcs == nil {
			sendError(w, "nuclio not initialized", http.StatusInternalServerError)
			return
		}

		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil || id >= len(srcs) {
			sendError(w, "bad id", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), articleKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func evtHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := r.Context().Value(articleKey).(int)
	if !ok {
		sendError(w, "no ID", http.StatusBadRequest)
		return
	}
	reply := DataReply{
		Data: srcData(id, srcs[id]),
	}
	setCtype(w)
	json.NewEncoder(w).Encode(reply)
}

func srcStats(id int, src event_source.EventSource) map[string]interface{} {
	return map[string]interface{}{
		"type":       "event_statistics",
		"id":         id,
		"attributes": asMap(src.Stats()),
	}
}

func evtStatsHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := r.Context().Value(articleKey).(int)
	if !ok {
		sendError(w, "no ID", http.StatusBadRequest)
		return
	}
	reply := DataReply{
		Data: srcStats(id, srcs[id]),
	}
	setCtype(w)
	json.NewEncoder(w).Encode(reply)
}

func listStatsHandler(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	if srcs == nil {
		sendError(w, "nuclio not initialized", http.StatusInternalServerError)
		return
	}
	data := make([]map[string]interface{}, len(srcs))
	for id, src := range srcs {
		data[id] = srcStats(id, src)
	}
	setCtype(w)
	reply := DataReply{Data: data}
	enc.Encode(reply)
}

func setCtype(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/vnd.api+json")
}

// SetCtype is a middleware that set content type to JSON API content type
func SetCtype(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		setCtype(w)
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
	id, ok := r.Context().Value(articleKey).(int)
	if !ok {
		sendError(w, "no ID", http.StatusBadRequest)
		return
	}

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

	setCtype(w)
	reply := DataReply{Data: map[string]bool{"ok": true}}
	json.NewEncoder(w).Encode(reply)
}

func init() {
	es := chi.NewRouter()
	es.Use(middleware.StripSlashes)
	es.Get("/statistics", listStatsHandler)
	es.Route("/:id", func(r chi.Router) {
		r.Use(evtCtx)
		r.Get("/", evtHandler)
		r.Post("/", evtPostHandler)
		r.Get("/statistics", evtStatsHandler)
	})

	rtr = chi.NewRouter()
	rtr.Use(middleware.Recoverer)
	rtr.Use(SetCtype)

	rtr.Mount("/event_sources", es)
	docgen.PrintRoutes(rtr)
}

func StartHTTPD(addr string, esrcs []event_source.EventSource) {
	srcs = esrcs
	go http.ListenAndServe(addr, rtr)
}
