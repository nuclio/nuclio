// Package kinesis provide GOlang API for http://aws.amazon.com/kinesis/
package kinesis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
)

const (
	ActionKey     = "Action"
	RegionEnvName = "AWS_REGION_NAME"

	// Regions
	USEast1      = "us-east-1"
	USWest2      = "us-west-2"
	EUWest1      = "eu-west-1"
	EUCentral1   = "eu-central-1"
	APSouthEast1 = "ap-southeast-1"
	APSouthEast2 = "ap-southeast-2"
	APNortheast1 = "ap-northeast-1"

	KinesisVersion  = "20131202"
	FirehoseVersion = "20150804"

	kinesisURL  = "https://kinesis.%s.amazonaws.com"
	firehoseURL = "https://firehose.%s.amazonaws.com"
)

// NewRegionFromEnv creates a region from the an expected environment variable
func NewRegionFromEnv() string {
	return os.Getenv(RegionEnvName)
}

// Structure for kinesis client
type Kinesis struct {
	client     *Client
	endpoint   string
	region     string
	version    string
	streamType string

	typeMu     sync.Mutex
	versionMu  sync.Mutex
	endpointMu sync.Mutex
}

// KinesisClient interface implemented by Kinesis
type KinesisClient interface {
	CreateStream(StreamName string, ShardCount int) error
	DeleteStream(StreamName string) error
	DescribeStream(args *RequestArgs) (resp *DescribeStreamResp, err error)
	DescribeDeliveryStream(args *RequestArgs) (resp *DescribeDeliveryStreamResp, err error)
	GetRecords(args *RequestArgs) (resp *GetRecordsResp, err error)
	GetShardIterator(args *RequestArgs) (resp *GetShardIteratorResp, err error)
	ListStreams(args *RequestArgs) (resp *ListStreamsResp, err error)
	MergeShards(args *RequestArgs) error
	PutRecord(args *RequestArgs) (resp *PutRecordResp, err error)
	PutRecords(args *RequestArgs) (resp *PutRecordsResp, err error)
	PutRecordBatch(args *RequestArgs) (resp *PutRecordBatchResp, err error)
	SplitShard(args *RequestArgs) error
}

// New returns an initialized AWS Kinesis client using the canonical live “production” endpoint
// for AWS Kinesis, i.e. https://kinesis.{region}.amazonaws.com
func New(auth Auth, region string) *Kinesis {
	endpoint := fmt.Sprintf(kinesisURL, region)
	return NewWithEndpoint(auth, region, endpoint)
}

// NewWithClient returns an initialized AWS Kinesis client using the canonical live “production” endpoint
// for AWS Kinesis, i.e. https://kinesis.{region}.amazonaws.com but with the ability to create a custom client
// with specific configurations like a timeout
func NewWithClient(region string, client *Client) *Kinesis {
	endpoint := fmt.Sprintf(kinesisURL, region)
	return &Kinesis{client: client, version: KinesisVersion, region: region, endpoint: endpoint, streamType: "Kinesis"}
}

// NewWithEndpoint returns an initialized AWS Kinesis client using the specified endpoint.
// This is generally useful for testing, so a local Kinesis server can be used.
func NewWithEndpoint(auth Auth, region, endpoint string) *Kinesis {
	// TODO: remove trailing slash on endpoint if there is one? does it matter?
	// TODO: validate endpoint somehow?
	return &Kinesis{client: NewClient(auth), version: KinesisVersion, region: region, endpoint: endpoint, streamType: "Kinesis"}
}

// Create params object for request
func makeParams(action string) map[string]string {
	params := make(map[string]string)
	params[ActionKey] = action
	return params
}

// RequestArgs store params for request
type RequestArgs struct {
	params  map[string]interface{}
	Records []Record
}

// NewFilter creates a new Filter.
func NewArgs() *RequestArgs {
	return &RequestArgs{
		params: make(map[string]interface{}),
	}
}

// Add appends a filtering parameter with the given name and value(s).
func (f *RequestArgs) Add(name string, value interface{}) {
	f.params[name] = value
}

func (f *RequestArgs) AddData(value []byte) {
	f.params["Data"] = value
}

// Error represent error from Kinesis API
type Error struct {
	// HTTP status code (200, 403, ...)
	StatusCode int
	// error code ("UnsupportedOperation", ...)
	Code string
	// The human-oriented error message
	Message   string
	RequestId string
}

// Return error message from error object
func (err *Error) Error() string {
	if err.Code == "" {
		return err.Message
	}
	return fmt.Sprintf("%s (%s)", err.Message, err.Code)
}

type jsonErrors struct {
	Code    string `json:"__type"`
	Message string
}

