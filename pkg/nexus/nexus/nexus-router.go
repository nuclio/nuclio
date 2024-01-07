package nexus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/interfaces"
)

type NexusRouter struct {
	Router chi.Router
	Nexus  *Nexus
}

func NewNexusRouter(nexus *Nexus) *NexusRouter {
	return &NexusRouter{
		Router: chi.NewRouter(),
		Nexus:  nexus,
	}
}

func (nexusRouter *NexusRouter) Initialize() {
	nexusRouter.Router.Post("/scheduler/{schedulerName}/start", nexusRouter.StartScheduler)
	nexusRouter.Router.Post("/scheduler/{schedulerName}/stop", nexusRouter.StopScheduler)
	nexusRouter.Router.Get("/scheduler", nexusRouter.GetAllSchedulersWithStatus)
	nexusRouter.Router.Put("/nexus-config", nexusRouter.ModifyNexusConfig)

	println("NexusRouter initialized")
}

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
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(schedulerJSON)
}

func (nexusRouter *NexusRouter) StartScheduler(w http.ResponseWriter, r *http.Request) {

	schedulerName := chi.URLParam(r, "schedulerName")

	if _, ok := nexusRouter.Nexus.schedulers[schedulerName]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Scheduler %s does not exist", schedulerName)))
		return
	}

	if nexusRouter.Nexus.schedulers[schedulerName].GetStatus() == interfaces.Running {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Scheduler %s is already running", schedulerName)))
		return
	}

	nexusRouter.Nexus.StartScheduler(schedulerName)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Scheduler %s started", schedulerName)))

}

func (nexusRouter *NexusRouter) StopScheduler(w http.ResponseWriter, r *http.Request) {
	schedulerName := chi.URLParam(r, "schedulerName")

	if _, ok := nexusRouter.Nexus.schedulers[schedulerName]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Scheduler %s does not exist", schedulerName)))
		return
	}

	if nexusRouter.Nexus.schedulers[schedulerName].GetStatus() == interfaces.Stopped {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Scheduler %s is already stopped", schedulerName)))
		return
	}

	nexusRouter.Nexus.StopScheduler(schedulerName)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Scheduler %s stopped", schedulerName)))
}

func (nexusRouter *NexusRouter) ModifyNexusConfig(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if maxParallelRequests := query.Get("maxParallelRequests"); maxParallelRequests != "" {
		maxParallelRequestsInt, err := strconv.ParseInt(maxParallelRequests, 10, 32)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("Invalid value for maxParallelRequests: %s", maxParallelRequests)))
			return
		}
		nexusRouter.Nexus.SetMaxParallelRequests(int32(maxParallelRequestsInt))

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(fmt.Sprintf("Max parallel requests set to %s", maxParallelRequests)))
	}

	w.WriteHeader(http.StatusOK)
}
