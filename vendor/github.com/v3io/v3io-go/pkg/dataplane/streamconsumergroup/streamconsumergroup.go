package streamconsumergroup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"

	"github.com/v3io/v3io-go/pkg/common"
	"github.com/v3io/v3io-go/pkg/dataplane"
	v3ioerrors "github.com/v3io/v3io-go/pkg/errors"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type streamConsumerGroup struct {
	logger         logger.Logger
	name           string
	config         *Config
	container      v3io.Container
	streamPath     string
	maxReplicas    int
	totalNumShards int
}

func NewStreamConsumerGroup(parentLogger logger.Logger,
	name string,
	config *Config,
	container v3io.Container,
	streamPath string,
	maxReplicas int) (StreamConsumerGroup, error) {
	var err error

	if config == nil {
		config = NewConfig()
	}

	newStreamConsumerGroup := streamConsumerGroup{
		logger:      parentLogger.GetChild(name),
		name:        name,
		config:      config,
		container:   container,
		streamPath:  streamPath,
		maxReplicas: maxReplicas,
	}

	// get the total number of shards for this stream
	newStreamConsumerGroup.totalNumShards, err = newStreamConsumerGroup.getTotalNumberOfShards()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get total number of shards")
	}

	return &newStreamConsumerGroup, nil
}

func (scg *streamConsumerGroup) GetState() (*State, error) {
	state, _, err := scg.getStateFromPersistency()

	return state, err
}

func (scg *streamConsumerGroup) GetShardSequenceNumber(shardID int) (uint64, error) {
	return scg.getShardSequenceNumberFromPersistency(shardID)
}

func (scg *streamConsumerGroup) GetNumShards() (int, error) {
	return scg.totalNumShards, nil
}

func (scg *streamConsumerGroup) getShardPath(shardID int) (string, error) {
	return path.Join(scg.streamPath, strconv.Itoa(shardID)), nil
}

func (scg *streamConsumerGroup) getTotalNumberOfShards() (int, error) {
	response, err := scg.container.DescribeStreamSync(&v3io.DescribeStreamInput{
		Path: scg.streamPath,
	})
	if err != nil {
		return 0, errors.Wrapf(err, "Failed describing stream: %s", scg.streamPath)
	}

	defer response.Release()

	return response.Output.(*v3io.DescribeStreamOutput).ShardCount, nil
}

func (scg *streamConsumerGroup) setState(modifier stateModifier) (*State, error) {
	var previousState, modifiedState *State

	backoff := scg.config.State.ModifyRetry.Backoff
	attempts := scg.config.State.ModifyRetry.Attempts

	err := common.RetryFunc(context.TODO(), scg.logger, attempts, nil, &backoff, func(int) (bool, error) {
		state, mtime, err := scg.getStateFromPersistency()
		if err != nil && err != v3ioerrors.ErrNotFound {
			return true, errors.Wrap(err, "Failed getting current state from persistency")
		}

		if state == nil {
			state, err = newState()
			if err != nil {
				return true, errors.Wrap(err, "Failed to create state")
			}
		}

		// for logging
		previousState = state.deepCopy()

		modifiedState, err = modifier(state)
		if err != nil {
			return true, errors.Wrap(err, "Failed modifying state")
		}

		// log only on change
		if !scg.statesEqual(previousState, modifiedState) {
			scg.logger.DebugWith("Modified state, saving",
				"previousState", previousState,
				"modifiedState", modifiedState)
		}

		err = scg.setStateInPersistency(modifiedState, mtime)
		if err != nil {
			return true, errors.Wrap(err, "Failed setting state in persistency state")
		}

		return false, nil
	})

	if err != nil {
		return nil, errors.Wrapf(err, "Failed modifying state, attempts exhausted. currentState(%s)", previousState.String())
	}

	return modifiedState, nil
}

func (scg *streamConsumerGroup) setStateInPersistency(state *State, mtime *int) error {
	stateContents, err := json.Marshal(state)
	if err != nil {
		return errors.Wrap(err, "Failed marshaling state file contents")
	}

	var condition string
	if mtime != nil {
		condition = fmt.Sprintf("__mtime_nsecs == %v", *mtime)
	}

	_, err = scg.container.UpdateItemSync(&v3io.UpdateItemInput{
		Path:      scg.getStateFilePath(),
		Condition: condition,
		Attributes: map[string]interface{}{
			stateContentsAttributeKey: string(stateContents),
		},
	})
	if err != nil {
		return errors.Wrap(err, "Failed setting state in persistency")
	}

	return nil
}

func (scg *streamConsumerGroup) getStateFromPersistency() (*State, *int, error) {
	response, err := scg.container.GetItemSync(&v3io.GetItemInput{
		Path:           scg.getStateFilePath(),
		AttributeNames: []string{"__mtime_nsecs", stateContentsAttributeKey},
	})

	if err != nil {
		errWithStatusCode, errHasStatusCode := err.(v3ioerrors.ErrorWithStatusCode)
		if !errHasStatusCode {
			return nil, nil, errors.Wrap(err, "Got error without status code")
		}

		if errWithStatusCode.StatusCode() != 404 {
			return nil, nil, errors.Wrap(err, "Failed getting state item")
		}

		return nil, nil, v3ioerrors.ErrNotFound
	}

	defer response.Release()

	getItemOutput := response.Output.(*v3io.GetItemOutput)

	stateContents, err := getItemOutput.Item.GetFieldString(stateContentsAttributeKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed getting state attribute")
	}

	var state State

	err = json.Unmarshal([]byte(stateContents), &state)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed unmarshalling state contents: %s", stateContents)
	}

	stateMtime, err := getItemOutput.Item.GetFieldInt("__mtime_nsecs")
	if err != nil {
		return nil, nil, errors.New("Failed getting mtime attribute")
	}

	return &state, &stateMtime, nil
}

