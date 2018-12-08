package scaler

type functionScaler interface {
	scaleFunctionToZero(string, string)
}

type metricReporter interface {
	reportMetric(metricEntry) error
}
