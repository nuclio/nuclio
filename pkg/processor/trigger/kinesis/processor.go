package kinesis

import (
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	"github.com/vmware/vmware-go-kcl/clientlibrary/interfaces"
)

type recordProcessorFactory struct {
	Logger         logger.Logger
	worker         *worker.Worker
	kinesisTrigger *kinesis
}

func (rpf *recordProcessorFactory) CreateProcessor() interfaces.IRecordProcessor {
	return rpf
}

func (rpf *recordProcessorFactory) Initialize(initializationInput *interfaces.InitializationInput) {
	rpf.Logger.InfoWith("Kinesis record processor was initialized", "initializationInput", initializationInput)
}

func (rpf *recordProcessorFactory) ProcessRecords(processRecordsInput *interfaces.ProcessRecordsInput) {
	rpf.Logger.DebugWith("Kinesis record processor received records", "processRecordsInput", processRecordsInput)
	for _, record := range processRecordsInput.Records {
		event := Event{
			body: record.Data,
		}
		rpf.kinesisTrigger.SubmitEventToWorker(nil, rpf.worker, &event) // nolint: errcheck
	}
}

func (rpf *recordProcessorFactory) Shutdown(shutdownInput *interfaces.ShutdownInput) {
	rpf.Logger.InfoWith("Kinesis record processor was shutdown", "shutdownInput", shutdownInput)
}
