/*
kinesis-cli is a command line interface tool for interacting with AWS kinesis.

To install:
go get github.com/sendgridlabs/go-kinesis/kinesis-cli

To build:
cd $GOPATH/src/github.com/sendgridlabs/go-kinesis/kinesis-cli; go build

To use:
run ./kinesis-cli to see the usage.
*/

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	// "github.com/sendgridlabs/go-kinesis"
	"github.com/sendgridlabs/go-kinesis"
)

const HELP = `Usage: ./kinesis-cli <command> [<arg>, ...]
(Note: expects $AWS_ACCESS_KEY, $AWS_SECRET_KEY and $AWS_REGION_NAME to be set)
Commands:
       create   <streamName> [<# shards>]
       delete   <streamName>
       describe <streamName> [<exclusive start shardId> <limit>]
       split    <streamName> <shardId> [<hash key>]
       merge    <streamName> <shardId> <adjacent shardId>

`

var EMPTY_STRING = ""
var EMPTY_INT = -1
var DEFAULT_NUM_SHARDS = 1

func create(args []string) {
	streamName := getArg(args, 0, "stream name", nil)
	numShards := getIntArg(args, 1, "stream name", &DEFAULT_NUM_SHARDS)
	if !confirm(fmt.Sprintf("create stream '%s' with %d shard(s)", streamName, numShards)) {
		fmt.Println("Create canceled.")
		return
	}
	if err := newClient().CreateStream(streamName, numShards); err != nil {
		die(false, "Error creating shard: %s", err)
	}
}

func delete(args []string) {
	streamName := getArg(args, 0, "stream name", nil)
	if !confirm("delete stream '" + streamName + "'") {
		fmt.Println("Delete canceled.")
		return
	}
	if err := newClient().DeleteStream(streamName); err != nil {
		die(false, "Error deleting shard: %s", err)
	}
}

func describe(args []string) {
	streamName := getArg(args, 0, "stream name", nil)
	exclusiveStartShardId := getArg(args, 1, "exclusive start shard id", &EMPTY_STRING)
	limit := getIntArg(args, 2, "limit", &EMPTY_INT)
	streamDesc := describeStream(streamName, exclusiveStartShardId, limit)

	prettyBytes, err := json.MarshalIndent(streamDesc, "", "    ")
	if err != nil {
		die(false, "Error marshaling response: %s", err)
	}
	fmt.Println(string(prettyBytes))
}

func split(args []string) {
	streamName := getArg(args, 0, "stream name", nil)
	shardId := getArg(args, 1, "shard id", nil)
	newStartHash := getArg(args, 2, "starting hash", &EMPTY_STRING)
	if newStartHash == "" {
		newStartHash = askForShardStartHash(streamName, shardId)
	}
	if !confirm(fmt.Sprintf("split shard %s at hash key %s", shardId, newStartHash)) {
		fmt.Println("Split canceled.")
		return
	}
	requestArgs := kinesis.NewArgs()
	requestArgs.Add("StreamName", streamName)
	requestArgs.Add("ShardToSplit", shardId)
	requestArgs.Add("NewStartingHashKey", newStartHash)
	if err := newClient().SplitShard(requestArgs); err != nil {
		die(false, "Error splitting shard: %s", err)
	}
}

func merge(args []string) {
	streamName := getArg(args, 0, "stream name", nil)
	shardId := getArg(args, 1, "shard id", nil)
	adjacentShardId := getArg(args, 2, "adjacent shard id", nil)
	requestArgs := kinesis.NewArgs()
	requestArgs.Add("StreamName", streamName)
	requestArgs.Add("ShardToMerge", shardId)
	requestArgs.Add("AdjacentShardToMerge", adjacentShardId)
	if !confirm(fmt.Sprintf("merge shards %s and %s", shardId, adjacentShardId)) {
		fmt.Println("Merge canceled.")
		return
	}
	if err := newClient().MergeShards(requestArgs); err != nil {
		die(false, "Error merging shards: %s", err)
	}
}

func main() {
	if len(os.Args) < 2 {
		die(true, "Error: no command specified.")
	}
	if os.Getenv(kinesis.AccessEnvKey) == "" ||
		os.Getenv(kinesis.SecretEnvKey) == "" {
		fmt.Printf("WARNING: %s and/or %s environment variables not set. Will "+
			"attempt to fetch credentials from metadata server.\n",
			kinesis.AccessEnvKey, kinesis.SecretEnvKey)
	}
	if os.Getenv(kinesis.RegionEnvName) == "" {
		fmt.Printf("WARNING: %s not set.\n", kinesis.RegionEnvName)
	}
	switch os.Args[1] {
	case "create":
		create(os.Args[2:])
	case "delete":
		delete(os.Args[2:])
	case "describe":
		describe(os.Args[2:])
	case "split":
		split(os.Args[2:])
	case "merge":
		merge(os.Args[2:])
	default:
		die(true, "Error: unknown command '%s'", os.Args[1])
	}
}

