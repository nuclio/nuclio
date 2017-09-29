package kinesis

// PutRecordBatchResp stores the information that provides by PutRecordBatch API call
type PutRecordBatchResp struct {
	FailedPutCount   int
	RequestResponses []PutRecordBatchResponses
}

// RecordBatchResponses stores individual Record information provided by PutRecordBatch API call
type PutRecordBatchResponses struct {
	ErrorCode    string
	ErrorMessage string
	RecordId     string
}

type S3DestinationDescriptionResp struct {
	BucketARN      string
	BufferingHints struct {
		IntervalInSeconds int
		SizeInMBs         int
	}
	CompressionFormat       string
	EncryptionConfiguration struct {
		KMSEncryptionConfig struct {
			AWSKMSKeyARN string
		}
		NoEncryptionConfig string
	}
	Prefix  string
	RoleARN string
}

type RedshiftDestinationDescriptionResp struct {
	ClusterJDBCURL string
	CopyCommand    struct {
		CopyOptions      string
		DataTableColumns string
		DataTableName    string
	}
	RoleARN                  string
	S3DestinationDescription S3DestinationDescriptionResp
	Username                 string
}

type DestinationsResp struct {
	DestinationId                  string
	RedshiftDestinationDescription RedshiftDestinationDescriptionResp
	S3DestinationDescription       S3DestinationDescriptionResp
}

// DescribeDeliveryStreamResp stores the information that provides by the Firehose DescribeDeliveryStream API call
type DescribeDeliveryStreamResp struct {
	DeliveryStreamDescription struct {
		CreateTimestamp      float32
		DeliveryStreamARN    string
		DeliveryStreamName   string
		DeliveryStreamStatus string
		Destinations         []DestinationsResp
		HasMoreDestinations  bool
		LastUpdatedTimestamp int
		VersionId            string
	}
}

// http://docs.aws.amazon.com/firehose/latest/APIReference/API_DescribeDeliveryStream.html
func (kinesis *Kinesis) DescribeDeliveryStream(args *RequestArgs) (resp *DescribeDeliveryStreamResp, err error) {
	kinesis.Firehose()
	params := makeParams("DescribeDeliveryStream")
	resp = &DescribeDeliveryStreamResp{}
	err = kinesis.query(params, args.params, resp)
	if err != nil {
		return nil, err
	}
	return
}

// http://docs.aws.amazon.com/firehose/latest/APIReference/API_PutRecordBatch.html
func (kinesis *Kinesis) PutRecordBatch(args *RequestArgs) (resp *PutRecordBatchResp, err error) {
	kinesis.Firehose()

	params := makeParams("PutRecordBatch")
	resp = &PutRecordBatchResp{}
	args.Add("Records", args.Records)
	err = kinesis.query(params, args.params, resp)

	if err != nil {
		return nil, err
	}
	return
}