func buildError(r *http.Response) error {
	// Reading the body into a []byte because we might need to put it into an error
	// message after having the JSON decoding fail to produce a message.
	body, ioerr := ioutil.ReadAll(r.Body)
	if ioerr != nil {
		return fmt.Errorf("Could not read response body: %s", ioerr)
	}

	errors := jsonErrors{}
	json.NewDecoder(bytes.NewReader(body)).Decode(&errors)

	var err Error
	err.Message = errors.Message
	err.Code = errors.Code
	err.StatusCode = r.StatusCode
	if err.Message == "" {
		err.Message = fmt.Sprintf("%s: %s", r.Status, body)
	}
	return &err
}

func (k *Kinesis) getStreamType() string {
	k.typeMu.Lock()
	defer k.typeMu.Unlock()
	return k.streamType
}

func (k *Kinesis) setStreamType(streamType string) {
	k.typeMu.Lock()
	k.streamType = streamType
	k.typeMu.Unlock()
}

func (k *Kinesis) getVersion() string {
	k.versionMu.Lock()
	defer k.versionMu.Unlock()
	return k.version
}

func (k *Kinesis) setVersion(version string) {
	k.versionMu.Lock()
	k.version = version
	k.versionMu.Unlock()
}

func (k *Kinesis) getEndpoint() string {
	k.endpointMu.Lock()
	defer k.endpointMu.Unlock()
	return k.endpoint
}

func (k *Kinesis) setEndpoint(endpoint string) {
	k.endpointMu.Lock()
	k.endpoint = endpoint
	k.endpointMu.Unlock()
}

func (k *Kinesis) Firehose() {
	k.setStreamType("Firehose")
	k.setVersion(FirehoseVersion)
	k.setEndpoint(fmt.Sprintf(firehoseURL, k.region))
}

// Query by AWS API
func (kinesis *Kinesis) query(params map[string]string, data interface{}, resp interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// request
	request, err := http.NewRequest(
		"POST",
		kinesis.getEndpoint(),
		bytes.NewReader(jsonData),
	)

	if err != nil {
		return err
	}

	// headers
	request.Header.Set("Content-Type", "application/x-amz-json-1.1")
	request.Header.Set("X-Amz-Target", fmt.Sprintf("%s_%s.%s", kinesis.getStreamType(), kinesis.getVersion(), params[ActionKey]))
	request.Header.Set("User-Agent", "Golang Kinesis")

	// response
	response, err := kinesis.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return buildError(response)
	}

	if resp == nil {
		return nil
	}

	return json.NewDecoder(response.Body).Decode(resp)
}

// CreateStream adds a new Amazon Kinesis stream to your AWS account
// StreamName is a name of stream, ShardCount is number of shards
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_CreateStream.html
func (kinesis *Kinesis) CreateStream(StreamName string, ShardCount int) error {
	params := makeParams("CreateStream")
	requestParams := struct {
		StreamName string
		ShardCount int
	}{
		StreamName,
		ShardCount,
	}
	err := kinesis.query(params, requestParams, nil)
	if err != nil {
		return err
	}
	return nil
}

// DeleteStream deletes a stream and all of its shards and data from your AWS account
// StreamName is a name of stream
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_DeleteStream.html
func (kinesis *Kinesis) DeleteStream(StreamName string) error {
	params := makeParams("DeleteStream")
	requestParams := struct {
		StreamName string
	}{
		StreamName,
	}
	err := kinesis.query(params, requestParams, nil)
	if err != nil {
		return err
	}
	return nil
}

// MergeShards merges two adjacent shards in a stream and combines them into a single shard to reduce the stream's capacity to ingest and transport data
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_MergeShards.html
func (kinesis *Kinesis) MergeShards(args *RequestArgs) error {
	params := makeParams("MergeShards")
	err := kinesis.query(params, args.params, nil)
	if err != nil {
		return err
	}
	return nil
}

// SplitShard splits a shard into two new shards in the stream, to increase the stream's capacity to ingest and transport data
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_SplitShard.html
func (kinesis *Kinesis) SplitShard(args *RequestArgs) error {
	params := makeParams("SplitShard")
	err := kinesis.query(params, args.params, nil)
	if err != nil {
		return err
	}
	return nil
}

// ListStreamsResp stores the information that provides by ListStreams API call
type ListStreamsResp struct {
	HasMoreStreams bool
	StreamNames    []string
}

// ListStreams returns an array of the names of all the streams that are associated with the AWS account making the ListStreams request
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_ListStreams.html
func (kinesis *Kinesis) ListStreams(args *RequestArgs) (resp *ListStreamsResp, err error) {
	params := makeParams("ListStreams")
	resp = &ListStreamsResp{}
	err = kinesis.query(params, args.params, resp)
	if err != nil {
		return nil, err
	}
	return
}

// DescribeStreamShards stores the information about list of shards inside DescribeStreamResp
type DescribeStreamShards struct {
	AdjacentParentShardId string
	HashKeyRange          struct {
		EndingHashKey   string
		StartingHashKey string
	}
	ParentShardId       string
	SequenceNumberRange struct {
		EndingSequenceNumber   string
		StartingSequenceNumber string
	}
	ShardId string
}

