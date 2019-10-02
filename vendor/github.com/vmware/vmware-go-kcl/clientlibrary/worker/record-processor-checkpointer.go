/*
 * Copyright (c) 2018 VMware, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
 * associated documentation files (the "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished to do
 * so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial
 * portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
 * NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */
package worker

import (
	"github.com/aws/aws-sdk-go/aws"

	chk "github.com/vmware/vmware-go-kcl/clientlibrary/checkpoint"
	kcl "github.com/vmware/vmware-go-kcl/clientlibrary/interfaces"
	par "github.com/vmware/vmware-go-kcl/clientlibrary/partition"
)

type (

	/* Objects of this class are prepared to checkpoint at a specific sequence number. They use an
	 * IRecordProcessorCheckpointer to do the actual checkpointing, so their checkpoint is subject to the same 'didn't go
	 * backwards' validation as a normal checkpoint.
	 */
	PreparedCheckpointer struct {
		pendingCheckpointSequenceNumber *kcl.ExtendedSequenceNumber
		checkpointer                    kcl.IRecordProcessorCheckpointer
	}

	/**
	 * This class is used to enable RecordProcessors to checkpoint their progress.
	 * The Amazon Kinesis Client Library will instantiate an object and provide a reference to the application
	 * RecordProcessor instance. Amazon Kinesis Client Library will create one instance per shard assignment.
	 */
	RecordProcessorCheckpointer struct {
		shard      *par.ShardStatus
		checkpoint chk.Checkpointer
	}
)

func NewRecordProcessorCheckpoint(shard *par.ShardStatus, checkpoint chk.Checkpointer) kcl.IRecordProcessorCheckpointer {
	return &RecordProcessorCheckpointer{
		shard:      shard,
		checkpoint: checkpoint,
	}
}

func (pc *PreparedCheckpointer) GetPendingCheckpoint() *kcl.ExtendedSequenceNumber {
	return pc.pendingCheckpointSequenceNumber
}

func (pc *PreparedCheckpointer) Checkpoint() error {
	return pc.checkpointer.Checkpoint(pc.pendingCheckpointSequenceNumber.SequenceNumber)
}

func (rc *RecordProcessorCheckpointer) Checkpoint(sequenceNumber *string) error {
	rc.shard.Mux.Lock()

	// checkpoint the last sequence of a closed shard
	if sequenceNumber == nil {
		rc.shard.Checkpoint = chk.SHARD_END
	} else {
		rc.shard.Checkpoint = aws.StringValue(sequenceNumber)
	}

	rc.shard.Mux.Unlock()
	return rc.checkpoint.CheckpointSequence(rc.shard)
}

func (rc *RecordProcessorCheckpointer) PrepareCheckpoint(sequenceNumber *string) (kcl.IPreparedCheckpointer, error) {
	return &PreparedCheckpointer{}, nil

}