//
// Command line helper functions
//

func die(printHelp bool, format string, args ...interface{}) {
	if printHelp {
		fmt.Print(HELP)
	}
	fmt.Printf(format, args...)
	fmt.Println("")
	os.Exit(1)
}

func confirm(action string) bool {
	prompt := fmt.Sprintf("Are you sure you want to %s?\n[y/N]: ", action)
	s := readString(prompt, "")
	return strings.ToLower(s) == "y"
}

func readString(prompt string, defaultStr string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	result, err := reader.ReadString('\n')
	if err != nil {
		die(false, "Error reading input: %s", err)
	}
	if result = strings.TrimSpace(result); result == "" {
		return defaultStr
	}
	return result
}

func getArg(specified []string, index int, name string, def *string) string {
	if index < len(specified) {
		return specified[index]
	}
	if def == nil {
		die(true, "Error: %s is required.", name)
	}
	return *def
}

func getIntArg(specified []string, index int, name string, def *int) int {
	var argStr string
	if def != nil {
		defStr := strconv.Itoa(*def)
		argStr = getArg(specified, index, name, &defStr)
	} else {
		argStr = getArg(specified, index, name, nil)
	}
	intArg, err := strconv.Atoi(argStr)
	if err != nil {
		die(false, "Error parsing %s as integer: %s\n%s", name, argStr, err)
	}
	return intArg
}

//
// Big int (for 128-bit start/end hash keys) helper functions
//

// Takes two large base 10 numeric strings a and b, and returns (a + b)/2
func getMiddle(lowStr, highStr string) *big.Int {
	low := bigIntFromStr(lowStr, 10)
	high := bigIntFromStr(highStr, 10)
	if low.Cmp(high) != -1 {
		die(false, "Error: %s is not smaller than %s", lowStr, highStr)
	}
	middle := new(big.Int)
	middle = middle.Div(middle.Add(low, high), big.NewInt(2))
	return middle
}

// Takes two large base 10 numeric strings low and high and returns low < x < high.
func isBetween(lowStr, highStr, xStr string) bool {
	low := bigIntFromStr(lowStr, 10)
	high := bigIntFromStr(highStr, 10)
	x := bigIntFromStr(xStr, 10)
	return x.Cmp(low) == 1 && x.Cmp(high) == -1
}

func bigIntFromStr(s string, base int) *big.Int {
	result := new(big.Int)
	result, success := result.SetString(s, 10)
	if !success {
		die(false, "Error: cannot create big int from string '%s'", s)
	}
	return result
}

//
// Kinesis helper functions
//

func newClient() kinesis.KinesisClient {
	auth, _ := kinesis.NewAuthFromEnv()
	return kinesis.New(auth, kinesis.NewRegionFromEnv())
}

func askForShardStartHash(streamName, shardId string) string {
	// Figure out a sensible default value for a split hash key.
	shardDesc := describeShard(streamName, shardId)
	if shardDesc == nil {
		die(false, "Error: No shard found with id %s", shardId)
	}
	existingStart, existingEnd := shardDesc.HashKeyRange.StartingHashKey, shardDesc.HashKeyRange.EndingHashKey
	newStartHash := getMiddle(existingStart, existingEnd).String()

	prompt := fmt.Sprintf("Shard's current hash key range (%s - %s)\nDefault (even split) key: %s\nType new key or press [enter] to choose default: ",
		existingStart, existingEnd, newStartHash)
	newStartHash = readString(prompt, newStartHash)
	if !isBetween(existingStart, existingEnd, newStartHash) {
		die(false, "New starting hash '%s' is not within shard's current range.", newStartHash)
	}
	return newStartHash
}

func describeShard(streamName, shardId string) *kinesis.DescribeStreamShards {
	describeResponse := describeStream(streamName, "", -1)
	for _, shard := range describeResponse.StreamDescription.Shards {
		if shard.ShardId == shardId {
			return &shard
		}
	}
	return nil
}

func describeStream(streamName, exclusiveStartShardId string, limit int) *kinesis.DescribeStreamResp {
	var response *kinesis.DescribeStreamResp
	done := false
	for !done {
		requestArgs := kinesis.NewArgs()
		requestArgs.Add("StreamName", streamName)
		if exclusiveStartShardId != "" {
			requestArgs.Add("ExclusiveStartShardId", exclusiveStartShardId)
		}
		if limit > 0 {
			requestArgs.Add("Limit", limit)
		}
		curResponse, err := newClient().DescribeStream(requestArgs)
		if err != nil {
			die(false, "Error describing stream: %s", err)
		}
		if response == nil {
			response = curResponse
		} else {
			shards := response.StreamDescription.Shards
			for _, shard := range curResponse.StreamDescription.Shards {
				shards = append(shards, shard)
				if len(shards) >= limit {
					done = true
					break
				}
				exclusiveStartShardId = shard.ShardId
				limit--
			}
		}
		done = done || !response.StreamDescription.HasMoreShards
	}
	return response
}
