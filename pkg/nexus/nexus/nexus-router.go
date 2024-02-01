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
	LOAD_BALANCER_PATH  = "/load-balancer"
	SCHEDULER_BASE_PATH = "/scheduler"
	START               = "/start"
	STOP                = "/stop"
)

// Initialize initializes the nexus router
func (nexusRouter *NexusRouter) Initialize() {
	nexusRouter.Router.Post(SCHEDULER_BASE_PATH+"/{schedulerName}"+START, nexusRouter.StartScheduler)
	nexusRouter.Router.Post(SCHEDULER_BASE_PATH+"/{schedulerName}"+STOP, nexusRouter.StopScheduler)
	nexusRouter.Router.Get(SCHEDULER_BASE_PATH, nexusRouter.GetAllSchedulersWithStatus)
	nexusRouter.Router.Put(LOAD_BALANCER_PATH, nexusRouter.modifyLoadBalancer)
	nexusRouter.Router.Post(LOAD_BALANCER_PATH+START, nexusRouter.startLoadBalancer)
	nexusRouter.Router.Post(LOAD_BALANCER_PATH+STOP, nexusRouter.stopLoadBalancer)

	fmt.Println("NexusRouter initialized")
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

// modifyLoadBalancer allows to modify the load balancer via PUT request
func (nexusRouter *NexusRouter) modifyLoadBalancer(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if targetLoadCPU := query.Get("targetLoadCPU"); targetLoadCPU != "" {
		targetLoadCPU, err := strconv.ParseFloat(targetLoadCPU, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			unhandledWriteString(w, fmt.Sprintf("Invalid value for targetLoadCPU: %f", targetLoadCPU))
			return
		}
		nexusRouter.Nexus.SetTargetLoadCPU(targetLoadCPU)

		w.WriteHeader(http.StatusAccepted)
		unhandledWriteString(w, fmt.Sprintf("Target CPU load set to %.1f\n", targetLoadCPU))
	}

	if targetLoadMemory := query.Get("targetLoadMemory"); targetLoadMemory != "" {
		targetLoadMemory, err := strconv.ParseFloat(targetLoadMemory, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			unhandledWriteString(w, fmt.Sprintf("Invalid value for targetLoadMemory: %f\n", targetLoadMemory))
			return
		}
		nexusRouter.Nexus.SetTargetLoadMemory(targetLoadMemory)

		w.WriteHeader(http.StatusAccepted)
		unhandledWriteString(w, fmt.Sprintf("Target memory load set to %.1f\n", targetLoadMemory))
	}

	if maxParallelRequests := query.Get("maxParallelRequests"); maxParallelRequests != "" {
		maxParallelRequestsInt, err := strconv.ParseInt(maxParallelRequests, 10, 32)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			unhandledWriteString(w, fmt.Sprintf("Invalid value for maxParallelRequests: %s\n", maxParallelRequests))
			return
		}
		nexusRouter.Nexus.SetMaxParallelRequests(int32(maxParallelRequestsInt))

		w.WriteHeader(http.StatusAccepted)
		unhandledWriteString(w, fmt.Sprintf("Max parallel requests set to %s\n", maxParallelRequests))
	}

	if limitMaxParallelRequests := query.Get("limitMaxParallelRequests"); limitMaxParallelRequests != "" {
		limitMaxParallelRequestsBool, err := strconv.ParseInt(limitMaxParallelRequests, 10, 32)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			unhandledWriteString(w, fmt.Sprintf("Invalid value for limitMaxParallelRequests: %s\n", limitMaxParallelRequests))
			return
		}
		nexusRouter.Nexus.SetLimitMaxParallelRequests(int(limitMaxParallelRequestsBool))

		w.WriteHeader(http.StatusAccepted)
		unhandledWriteString(w, fmt.Sprintf("Limit max parallel requests set to %s\n", limitMaxParallelRequests))
	}

	w.WriteHeader(http.StatusOK)
}

// StartLoadBalancer allows to start the load balancer via POST request
func (nexusRouter *NexusRouter) startLoadBalancer(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	unhandledWriteString(w, "LoadBalancer started")
}

// StopLoadBalancer allows to stop the load balancer via POST request
func (nexusRouter *NexusRouter) stopLoadBalancer(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	unhandledWriteString(w, "LoadBalancer stopped")
}

// unhandledWriteString writes a string to the response writer
func unhandledWriteString(w http.ResponseWriter, responseString string) {
	_, writeErr := w.Write([]byte(responseString))
	if writeErr != nil {
		return
	}
}
