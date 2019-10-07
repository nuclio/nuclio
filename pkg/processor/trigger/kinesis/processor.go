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
	shardID        string
}

func (rpf *recordProcessorFactory) CreateProcessor() interfaces.IRecordProcessor {
	return rpf
}

func (rpf *recordProcessorFactory) Initialize(initializationInput *interfaces.InitializationInput) {
	rpf.Logger.InfoWith("Kinesis record processor was initialized", "initializationInput", initializationInput)
	rpf.shardID = initializationInput.ShardId
}

func (rpf *recordProcessorFactory) ProcessRecords(processRecordsInput *interfaces.ProcessRecordsInput) {
	rpf.Logger.DebugWith("Kinesis record processor received records", "shardID", rpf.shardID, "processRecordsInput", processRecordsInput)

	for _, record := range processRecordsInput.Records {
		event := Event{
			body: record.Data,
		}
		rpf.kinesisTrigger.SubmitEventToWorker(nil, rpf.worker, &event) // nolint: errcheck
	}

	lastSequenceNumber := processRecordsInput.Records[len(processRecordsInput.Records)-1].SequenceNumber
	err := processRecordsInput.Checkpointer.Checkpoint(lastSequenceNumber)
	if err != nil {
		rpf.Logger.ErrorWith("Failed to save checkpoint", "shardID", rpf.shardID, rpf.shardID, "processRecordsInput", processRecordsInput)
	}
}

func (rpf *recordProcessorFactory) Shutdown(shutdownInput *interfaces.ShutdownInput) {
	rpf.Logger.InfoWith("Kinesis record processor was shutdown", "shutdownInput", shutdownInput)
}
