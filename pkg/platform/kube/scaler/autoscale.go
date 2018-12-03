package scaler

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"sync"
	"time"
)

type Autoscale struct {
	logger       logger.Logger
	namespace    string
	statsChannel <-chan entry
	stats        map[statKey][]entry
	scaler       Scaler
	statsMutex   sync.Mutex
}

func NewAutoScaler(parentLogger logger.Logger,
	namespace string,
	ch <-chan entry,
	scaler Scaler) *Autoscale {
	return &Autoscale{
		logger:       parentLogger.GetChild("autoscale"),
		namespace:    namespace,
		statsChannel: ch,
		stats:        make(map[statKey][]entry),
		scaler:       scaler,
	}
}

func (as *Autoscale) CheckToScale(t time.Time, functions map[statKey]*functionconfig.Spec) {
	as.logger.Debug("Checking to scale")
	for key, stats := range as.stats {
		if _, found := functions[key]; !found {
			continue
		}

		if functions[key].Metrics == nil {
			as.logger.Debug("No metric resources defined for the function")
			continue
		}

		for _, metric := range functions[key].Metrics {

			if metric.SourceType != key.sourceType {
				continue
			}

			window, err := time.ParseDuration(metric.WindowSize)
			if err != nil {
				as.logger.DebugWith("Failed to parse window size for function", "functionName", key.functionName)
				continue
			}

			// this will give out the greatest delta
			var minStat *entry
			for idx, stat := range stats {

				if stat.value <= metric.ThresholdValue && minStat == nil {
					minStat = &stats[idx]
				} else if stat.value > metric.ThresholdValue {
					minStat = nil
				}
			}

			if minStat != nil && t.Sub(minStat.timestamp) > window {
				as.logger.DebugWith("Stat is below threshold and passed the window",
					"statValue", minStat.value,
					"function", minStat.functionName,
					"threshold", metric.ThresholdValue,
					"deltaSeconds", t.Sub(minStat.timestamp).Seconds(),
					"windowSize", metric.WindowSize)
				as.scaler.Scale(key.namespace, key.functionName, 0)
				as.removeEntry(key)
			} else {
				if minStat != nil {
					as.logger.DebugWith("Function still in window",
						"functionName", key.functionName,
						"value", minStat.value,
						"threshold", metric.ThresholdValue,
						"deltaSeconds", t.Sub(minStat.timestamp).Seconds(),
						"windowSize", metric.WindowSize)
				} else {
					as.logger.Debug("Function is above threshold")
				}
				//TODO clean all metrics with time earlier than now minus window size
			}
		}
	}
}

func (as *Autoscale) addEntry(key statKey, entry entry) {
	as.statsMutex.Lock()
	defer as.statsMutex.Unlock()
	as.stats[key] = append(as.stats[key], entry)
}

func (as *Autoscale) removeEntry(key statKey) {
	as.statsMutex.Lock()
	defer as.statsMutex.Unlock()
	delete(as.stats, key)
}

func (as *Autoscale) start() {
	go func() {
		for entry := range as.statsChannel {
			as.logger.Debug("Got stat")
			key := statKey{
				sourceType: entry.sourceType,
				namespace:    entry.namespace,
				functionName: entry.functionName,
			}
			as.addEntry(key, entry)
		}
	}()
}
