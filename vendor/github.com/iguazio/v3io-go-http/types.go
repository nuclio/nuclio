package v3io

import (
	"encoding/xml"

	"github.com/valyala/fasthttp"
)

//
// Request / response
//

type Request struct {
	ID uint64

	// the container on which the request was performed (if applicable)
	container *Container

	// the session on which the request was performed (if applicable)
	session *Session

	// holds the input (e.g. ListBucketInput, GetItemInput)
	Input interface{}

	// the channel to which the response must be posted
	responseChan chan *Response

	// pointer to container
	requestResponse *RequestResponse
}

type Response struct {
	response *fasthttp.Response

	// hold a decoded output, if any
	Output interface{}

	// Equal to the ID of request
	ID uint64

	// holds the error for async responses
	Error error

	// pointer to container
	requestResponse *RequestResponse
}

func (r *Response) Release() {
	if r.response != nil {
		fasthttp.ReleaseResponse(r.response)
	}
}

func (r *Response) Body() []byte {
	return r.response.Body()
}

func (r *Response) Request() *Request {
	return &r.requestResponse.Request
}

// holds both a request and response
type RequestResponse struct {
	Request  Request
	Response Response
}

type ListBucketInput struct {
	Path string
}

type Content struct {
	XMLName        xml.Name `xml:"Contents"`
	Key            string   `xml:"Key"`
	Size           int      `xml:"Size"`
	LastSequenceId int      `xml:"LastSequenceId"`
	ETag           string   `xml:"ETag"`
	LastModified   string   `xml:"LastModified"`
}

type CommonPrefix struct {
	XMLName xml.Name `xml:"CommonPrefixes"`
	Prefix  string   `xml:"Prefix"`
}

type ListBucketOutput struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Name           string         `xml:"Name"`
	NextMarker     string         `xml:"NextMarker"`
	MaxKeys        string         `xml:"MaxKeys"`
	Contents       []Content      `xml:"Contents"`
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes"`
}

type ListAllInput struct {
}

type ListAllOutput struct {
	XMLName xml.Name    `xml:"ListAllMyBucketsResult"`
	Owner   interface{} `xml:"Owner"`
	Buckets Buckets     `xml:"Buckets"`
}

type Buckets struct {
	XMLName xml.Name `xml:"Buckets"`
	Bucket  []Bucket `xml:"Bucket"`
}

type Bucket struct {
	XMLName      xml.Name `xml:"Bucket"`
	Name         string   `xml:"Name"`
	CreationDate string   `xml:"CreationDate"`
	Id           int      `xml:"Id"`
}

type GetObjectInput struct {
	Path string
}

type PutObjectInput struct {
	Path string
	Body []byte
}

type DeleteObjectInput struct {
	Path string
}

type SetObjectInput struct {
	Path                       string
	ValidationModifiedTimeSec  uint64
	ValidationModifiedTimeNsec uint64
	ValidationOperation        string
	ValidationMask             uint64
	ValidationValue            uint64
	SetOperation               string
	DataMask                   uint64
	DataValue                  uint64
}

type PutItemInput struct {
	Path       string
	Attributes map[string]interface{}
	Condition  *string
}

type PutItemsInput struct {
	Path  string
	Items map[string]map[string]interface{}
}

type PutItemsOutput struct {
	Success bool
	Errors  map[string]error
}

type UpdateItemInput struct {
	Path       string
	Attributes map[string]interface{}
	Expression *string
	Condition  *string
}

type GetItemInput struct {
	Path           string
	AttributeNames []string
}

type Item map[string]interface{}

type GetItemOutput struct {
	Item Item
}

type GetItemsInput struct {
	Path           string
	AttributeNames []string
	Filter         string
	Marker         string
	Limit          int
	Segment        int
	TotalSegments  int
}

type GetItemsOutput struct {
	Last       bool
	NextMarker string
	Items      []Item
}

type CreateStreamInput struct {
	Path                 string
	ShardCount           int
	RetentionPeriodHours int
}

type StreamRecord struct {
	ShardID *int
	Data    []byte
}

type PutRecordsInput struct {
	Path    string
	Records []*StreamRecord
}

type PutRecordResult struct {
	SequenceNumber int
	ShardID        int `json:"ShardId"`
	ErrorCode      int
	ErrorMessage   string
}

type PutRecordsOutput struct {
	FailedRecordCount int
	Records           []PutRecordResult
}

type DeleteStreamInput struct {
	Path string
}

type SeekShardInputType int

const (
	SeekShardInputTypeTime SeekShardInputType = iota
	SeekShardInputTypeSequence
	SeekShardInputTypeLatest
	SeekShardInputTypeEarliest
)

type SeekShardInput struct {
	Path                   string
	Type                   SeekShardInputType
	StartingSequenceNumber int
	Timestamp              int
}

type SeekShardOutput struct {
	Location string
}

type GetRecordsInput struct {
	Path     string
	Location string
	Limit    int
}

type GetRecordsResult struct {
	ArrivalTimeSec  int
	ArrivalTimeNSec int
	SequenceNumber  int
	ClientInfo      string
	PartitionKey    string
	Data            []byte
}

type GetRecordsOutput struct {
	NextLocation        string
	MSecBehindLatest    int
	RecordsBehindLatest int
	Records             []GetRecordsResult
}
