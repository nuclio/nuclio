/*
Copyright 2018 The v3io Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v3io

import (
	"context"
	"encoding/xml"
	"os"
	"strconv"
	"strings"
	"time"
)

//
// Control plane
//

type NewSessionInput struct {
	URL       string
	Username  string
	Password  string
	AccessKey string
}

type NewContainerInput struct {
	ContainerName string
}

//
// Data plane
//

type DataPlaneInput struct {
	Ctx                 context.Context
	URL                 string
	ContainerName       string
	AuthenticationToken string
	AccessKey           string
	Timeout             time.Duration
}

type DataPlaneOutput struct {
	ctx context.Context
}

//
// Container
//

type GetClusterMDInput struct {
	DataPlaneInput
}
type GetClusterMDOutput struct {
	DataPlaneOutput
	NumberOfVNs int
}

type GetContainerContentsInput struct {
	DataPlaneInput
	Path             string
	GetAllAttributes bool   // if "true" return ALL available attributes
	DirectoriesOnly  bool   // if "true" return directory entries only, otherwise return children of any kind
	Limit            int    // max number of entries per request
	Marker           string // start from specific entry (e.g. to get next chunk)
}

type Content struct {
	Key            string `xml:"Key"`
	Size           *int   `xml:"Size"`           // file size in bytes
	LastSequenceID *int   `xml:"LastSequenceId"` // greater than zero for shard files
	LastModified   string `xml:"LastModified"`   // Date in format time.RFC3339: "2019-06-02T14:30:39.18Z"

	Mode         FileMode `xml:"Mode"`         // octal (ListDir) or decimal (GetItems) base, depends on API, e.g. 33204 or 0100664
	AccessTime   string   `xml:"AccessTime"`   // Date in format time.RFC3339: "2019-06-02T14:30:39.18Z"
	CreatingTime string   `xml:"CreatingTime"` // Date in format time.RFC3339: "2019-06-02T14:30:39.18Z"
	GID          string   `xml:"GID"`          // Hexadecimal representation of GID (e.g. "3e8" -> i.e. "0x3e8" == 1000)
	UID          string   `xml:"UID"`          // Hexadecimal representation of UID (e.g. "3e8" -> i.e. "0x3e8" == 1000)
	InodeNumber  *uint32  `xml:"InodeNumber"`  // iNode number
}

type CommonPrefix struct {
	Prefix       string   `xml:"Prefix"`       // directory name
	LastModified string   `xml:"LastModified"` // Date in format time.RFC3339: "2019-06-02T14:30:39.18Z"
	AccessTime   string   `xml:"AccessTime"`   // Date in format time.RFC3339: "2019-06-02T14:30:39.18Z"
	CreatingTime string   `xml:"CreatingTime"` // Date in format time.RFC3339: "2019-06-02T14:30:39.18Z"
	Mode         FileMode `xml:"Mode"`         // octal number, e.g. 040775
	GID          string   `xml:"GID"`          // Hexadecimal representation of GID (e.g. "3e8" -> i.e. "0x3e8" == 1000)
	UID          string   `xml:"UID"`          // Hexadecimal representation of UID (e.g. "3e8" -> i.e. "0x3e8" == 1000)
	InodeNumber  *uint64  `xml:"InodeNumber"`  // iNode number
}

type FileMode string

func (vfm FileMode) FileMode() (os.FileMode, error) {
	return mode(vfm)
}

func (vfm FileMode) String() string {
	mode, err := vfm.FileMode()
	if err != nil {
		return "unresolved"
	}
	return mode.String()
}

func mode(v3ioFileMode FileMode) (os.FileMode, error) {
	const S_IFMT = 0xf000     // nolint: golint
	const IP_OFFMASK = 0x1fff // nolint: golint

	// Note, File mode from different API's has different base.
	// For example Scan API returns file mode as decimal number (base 10) while ListDir as Octal (base 8)
	var sFileMode = string(v3ioFileMode)
	if strings.HasPrefix(sFileMode, "0") {

		// Convert octal representation of V3IO into decimal representation of Go
		mode, err := strconv.ParseUint(sFileMode, 8, 32)
		if err != nil {
			return os.FileMode(S_IFMT), err
		}

		golangFileMode := ((mode & S_IFMT) << 17) | (mode & IP_OFFMASK)
		return os.FileMode(golangFileMode), nil
	}

	mode, err := strconv.ParseUint(sFileMode, 10, 32)
	if err != nil {
		return os.FileMode(S_IFMT), err
	}
	return os.FileMode(mode), nil
}

type GetContainerContentsOutput struct {
	Name           string         `xml:"Name"`           // Bucket name
	NextMarker     string         `xml:"NextMarker"`     // if not empty and isTruncated="true" - has more children (need another fetch to get them)
	MaxKeys        string         `xml:"MaxKeys"`        // max number of entries in single batch
	Contents       []Content      `xml:"Contents"`       // files
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes"` // directories
	IsTruncated    bool           `xml:"IsTruncated"`    // "true" if has more content. Note, "NextMarker" should not be empty if "true"
}

type GetContainersInput struct {
	DataPlaneInput
}

type GetContainersOutput struct {
	DataPlaneOutput
	XMLName xml.Name    `xml:"ListBucketResult"`
	Owner   interface{} `xml:"Owner"`
	Results Containers  `xml:"Buckets"`
}

type Containers struct {
	Name       xml.Name        `xml:"Buckets"`
	Containers []ContainerInfo `xml:"Bucket"`
}

type ContainerInfo struct {
	BucketName   xml.Name `xml:"Bucket"`
	Name         string   `xml:"Name"`
	CreationDate string   `xml:"CreationDate"`
	ID           int      `xml:"Id"`
}

//
// Object
//

type GetObjectInput struct {
	DataPlaneInput
	Path     string
	Offset   int
	NumBytes int
}

type PutObjectInput struct {
	DataPlaneInput
	Path   string
	Offset int
	Body   []byte
	Append bool
}

type DeleteObjectInput struct {
	DataPlaneInput
	Path string
}

//
// KV
//

type PutItemInput struct {
	DataPlaneInput
	Path       string
	Condition  string
	Attributes map[string]interface{}
	UpdateMode string
}

type PutItemOutput struct {
	DataPlaneInput
	MtimeSecs  int
	MtimeNSecs int
}

type PutItemsInput struct {
	DataPlaneInput
	Path      string
	Condition string
	Items     map[string]map[string]interface{}
}

type PutItemsOutput struct {
	DataPlaneOutput
	Success bool
	Errors  map[string]error
}

type UpdateItemInput struct {
	DataPlaneInput
	Path       string
	Attributes map[string]interface{}
	Expression *string
	Condition  string
	UpdateMode string
}

type UpdateItemOutput struct {
	DataPlaneInput
	MtimeSecs  int
	MtimeNSecs int
}

type GetItemInput struct {
	DataPlaneInput
	Path           string
	AttributeNames []string
}

type GetItemOutput struct {
	DataPlaneOutput
	Item Item
}

type GetItemsInput struct {
	DataPlaneInput
	Path                string
	TableName           string
	AttributeNames      []string
	Filter              string
	Marker              string
	ShardingKey         string
	Limit               int
	Segment             int
	TotalSegments       int
	SortKeyRangeStart   string
	SortKeyRangeEnd     string
	RequestJSONResponse bool `json:"RequestJsonResponse"`
}

type GetItemsOutput struct {
	DataPlaneOutput
	Last       bool
	NextMarker string
	Items      []Item
}

//
// Stream
//

type StreamRecord struct {
	ShardID        *int
	Data           []byte
	ClientInfo     []byte
	PartitionKey   string
	SequenceNumber uint64
}

type SeekShardInputType int

const (
	SeekShardInputTypeTime SeekShardInputType = iota
	SeekShardInputTypeSequence
	SeekShardInputTypeLatest
	SeekShardInputTypeEarliest
)

type CreateStreamInput struct {
	DataPlaneInput
	Path                 string
	ShardCount           int
	RetentionPeriodHours int
}

type CheckPathExistsInput struct {
	DataPlaneInput
	Path string
}

type DescribeStreamInput struct {
	DataPlaneInput
	Path string
}

type DescribeStreamOutput struct {
	DataPlaneOutput
	ShardCount           int
	RetentionPeriodHours int
}

type DeleteStreamInput struct {
	DataPlaneInput
	Path string
}

type PutRecordsInput struct {
	DataPlaneInput
	Path    string
	Records []*StreamRecord
}

type PutRecordResult struct {
	SequenceNumber uint64
	ShardID        int `json:"ShardId"`
	ErrorCode      int
	ErrorMessage   string
}

type PutRecordsOutput struct {
	DataPlaneOutput
	FailedRecordCount int
	Records           []PutRecordResult
}

type SeekShardInput struct {
	DataPlaneInput
	Path                   string
	Type                   SeekShardInputType
	StartingSequenceNumber uint64
	Timestamp              int
}

type SeekShardOutput struct {
	DataPlaneOutput
	Location string
}

type GetRecordsInput struct {
	DataPlaneInput
	Path     string
	Location string
	Limit    int
}

type GetRecordsResult struct {
	ArrivalTimeSec  int
	ArrivalTimeNSec int
	SequenceNumber  uint64
	ClientInfo      []byte
	PartitionKey    string
	Data            []byte
}

type GetRecordsOutput struct {
	DataPlaneOutput
	NextLocation        string
	MSecBehindLatest    int
	RecordsBehindLatest int
	Records             []GetRecordsResult
}
