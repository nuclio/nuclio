package rest

import (
	"encoding/json"
	"net/http"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
)

var (
	srcs []event_source.EventSource
	rtr  *chi.Mux
)

func newError(msg string) interface{} {
	return struct {
		Errors []interface{} `json:"errors"`
	}{
		Errors: []interface{}{
			struct {
				Title string `json:"title"`
			}{msg},
		},
	}
}

func srcHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.api+json")
	enc := json.NewEncoder(w)
	if srcs == nil {
		w.WriteHeader(http.StatusInternalServerError)
		enc.Encode(newError("nuclio not initialized"))
		return
	}
	var reply = struct {
		Data []interface{} `json:"data"`
	}{}
	for id, src := range srcs {
		reply.Data = append(reply.Data, map[string]interface{}{
			"type":       "event_source",
			"id":         id,
			"attributes": src.Config(),
		})
	}
	enc.Encode(reply)
}

func init() {
	rtr = chi.NewRouter()
	rtr.Use(middleware.Recoverer)

	// Routes
	rtr.Get("/event_sources", srcHandler)
}

func StartHTTPD(addr string, esrcs []event_source.EventSource) {
	srcs = esrcs
	go http.ListenAndServe(addr, rtr)
}
