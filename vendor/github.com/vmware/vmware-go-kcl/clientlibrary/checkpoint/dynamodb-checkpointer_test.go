/*
 * Copyright (c) 2019 VMware, Inc.
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
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"

	cfg "github.com/vmware/vmware-go-kcl/clientlibrary/config"
	par "github.com/vmware/vmware-go-kcl/clientlibrary/partition"
)

func TestDoesTableExist(t *testing.T) {
	svc := &mockDynamoDB{tableExist: true, item: map[string]*dynamodb.AttributeValue{}}
	checkpoint := &DynamoCheckpoint{
		TableName: "TableName",
		svc:       svc,
	}
	if !checkpoint.doesTableExist() {
		t.Error("Table exists but returned false")
	}

	svc = &mockDynamoDB{tableExist: false}
	checkpoint.svc = svc
	if checkpoint.doesTableExist() {
		t.Error("Table does not exist but returned true")
	}
}

func TestGetLeaseNotAquired(t *testing.T) {
	svc := &mockDynamoDB{tableExist: true, item: map[string]*dynamodb.AttributeValue{}}
	kclConfig := cfg.NewKinesisClientLibConfig("appName", "test", "us-west-2", "abc").
		WithInitialPositionInStream(cfg.LATEST).
		WithMaxRecords(10).
		WithMaxLeasesForWorker(1).
		WithShardSyncIntervalMillis(5000).
		WithFailoverTimeMillis(300000).
		WithMetricsBufferTimeMillis(10000).
		WithMetricsMaxQueueSize(20)
	checkpoint := NewDynamoCheckpoint(kclConfig).WithDynamoDB(svc)
	checkpoint.Init()
	err := checkpoint.GetLease(&par.ShardStatus{
		ID:         "0001",
		Checkpoint: "",
		Mux:        &sync.Mutex{},
	}, "abcd-efgh")
	if err != nil {
		t.Errorf("Error getting lease %s", err)
	}

	err = checkpoint.GetLease(&par.ShardStatus{
		ID:         "0001",
		Checkpoint: "",
		Mux:        &sync.Mutex{},
	}, "ijkl-mnop")
	if err == nil || err.Error() != ErrLeaseNotAquired {
		t.Errorf("Got a lease when it was already held by abcd-efgh: %s", err)
	}
}

func TestGetLeaseAquired(t *testing.T) {
	svc := &mockDynamoDB{tableExist: true, item: map[string]*dynamodb.AttributeValue{}}
	kclConfig := cfg.NewKinesisClientLibConfig("appName", "test", "us-west-2", "abc").
		WithInitialPositionInStream(cfg.LATEST).
		WithMaxRecords(10).
		WithMaxLeasesForWorker(1).
		WithShardSyncIntervalMillis(5000).
		WithFailoverTimeMillis(300000).
		WithMetricsBufferTimeMillis(10000).
		WithMetricsMaxQueueSize(20)
	checkpoint := NewDynamoCheckpoint(kclConfig).WithDynamoDB(svc)
	checkpoint.Init()
	marshalledCheckpoint := map[string]*dynamodb.AttributeValue{
		"ShardID": {
			S: aws.String("0001"),
		},
		"AssignedTo": {
			S: aws.String("abcd-efgh"),
		},
		"LeaseTimeout": {
			S: aws.String(time.Now().AddDate(0, -1, 0).UTC().Format(time.RFC3339)),
		},
		"SequenceID": {
			S: aws.String("deadbeef"),
		},
	}
	input := &dynamodb.PutItemInput{
		TableName: aws.String("TableName"),
		Item:      marshalledCheckpoint,
	}
	checkpoint.svc.PutItem(input)
	shard := &par.ShardStatus{
		ID:         "0001",
		Checkpoint: "deadbeef",
		Mux:        &sync.Mutex{},
	}
	err := checkpoint.GetLease(shard, "ijkl-mnop")

	if err != nil {
		t.Errorf("Lease not aquired after timeout %s", err)
	}

	id, ok := svc.item[CHECKPOINT_SEQUENCE_NUMBER_KEY]
	if !ok {
		t.Error("Expected checkpoint to be set by GetLease")
	} else if *id.S != "deadbeef" {
		t.Errorf("Expected checkpoint to be deadbeef. Got '%s'", *id.S)
	}

	// release owner info
	err = checkpoint.RemoveLeaseOwner(shard.ID)
	assert.Nil(t, err)

	status := &par.ShardStatus{
		ID:  shard.ID,
		Mux: &sync.Mutex{},
	}
	checkpoint.FetchCheckpoint(status)

	// checkpointer and parent shard id should be the same
	assert.Equal(t, shard.Checkpoint, status.Checkpoint)
	assert.Equal(t, shard.ParentShardId, status.ParentShardId)

	// Only the lease owner has been wiped out
	assert.Equal(t, "", status.GetLeaseOwner())
}

type mockDynamoDB struct {
	dynamodbiface.DynamoDBAPI
	tableExist bool
	item       map[string]*dynamodb.AttributeValue
}

func (m *mockDynamoDB) DescribeTable(*dynamodb.DescribeTableInput) (*dynamodb.DescribeTableOutput, error) {
	if !m.tableExist {
		return &dynamodb.DescribeTableOutput{}, awserr.New(dynamodb.ErrCodeResourceNotFoundException, "doesNotExist", errors.New(""))
	}
	return &dynamodb.DescribeTableOutput{}, nil
}

func (m *mockDynamoDB) PutItem(input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	item := input.Item

	if shardID, ok := item[LEASE_KEY_KEY]; ok {
		m.item[LEASE_KEY_KEY] = shardID
	}

	if owner, ok := item[LEASE_OWNER_KEY]; ok {
		m.item[LEASE_OWNER_KEY] = owner
	}

	if timeout, ok := item[LEASE_TIMEOUT_KEY]; ok {
		m.item[LEASE_TIMEOUT_KEY] = timeout
	}

	if checkpoint, ok := item[CHECKPOINT_SEQUENCE_NUMBER_KEY]; ok {
		m.item[CHECKPOINT_SEQUENCE_NUMBER_KEY] = checkpoint
	}

	if parent, ok := item[PARENT_SHARD_ID_KEY]; ok {
		m.item[PARENT_SHARD_ID_KEY] = parent
	}

	return nil, nil
}

func (m *mockDynamoDB) GetItem(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{
		Item: m.item,
	}, nil
}

func (m *mockDynamoDB) UpdateItem(input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	exp := input.UpdateExpression

	if aws.StringValue(exp) == "remove "+LEASE_OWNER_KEY {
		delete(m.item, LEASE_OWNER_KEY)
	}

	return nil, nil
}

func (m *mockDynamoDB) CreateTable(input *dynamodb.CreateTableInput) (*dynamodb.CreateTableOutput, error) {
	return &dynamodb.CreateTableOutput{}, nil
}
