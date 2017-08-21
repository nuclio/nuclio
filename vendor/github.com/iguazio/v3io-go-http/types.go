package v3io

import "encoding/xml"

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

type PutItemInput struct {
	Path       string
	Attributes map[string]interface{}
}

type UpdateItemInput struct {
	Path       string
	Attributes map[string]interface{}
}

type GetItemInput struct {
	Path           string
	AttributeNames []string
}

type GetItemOutput struct {
	Attributes map[string]interface{}
}
