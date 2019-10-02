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
// The implementation is derived from https://github.com/patrobinson/gokini
//
// Copyright 2018 Patrick robinson
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
package checkpoint

import (
	"errors"
	par "github.com/vmware/vmware-go-kcl/clientlibrary/partition"
)

const (
	LEASE_KEY_KEY                  = "ShardID"
	LEASE_OWNER_KEY                = "AssignedTo"
	LEASE_TIMEOUT_KEY              = "LeaseTimeout"
	CHECKPOINT_SEQUENCE_NUMBER_KEY = "Checkpoint"
	PARENT_SHARD_ID_KEY            = "ParentShardId"

	// We've completely processed all records in this shard.
	SHARD_END = "SHARD_END"

	// ErrLeaseNotAquired is returned when we failed to get a lock on the shard
	ErrLeaseNotAquired = "Lease is already held by another node"
)

// Checkpointer handles checkpointing when a record has been processed
type Checkpointer interface {
	// Init initialises the Checkpoint
	Init() error

	// GetLease attempts to gain a lock on the given shard
	GetLease(*par.ShardStatus, string) error

	// CheckpointSequence writes a checkpoint at the designated sequence ID
	CheckpointSequence(*par.ShardStatus) error

	// FetchCheckpoint retrieves the checkpoint for the given shard
	FetchCheckpoint(*par.ShardStatus) error

	// RemoveLeaseInfo to remove lease info for shard entry because the shard no longer exists
	RemoveLeaseInfo(string) error

	// RemoveLeaseOwner to remove lease owner for the shard entry to make the shard available for reassignment
	RemoveLeaseOwner(string) error
}

// ErrSequenceIDNotFound is returned by FetchCheckpoint when no SequenceID is found
var ErrSequenceIDNotFound = errors.New("SequenceIDNotFoundForShard")
