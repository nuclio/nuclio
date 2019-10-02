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
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	log "github.com/sirupsen/logrus"

	"github.com/vmware/vmware-go-kcl/clientlibrary/config"
	par "github.com/vmware/vmware-go-kcl/clientlibrary/partition"
)

const (
	// ErrInvalidDynamoDBSchema is returned when there are one or more fields missing from the table
	ErrInvalidDynamoDBSchema = "The DynamoDB schema is invalid and may need to be re-created"

	// NumMaxRetries is the max times of doing retry
	NumMaxRetries = 5
)

// DynamoCheckpoint implements the Checkpoint interface using DynamoDB as a backend
type DynamoCheckpoint struct {
	TableName               string
	leaseTableReadCapacity  int64
	leaseTableWriteCapacity int64

	LeaseDuration  int
	svc            dynamodbiface.DynamoDBAPI
	kclConfig      *config.KinesisClientLibConfiguration
	Retries        int
	skipTableCheck bool
}

func NewDynamoCheckpoint(kclConfig *config.KinesisClientLibConfiguration) *DynamoCheckpoint {
	checkpointer := &DynamoCheckpoint{
		TableName:               kclConfig.TableName,
		leaseTableReadCapacity:  int64(kclConfig.InitialLeaseTableReadCapacity),
		leaseTableWriteCapacity: int64(kclConfig.InitialLeaseTableWriteCapacity),
		LeaseDuration:           kclConfig.FailoverTimeMillis,
		kclConfig:               kclConfig,
		Retries:                 NumMaxRetries,
	}

	return checkpointer
}

// WithDynamoDB is used to provide DynamoDB service
func (checkpointer *DynamoCheckpoint) WithDynamoDB(svc dynamodbiface.DynamoDBAPI) *DynamoCheckpoint {
	checkpointer.svc = svc
	return checkpointer
}

// Init initialises the DynamoDB Checkpoint
func (checkpointer *DynamoCheckpoint) Init() error {
	log.Info("Creating DynamoDB session")

	s, err := session.NewSession(&aws.Config{
		Region:      aws.String(checkpointer.kclConfig.RegionName),
		Endpoint:    aws.String(checkpointer.kclConfig.DynamoDBEndpoint),
		Credentials: checkpointer.kclConfig.DynamoDBCredentials,
		Retryer:     client.DefaultRetryer{NumMaxRetries: checkpointer.Retries},
	})

	if err != nil {
		// no need to move forward
		log.Fatalf("Failed in getting DynamoDB session for creating Worker: %+v", err)
	}

	if checkpointer.svc == nil {
		checkpointer.svc = dynamodb.New(s)
	}

	if !checkpointer.skipTableCheck && !checkpointer.doesTableExist() {
		return checkpointer.createTable()
	}
	return nil
}

// GetLease attempts to gain a lock on the given shard
func (checkpointer *DynamoCheckpoint) GetLease(shard *par.ShardStatus, newAssignTo string) error {
	newLeaseTimeout := time.Now().Add(time.Duration(checkpointer.LeaseDuration) * time.Millisecond).UTC()
	newLeaseTimeoutString := newLeaseTimeout.Format(time.RFC3339)
	currentCheckpoint, err := checkpointer.getItem(shard.ID)
	if err != nil {
		return err
	}

	assignedVar, assignedToOk := currentCheckpoint[LEASE_OWNER_KEY]
	leaseVar, leaseTimeoutOk := currentCheckpoint[LEASE_TIMEOUT_KEY]
	var conditionalExpression string
	var expressionAttributeValues map[string]*dynamodb.AttributeValue

	if !leaseTimeoutOk || !assignedToOk {
		conditionalExpression = "attribute_not_exists(AssignedTo)"
	} else {
		assignedTo := *assignedVar.S
		leaseTimeout := *leaseVar.S

		currentLeaseTimeout, err := time.Parse(time.RFC3339, leaseTimeout)
		if err != nil {
			return err
		}

		if !time.Now().UTC().After(currentLeaseTimeout) && assignedTo != newAssignTo {
			return errors.New(ErrLeaseNotAquired)
		}

		log.Debugf("Attempting to get a lock for shard: %s, leaseTimeout: %s, assignedTo: %s", shard.ID, currentLeaseTimeout, assignedTo)
		conditionalExpression = "ShardID = :id AND AssignedTo = :assigned_to AND LeaseTimeout = :lease_timeout"
		expressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":id": {
				S: aws.String(shard.ID),
			},
			":assigned_to": {
				S: aws.String(assignedTo),
			},
			":lease_timeout": {
				S: aws.String(leaseTimeout),
			},
		}
	}

	marshalledCheckpoint := map[string]*dynamodb.AttributeValue{
		LEASE_KEY_KEY: {
			S: aws.String(shard.ID),
		},
		LEASE_OWNER_KEY: {
			S: aws.String(newAssignTo),
		},
		LEASE_TIMEOUT_KEY: {
			S: aws.String(newLeaseTimeoutString),
		},
	}

	if len(shard.ParentShardId) > 0 {
		marshalledCheckpoint[PARENT_SHARD_ID_KEY] = &dynamodb.AttributeValue{S: aws.String(shard.ParentShardId)}
	}

	if shard.Checkpoint != "" {
		marshalledCheckpoint[CHECKPOINT_SEQUENCE_NUMBER_KEY] = &dynamodb.AttributeValue{
			S: aws.String(shard.Checkpoint),
		}
	}

	err = checkpointer.conditionalUpdate(conditionalExpression, expressionAttributeValues, marshalledCheckpoint)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return errors.New(ErrLeaseNotAquired)
			}
		}
		return err
	}

	shard.Mux.Lock()
	shard.AssignedTo = newAssignTo
	shard.LeaseTimeout = newLeaseTimeout
	shard.Mux.Unlock()

	return nil
}