// DescribeStreamResp stores the information that provides by DescribeStream API call
type DescribeStreamResp struct {
	StreamDescription struct {
		HasMoreShards bool
		Shards        []DescribeStreamShards
		StreamARN     string
		StreamName    string
		StreamStatus  string
	}
}

// DescribeStream returns the following information about the stream: the current status of the stream,
// the stream Amazon Resource Name (ARN), and an array of shard objects that comprise the stream.
// For each shard object there is information about the hash key and sequence number ranges that
// the shard spans, and the IDs of any earlier shards that played in a role in a MergeShards or
// SplitShard operation that created the shard
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_DescribeStream.html
func (kinesis *Kinesis) DescribeStream(args *RequestArgs) (resp *DescribeStreamResp, err error) {
	params := makeParams("DescribeStream")
	resp = &DescribeStreamResp{}
	err = kinesis.query(params, args.params, resp)
	if err != nil {
		return nil, err
	}
	return
}

// GetShardIteratorResp stores the information that provides by GetShardIterator API call
type GetShardIteratorResp struct {
	ShardIterator string
}

// GetShardIterator returns a shard iterator
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_GetShardIterator.html
func (kinesis *Kinesis) GetShardIterator(args *RequestArgs) (resp *GetShardIteratorResp, err error) {
	params := makeParams("GetShardIterator")
	resp = &GetShardIteratorResp{}
	err = kinesis.query(params, args.params, resp)
	if err != nil {
		return nil, err
	}
	return
}

// GetNextRecordsRecords stores the information that provides by GetNextRecordsResp
type GetRecordsRecords struct {
	ApproximateArrivalTimestamp float64
	Data                        []byte
	PartitionKey                string
	SequenceNumber              string
}

func (r GetRecordsRecords) GetData() []byte {
	return r.Data
}

// GetNextRecordsResp stores the information that provides by GetNextRecords API call
type GetRecordsResp struct {
	MillisBehindLatest int64
	NextShardIterator  string
	Records            []GetRecordsRecords
}

// GetRecords returns one or more data records from a shard
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_GetRecords.html
func (kinesis *Kinesis) GetRecords(args *RequestArgs) (resp *GetRecordsResp, err error) {
	params := makeParams("GetRecords")
	resp = &GetRecordsResp{}
	err = kinesis.query(params, args.params, resp)
	if err != nil {
		return nil, err
	}
	return
}

// PutRecordResp stores the information that provides by PutRecord API call
type PutRecordResp struct {
	SequenceNumber string
	ShardId        string
}

// PutRecord puts a data record into an Amazon Kinesis stream from a producer.
// args must contain a single record added with AddRecord.
// More info: http://docs.aws.amazon.com/kinesis/latest/APIReference/API_PutRecord.html
func (kinesis *Kinesis) PutRecord(args *RequestArgs) (resp *PutRecordResp, err error) {
	params := makeParams("PutRecord")

	if _, ok := args.params["Data"]; !ok && len(args.Records) == 0 {
		return nil, errors.New("PutRecord requires its args param to contain a record added with either AddRecord or AddData.")
	} else if ok && len(args.Records) > 0 {
		return nil, errors.New("PutRecord requires its args param to contain a record added with either AddRecord or AddData but not both.")
	} else if len(args.Records) > 1 {
		return nil, errors.New("PutRecord does not support more than one record.")
	}

	if len(args.Records) > 0 {
		args.AddData(args.Records[0].Data)
		args.Add("PartitionKey", args.Records[0].PartitionKey)
	}

	resp = &PutRecordResp{}
	err = kinesis.query(params, args.params, resp)
	if err != nil {
		return nil, err
	}
	return
}

// PutRecords puts multiple data records into an Amazon Kinesis stream from a producer
// more info http://docs.aws.amazon.com/kinesis/latest/APIReference/API_PutRecords.html
func (kinesis *Kinesis) PutRecords(args *RequestArgs) (resp *PutRecordsResp, err error) {
	params := makeParams("PutRecords")
	resp = &PutRecordsResp{}
	args.Add("Records", args.Records)
	err = kinesis.query(params, args.params, resp)

	if err != nil {
		return nil, err
	}
	return
}

// PutRecordsResp stores the information that provides by PutRecord API call
type PutRecordsResp struct {
	FailedRecordCount int
	Records           []PutRecordsRespRecord
}

// RecordResp stores individual Record information provided by PutRecords API call
type PutRecordsRespRecord struct {
	ErrorCode      string
	ErrorMessage   string
	SequenceNumber string
	ShardId        string
}

// Add data and partition for sending multiple Records to Kinesis in one API call
func (f *RequestArgs) AddRecord(value []byte, partitionKey string) {
	r := Record{
		Data:         value,
		PartitionKey: partitionKey,
	}
	f.Records = append(f.Records, r)
}

// Record stores the Data and PartitionKey for PutRecord or PutRecords calls to Kinesis API
type Record struct {
	Data         []byte
	PartitionKey string
}
