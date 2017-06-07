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
	srcs []event_source.EventSource
	rtr  *chi.Mux
)

type ErrorMessage struct {
	Title string `json:"title"`
}

type ErrorReply struct {
	Errors []ErrorMessage `json:"errors"`
}

func newError(msg string) *ErrorReply {
	return &ErrorReply{
		Errors: []ErrorMessage{
			{Title: msg},
		},
	}
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
		w.WriteHeader(http.StatusInternalServerError)
		enc.Encode(newError("nuclio not initialized"))
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
		enc := json.NewEncoder(w)

		if srcs == nil {
			w.WriteHeader(http.StatusInternalServerError)
			enc.Encode(newError("nuclio not initialized"))
			return
		}

		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil || id >= len(srcs) {
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(newError("bad id"))
			return
		}

		ctx := context.WithValue(r.Context(), articleKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func evtHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := r.Context().Value(articleKey).(int)
	if !ok {
		http.Error(w, "no ID", http.StatusBadRequest)
		return
	}
	reply := DataReply{
		Data: srcData(id, srcs[id]),
	}
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
		http.Error(w, "no ID", http.StatusBadRequest)
		return
	}
	reply := DataReply{
		Data: srcStats(id, srcs[id]),
	}
	json.NewEncoder(w).Encode(reply)
}

func listStatsHandler(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	if srcs == nil {
		w.WriteHeader(http.StatusInternalServerError)
		enc.Encode(newError("nuclio not initialized"))
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

func init() {
	es := chi.NewRouter()
	es.Use(middleware.StripSlashes)
	es.Get("/statistics", listStatsHandler)
	es.Route("/:id", func(r chi.Router) {
		r.Use(evtCtx)
		r.Get("/", evtHandler)
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