// CheckpointSequence writes a checkpoint at the designated sequence ID
func (checkpointer *DynamoCheckpoint) CheckpointSequence(shard *par.ShardStatus) error {
	leaseTimeout := shard.LeaseTimeout.UTC().Format(time.RFC3339)
	marshalledCheckpoint := map[string]*dynamodb.AttributeValue{
		LEASE_KEY_KEY: {
			S: aws.String(shard.ID),
		},
		CHECKPOINT_SEQUENCE_NUMBER_KEY: {
			S: aws.String(shard.Checkpoint),
		},
		LEASE_OWNER_KEY: {
			S: aws.String(shard.AssignedTo),
		},
		LEASE_TIMEOUT_KEY: {
			S: aws.String(leaseTimeout),
		},
	}

	if len(shard.ParentShardId) > 0 {
		marshalledCheckpoint[PARENT_SHARD_ID_KEY] = &dynamodb.AttributeValue{S: &shard.ParentShardId}
	}

	return checkpointer.saveItem(marshalledCheckpoint)
}

// FetchCheckpoint retrieves the checkpoint for the given shard
func (checkpointer *DynamoCheckpoint) FetchCheckpoint(shard *par.ShardStatus) error {
	checkpoint, err := checkpointer.getItem(shard.ID)
	if err != nil {
		return err
	}

	sequenceID, ok := checkpoint[CHECKPOINT_SEQUENCE_NUMBER_KEY]
	if !ok {
		return ErrSequenceIDNotFound
	}
	log.Debugf("Retrieved Shard Iterator %s", *sequenceID.S)
	shard.Mux.Lock()
	defer shard.Mux.Unlock()
	shard.Checkpoint = aws.StringValue(sequenceID.S)

	if assignedTo, ok := checkpoint[LEASE_OWNER_KEY]; ok {
		shard.AssignedTo = aws.StringValue(assignedTo.S)
	}
	return nil
}

// RemoveLeaseInfo to remove lease info for shard entry in dynamoDB because the shard no longer exists in Kinesis
func (checkpointer *DynamoCheckpoint) RemoveLeaseInfo(shardID string) error {
	err := checkpointer.removeItem(shardID)

	if err != nil {
		log.Errorf("Error in removing lease info for shard: %s, Error: %+v", shardID, err)
	} else {
		log.Infof("Lease info for shard: %s has been removed.", shardID)
	}

	return err
}

// RemoveLeaseOwner to remove lease owner for the shard entry
func (checkpointer *DynamoCheckpoint) RemoveLeaseOwner(shardID string) error {
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(checkpointer.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			LEASE_KEY_KEY: {
				S: aws.String(shardID),
			},
		},
		UpdateExpression: aws.String("remove " + LEASE_OWNER_KEY),
	}

	_, err := checkpointer.svc.UpdateItem(input)

	return err
}

func (checkpointer *DynamoCheckpoint) createTable() error {
	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(LEASE_KEY_KEY),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(LEASE_KEY_KEY),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(checkpointer.leaseTableReadCapacity),
			WriteCapacityUnits: aws.Int64(checkpointer.leaseTableWriteCapacity),
		},
		TableName: aws.String(checkpointer.TableName),
	}
	_, err := checkpointer.svc.CreateTable(input)
	return err
}

func (checkpointer *DynamoCheckpoint) doesTableExist() bool {
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(checkpointer.TableName),
	}
	_, err := checkpointer.svc.DescribeTable(input)
	return err == nil
}

func (checkpointer *DynamoCheckpoint) saveItem(item map[string]*dynamodb.AttributeValue) error {
	return checkpointer.putItem(&dynamodb.PutItemInput{
		TableName: aws.String(checkpointer.TableName),
		Item:      item,
	})
}

func (checkpointer *DynamoCheckpoint) conditionalUpdate(conditionExpression string, expressionAttributeValues map[string]*dynamodb.AttributeValue, item map[string]*dynamodb.AttributeValue) error {
	return checkpointer.putItem(&dynamodb.PutItemInput{
		ConditionExpression:       aws.String(conditionExpression),
		TableName:                 aws.String(checkpointer.TableName),
		Item:                      item,
		ExpressionAttributeValues: expressionAttributeValues,
	})
}

func (checkpointer *DynamoCheckpoint) putItem(input *dynamodb.PutItemInput) error {
	_, err := checkpointer.svc.PutItem(input)
	return err
}

func (checkpointer *DynamoCheckpoint) getItem(shardID string) (map[string]*dynamodb.AttributeValue, error) {
	item, err := checkpointer.svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(checkpointer.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			LEASE_KEY_KEY: {
				S: aws.String(shardID),
			},
		},
	})
	return item.Item, err
}

func (checkpointer *DynamoCheckpoint) removeItem(shardID string) error {
	_, err := checkpointer.svc.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(checkpointer.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			LEASE_KEY_KEY: {
				S: aws.String(shardID),
			},
		},
	})
	return err
}