func (scg *streamConsumerGroup) getStateFilePath() string {
	return path.Join(scg.streamPath, fmt.Sprintf("%s-state.json", scg.name))
}

func (scg *streamConsumerGroup) getShardLocationFromPersistency(shardID int) (string, error) {
	scg.logger.DebugWith("Getting shard sequenceNumber from persistency", "shardID", shardID)

	seekShardInput := v3io.SeekShardInput{}

	// get the shard sequenceNumber from the item
	shardSequenceNumber, err := scg.getShardSequenceNumberFromPersistency(shardID)
	if err != nil {

		// if the error is that the attribute wasn't found, but the shard was found - seek the shard
		// according to the configuration
		if err != ErrShardSequenceNumberAttributeNotFound {
			return "", errors.Wrap(err, "Failed to get shard sequenceNumber from item attributes")
		}

		seekShardInput.Type = scg.config.Claim.RecordBatchFetch.InitialLocation
	} else {

		// use sequence number
		seekShardInput.Type = v3io.SeekShardInputTypeSequence
		seekShardInput.StartingSequenceNumber = shardSequenceNumber + 1
	}

	seekShardInput.Path, err = scg.getShardPath(shardID)
	if err != nil {
		return "", errors.Wrapf(err, "Failed getting shard path: %v", shardID)
	}

	return scg.getShardLocationWithSeek(&seekShardInput)
}

// returns the sequenceNumber, an error re: the shard itself and an error re: the attribute in the shard
func (scg *streamConsumerGroup) getShardSequenceNumberFromPersistency(shardID int) (uint64, error) {
	shardPath, err := scg.getShardPath(shardID)
	if err != nil {
		return 0, errors.Wrapf(err, "Failed getting shard path: %v", shardID)
	}

	response, err := scg.container.GetItemSync(&v3io.GetItemInput{
		Path:           shardPath,
		AttributeNames: []string{scg.getShardCommittedSequenceNumberAttributeName()},
	})

	if err != nil {
		errWithStatusCode, errHasStatusCode := err.(v3ioerrors.ErrorWithStatusCode)
		if !errHasStatusCode {
			return 0, errors.Wrap(err, "Got error without status code")
		}

		if errWithStatusCode.StatusCode() != http.StatusNotFound {
			return 0, errors.Wrap(err, "Failed getting shard item")
		}

		// TODO: remove after errors.Is support added
		scg.logger.DebugWith("Could not find shard, probably doesn't exist yet", "path", shardPath)

		return 0, ErrShardNotFound
	}

	defer response.Release()

	getItemOutput := response.Output.(*v3io.GetItemOutput)

	// return the attribute name
	sequenceNumber, err := getItemOutput.Item.GetFieldUint64(scg.getShardCommittedSequenceNumberAttributeName())
	if err != nil && err == v3ioerrors.ErrNotFound {
		return 0, ErrShardSequenceNumberAttributeNotFound
	}

	// return the sequenceNumber we found
	return sequenceNumber, nil
}

func (scg *streamConsumerGroup) getShardLocationWithSeek(seekShardInput *v3io.SeekShardInput) (string, error) {
	scg.logger.DebugWith("Seeking shard", "shardPath", seekShardInput.Path, "seekShardInput", seekShardInput)

	response, err := scg.container.SeekShardSync(seekShardInput)
	if err != nil {
		return "", errors.Wrap(err, "Failed to seek shard")
	}
	defer response.Release()

	location := response.Output.(*v3io.SeekShardOutput).Location

	scg.logger.DebugWith("Seek shard succeeded", "shardPath", seekShardInput.Path, "location", location)

	return location, nil
}

func (scg *streamConsumerGroup) getShardCommittedSequenceNumberAttributeName() string {
	return fmt.Sprintf("__%s_committed_sequence_number", scg.name)
}

func (scg *streamConsumerGroup) setShardSequenceNumberInPersistency(shardID int, sequenceNumber uint64) error {
	scg.logger.DebugWith("Setting shard sequenceNumber in persistency", "shardID", shardID, "sequenceNumber", sequenceNumber)
	shardPath, err := scg.getShardPath(shardID)
	if err != nil {
		return errors.Wrapf(err, "Failed getting shard path: %v", shardID)
	}

	_, err = scg.container.UpdateItemSync(&v3io.UpdateItemInput{
		Path: shardPath,
		Attributes: map[string]interface{}{
			scg.getShardCommittedSequenceNumberAttributeName(): sequenceNumber,
		},
	})
	return err
}

// returns true if the states are equal, ignoring heartbeat times
func (scg *streamConsumerGroup) statesEqual(state0 *State, state1 *State) bool {
	if state0.SchemasVersion != state1.SchemasVersion {
		return false
	}

	if len(state0.SessionStates) != len(state1.SessionStates) {
		return false
	}

	// since we compared lengths, we can only do this from state0
	for _, state0SessionState := range state0.SessionStates {
		session1SessionState := state1.findSessionStateByMemberID(state0SessionState.MemberID)

		// if couldn't find session state
		if session1SessionState == nil {
			return false
		}

		if !common.IntSlicesEqual(state0SessionState.Shards, session1SessionState.Shards) {
			return false
		}
	}

	return true
}
