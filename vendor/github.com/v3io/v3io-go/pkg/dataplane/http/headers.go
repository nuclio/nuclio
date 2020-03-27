package v3iohttp

// function names
const (
	putItemFunctionName        = "PutItem"
	updateItemFunctionName     = "UpdateItem"
	getItemFunctionName        = "GetItem"
	getItemsFunctionName       = "GetItems"
	createStreamFunctionName   = "CreateStream"
	describeStreamFunctionName = "DescribeStream"
	putRecordsFunctionName     = "PutRecords"
	getRecordsFunctionName     = "GetRecords"
	seekShardsFunctionName     = "SeekShard"
	getClusterMDFunctionName   = "GetClusterMD"
)

// headers for put item
var putItemHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": putItemFunctionName,
}

// headers for GetClusterMD
var getClusterMDHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": getClusterMDFunctionName,
}

// headers for update item
var updateItemHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": updateItemFunctionName,
}

// headers for update item
var getItemHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": getItemFunctionName,
}

// headers for get items
var getItemsHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": getItemsFunctionName,
}

// headers for get items requesting captain-proto response
var getItemsHeadersCapnp = map[string]string{
	"Content-Type":                 "application/json",
	"X-v3io-function":              getItemsFunctionName,
	"X-v3io-response-content-type": "capnp",
}

// headers for create stream
var createStreamHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": createStreamFunctionName,
}

// headers for get records
var describeStreamHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": describeStreamFunctionName,
}

// headers for put records
var putRecordsHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": putRecordsFunctionName,
}

// headers for put records
var getRecordsHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": getRecordsFunctionName,
}

// headers for seek records
var seekShardsHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": seekShardsFunctionName,
}

// map between SeekShardInputType and its encoded counterpart
var seekShardsInputTypeToString = [...]string{
	"TIME",
	"SEQUENCE",
	"LATEST",
	"EARLIEST",
}
