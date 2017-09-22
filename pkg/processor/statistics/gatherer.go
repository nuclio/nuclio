package statistics

// a reflection of an object in the processor (e.g. event source, runtime, worker) that holds promethues
// metrics. when Gather() is called, the resource is queried for its primitive statistics. this way we decouple
// prometheus metrics from the fast path
type Gatherer interface {
	Gather() error
}
