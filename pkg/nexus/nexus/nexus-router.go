package nexus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
)

// NexusRouter is the router for the nexus
type NexusRouter struct {
	Router chi.Router
	Nexus  *Nexus
}

// NewNexusRouter creates a new nexus router
func NewNexusRouter(nexus *Nexus) *NexusRouter {
	return &NexusRouter{
		Router: chi.NewRouter(),
		Nexus:  nexus,
	}
}

const (
	SCHEDULER_BASE_PATH = "/scheduler"
)

// Initialize initializes the nexus router
func (nexusRouter *NexusRouter) Initialize() {
	nexusRouter.Router.Post(SCHEDULER_BASE_PATH+"/{schedulerName}/start", nexusRouter.StartScheduler)
	nexusRouter.Router.Post(SCHEDULER_BASE_PATH+"/{schedulerName}/stop", nexusRouter.StopScheduler)
	nexusRouter.Router.Get(SCHEDULER_BASE_PATH, nexusRouter.GetAllSchedulersWithStatus)
	nexusRouter.Router.Put("/config", nexusRouter.ModifyNexusConfig)

	println("NexusRouter initialized")
}

// GetAllSchedulersWithStatus allows to get all schedulers with their status via GET request
func (nexusRouter *NexusRouter) GetAllSchedulersWithStatus(w http.ResponseWriter, r *http.Request) {
	schedulers := nexusRouter.Nexus.GetAllSchedulers()

	schedulerList := make([]map[string]interface{}, 0)
	for name, scheduler := range schedulers {
		schedulerMap := map[string]interface{}{
			"name":   name,
			"status": scheduler.GetStatus(),
		}
		schedulerList = append(schedulerList, schedulerMap)
	}

	schedulerJSON, err := json.Marshal(schedulerList)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(err.Error()))
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(schedulerJSON)
	if err != nil {
		fmt.Println(err)
	}
}

// StartScheduler allows to start a scheduler via POST request
func (nexusRouter *NexusRouter) StartScheduler(w http.ResponseWriter, r *http.Request) {

	schedulerName := chi.URLParam(r, "schedulerName")

	if _, ok := nexusRouter.Nexus.schedulers[schedulerName]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		unhandledWriteString(w, fmt.Sprintf("Scheduler %s does not exist", schedulerName))
		return
	}

	if nexusRouter.Nexus.schedulers[schedulerName].GetStatus() == interfaces.Running {
		w.WriteHeader(http.StatusOK)
		unhandledWriteString(w, fmt.Sprintf("Scheduler %s is already running", schedulerName))
		return
	}

	nexusRouter.Nexus.StartScheduler(schedulerName)

	w.WriteHeader(http.StatusOK)
	unhandledWriteString(w, fmt.Sprintf("Scheduler %s started", schedulerName))
}

// StopScheduler allows to stop a scheduler via POST request
func (nexusRouter *NexusRouter) StopScheduler(w http.ResponseWriter, r *http.Request) {
	schedulerName := chi.URLParam(r, "schedulerName")

	if _, ok := nexusRouter.Nexus.schedulers[schedulerName]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		unhandledWriteString(w, fmt.Sprintf("Scheduler %s does not exist", schedulerName))
		return
	}

	if nexusRouter.Nexus.schedulers[schedulerName].GetStatus() == interfaces.Stopped {
		w.WriteHeader(http.StatusOK)
		unhandledWriteString(w, fmt.Sprintf("Scheduler %s is already stopped", schedulerName))
		return
	}

	nexusRouter.Nexus.StopScheduler(schedulerName)

	w.WriteHeader(http.StatusOK)
	unhandledWriteString(w, fmt.Sprintf("Scheduler %s stopped", schedulerName))
}

// ModifyNexusConfig allows to modify the nexus config via PUT request
func (nexusRouter *NexusRouter) ModifyNexusConfig(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if maxParallelRequests := query.Get("maxParallelRequests"); maxParallelRequests != "" {
		maxParallelRequestsInt, err := strconv.ParseInt(maxParallelRequests, 10, 32)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			unhandledWriteString(w, fmt.Sprintf("Invalid value for maxParallelRequests: %s", maxParallelRequests))
			return
		}
		nexusRouter.Nexus.SetMaxParallelRequests(int32(maxParallelRequestsInt))

		w.WriteHeader(http.StatusAccepted)
		unhandledWriteString(w, fmt.Sprintf("Max parallel requests set to %s", maxParallelRequests))
	}

	w.WriteHeader(http.StatusOK)
}

// unhandledWriteString writes a string to the response writer
func unhandledWriteString(w http.ResponseWriter, responseString string) {
	_, writeErr := w.Write([]byte(responseString))
	if writeErr != nil {
		return
	}
}
