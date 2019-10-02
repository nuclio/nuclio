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
// The implementation is derived from https://github.com/awslabs/amazon-kinesis-client
/*
 * Copyright 2014-2015 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Amazon Software License (the "License").
 * You may not use this file except in compliance with the License.
 * A copy of the License is located at
 *
 * http://aws.amazon.com/asl/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
 * express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */
package interfaces

type (
	// IRecordProcessor is the interface for some callback functions invoked by KCL will
	// The main task of using KCL is to provide implementation on IRecordProcessor interface.
	// Note: This is exactly the same interface as Amazon KCL IRecordProcessor v2
	IRecordProcessor interface {
		/**
		 * Invoked by the Amazon Kinesis Client Library before data records are delivered to the RecordProcessor instance
		 * (via processRecords).
		 *
		 * @param initializationInput Provides information related to initialization
		 */
		Initialize(initializationInput *InitializationInput)

		/**
		 * Process data records. The Amazon Kinesis Client Library will invoke this method to deliver data records to the
		 * application.
		 * Upon fail over, the new instance will get records with sequence number > checkpoint position
		 * for each partition key.
		 *
		 * @param processRecordsInput Provides the records to be processed as well as information and capabilities related
		 *        to them (eg checkpointing).
		 */
		ProcessRecords(processRecordsInput *ProcessRecordsInput)

		/**
		 * Invoked by the Amazon Kinesis Client Library to indicate it will no longer send data records to this
		 * RecordProcessor instance.
		 *
		 * <h2><b>Warning</b></h2>
		 *
		 * When the value of {@link ShutdownInput#getShutdownReason()} is
		 * {@link com.amazonaws.services.kinesis.clientlibrary.lib.worker.ShutdownReason#TERMINATE} it is required that you
		 * checkpoint. Failure to do so will result in an IllegalArgumentException, and the KCL no longer making progress.
		 *
		 * @param shutdownInput
		 *            Provides information and capabilities (eg checkpointing) related to shutdown of this record processor.
		 */
		Shutdown(shutdownInput *ShutdownInput)
	}

	// IRecordProcessorFactory is interface for creating IRecordProcessor. Each Worker can have multiple threads
	// for processing shard. Client can choose either creating one processor per shard or sharing them.
	IRecordProcessorFactory interface {

		/**
		 * Returns a record processor to be used for processing data records for a (assigned) shard.
		 *
		 * @return Returns a processor object.
		 */
		CreateProcessor() IRecordProcessor
	}
)
