package v3iohttp

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/v3io/v3io-go/pkg/dataplane"
	"github.com/v3io/v3io-go/pkg/dataplane/schemas/node/common"
	"github.com/v3io/v3io-go/pkg/errors"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/valyala/fasthttp"
	"zombiezen.com/go/capnproto2"
)

// TODO: Request should have a global pool
var requestID uint64

type context struct {
	logger           logger.Logger
	requestChan      chan *v3io.Request
	httpClient       *fasthttp.Client
	clusterEndpoints []string
	numWorkers       int
}

type NewClientInput struct {
	tlsConfig       *tls.Config
	dialTimeout     time.Duration
	maxConnsPerHost int
}

func NewClient(newClientInput *NewClientInput) *fasthttp.Client {
	tlsConfig := newClientInput.tlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	dialTimeout := newClientInput.dialTimeout
	if dialTimeout == 0 {
		dialTimeout = fasthttp.DefaultDialTimeout
	}
	dialFunction := func(addr string) (net.Conn, error) {
		return fasthttp.DialTimeout(addr, dialTimeout)
	}

	return &fasthttp.Client{
		TLSConfig:       tlsConfig,
		Dial:            dialFunction,
		MaxConnsPerHost: newClientInput.maxConnsPerHost,
	}
}

func NewDefaultClient() *fasthttp.Client {
	return NewClient(&NewClientInput{})
}

func NewContext(parentLogger logger.Logger, client *fasthttp.Client, newContextInput *v3io.NewContextInput) (v3io.Context, error) {
	requestChanLen := newContextInput.RequestChanLen
	if requestChanLen == 0 {
		requestChanLen = 1024
	}

	numWorkers := newContextInput.NumWorkers
	if numWorkers == 0 {
		numWorkers = 8
	}

	newContext := &context{
		logger:      parentLogger.GetChild("context.http"),
		httpClient:  client,
		requestChan: make(chan *v3io.Request, requestChanLen),
		numWorkers:  numWorkers,
	}

	for workerIndex := 0; workerIndex < numWorkers; workerIndex++ {
		go newContext.workerEntry(workerIndex)
	}

	return newContext, nil
}

// create a new session
func (c *context) NewSession(newSessionInput *v3io.NewSessionInput) (v3io.Session, error) {
	return newSession(c.logger,
		c,
		newSessionInput.URL,
		newSessionInput.Username,
		newSessionInput.Password,
		newSessionInput.AccessKey)
}

// GetContainers
func (c *context) GetContainers(getContainersInput *v3io.GetContainersInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(getContainersInput, context, responseChan)
}

// GetContainersSync
func (c *context) GetContainersSync(getContainersInput *v3io.GetContainersInput) (*v3io.Response, error) {
	return c.sendRequestAndXMLUnmarshal(
		&getContainersInput.DataPlaneInput,
		http.MethodGet,
		"",
		"",
		nil,
		nil,
		&v3io.GetContainersOutput{})
}

// GetContainers
func (c *context) GetContainerContents(getContainerContentsInput *v3io.GetContainerContentsInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(getContainerContentsInput, context, responseChan)
}

// GetContainerContentsSync
func (c *context) GetContainerContentsSync(getContainerContentsInput *v3io.GetContainerContentsInput) (*v3io.Response, error) {
	getContainerContentOutput := v3io.GetContainerContentsOutput{}

	var queryBuilder strings.Builder
	if getContainerContentsInput.Path != "" {
		queryBuilder.WriteString("prefix=")
		queryBuilder.WriteString(getContainerContentsInput.Path)
	}

	if getContainerContentsInput.DirectoriesOnly {
		queryBuilder.WriteString("&prefix-only=1")
	}

	if getContainerContentsInput.GetAllAttributes {
		queryBuilder.WriteString("&prefix-info=1")
	}

	if getContainerContentsInput.Marker != "" {
		queryBuilder.WriteString("&marker=")
		queryBuilder.WriteString(getContainerContentsInput.Marker)
	}

	if getContainerContentsInput.Limit > 0 {
		queryBuilder.WriteString("&max-keys=")
		queryBuilder.WriteString(strconv.Itoa(getContainerContentsInput.Limit))
	}

	return c.sendRequestAndXMLUnmarshal(&getContainerContentsInput.DataPlaneInput,
		http.MethodGet,
		"",
		queryBuilder.String(),
		nil,
		nil,
		&getContainerContentOutput)
}

// GetItem
func (c *context) GetItem(getItemInput *v3io.GetItemInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(getItemInput, context, responseChan)
}

type attributeValuesSection struct {
	accumulatedPreviousSectionsLength int
	data                              node_common_capnp.VnObjectAttributeValuePtr_List
}

// GetItemSync
func (c *context) GetItemSync(getItemInput *v3io.GetItemInput) (*v3io.Response, error) {

	// no need to marshal, just sprintf
	body := fmt.Sprintf(`{"AttributesToGet": "%s"}`, strings.Join(getItemInput.AttributeNames, ","))

	response, err := c.sendRequest(&getItemInput.DataPlaneInput,
		http.MethodPut,
		getItemInput.Path,
		"",
		getItemHeaders,
		[]byte(body),
		false)

	if err != nil {
		return nil, err
	}

	// ad hoc structure that contains response
	item := struct {
		Item map[string]map[string]interface{}
	}{}

	// unmarshal the body
	err = json.Unmarshal(response.Body(), &item)
	if err != nil {
		return nil, err
	}

	// decode the response
	attributes, err := c.decodeTypedAttributes(item.Item)
	if err != nil {
		return nil, err
	}

	// attach the output to the response
	response.Output = &v3io.GetItemOutput{Item: attributes}

	return response, nil
}

// GetItems
func (c *context) GetItems(getItemsInput *v3io.GetItemsInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(getItemsInput, context, responseChan)
}

// GetItemSync
func (c *context) GetItemsSync(getItemsInput *v3io.GetItemsInput) (*v3io.Response, error) {

	// create GetItem Body
	body := map[string]interface{}{}

	if len(getItemsInput.AttributeNames) > 0 {
		body["AttributesToGet"] = strings.Join(getItemsInput.AttributeNames, ",")
	}

	if getItemsInput.TableName != "" {
		body["TableName"] = getItemsInput.TableName
	}

	if getItemsInput.Filter != "" {
		body["FilterExpression"] = getItemsInput.Filter
	}

	if getItemsInput.Marker != "" {
		body["Marker"] = getItemsInput.Marker
	}

	if getItemsInput.ShardingKey != "" {
		body["ShardingKey"] = getItemsInput.ShardingKey
	}

	if getItemsInput.Limit != 0 {
		body["Limit"] = getItemsInput.Limit
	}

	if getItemsInput.TotalSegments != 0 {
		body["TotalSegment"] = getItemsInput.TotalSegments
		body["Segment"] = getItemsInput.Segment
	}

	if getItemsInput.SortKeyRangeStart != "" {
		body["SortKeyRangeStart"] = getItemsInput.SortKeyRangeStart
	}

	if getItemsInput.SortKeyRangeEnd != "" {
		body["SortKeyRangeEnd"] = getItemsInput.SortKeyRangeEnd
	}

	marshalledBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	headers := getItemsHeadersCapnp
	if getItemsInput.RequestJSONResponse {
		headers = getItemsHeaders
	}

	response, err := c.sendRequest(&getItemsInput.DataPlaneInput,
		"PUT",
		getItemsInput.Path,
		"",
		headers,
		marshalledBody,
		false)

	if err != nil {
		return nil, err
	}

	contentType := string(response.HeaderPeek("Content-Type"))

	if contentType != "application/octet-capnp" {
		c.logger.DebugWithCtx(getItemsInput.Ctx, "Body", "body", string(response.Body()))
		response.Output, err = c.getItemsParseJSONResponse(response, getItemsInput)
	} else {
		var withWildcard bool
		for _, attributeName := range getItemsInput.AttributeNames {
			if attributeName == "*" || attributeName == "**" {
				withWildcard = true
				break
			}
		}
		response.Output, err = c.getItemsParseCAPNPResponse(response, withWildcard)
	}
	return response, err
}

// PutItem
func (c *context) PutItem(putItemInput *v3io.PutItemInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(putItemInput, context, responseChan)
}

// PutItemSync
func (c *context) PutItemSync(putItemInput *v3io.PutItemInput) error {
	var body map[string]interface{}
	if putItemInput.UpdateMode != "" {
		body = map[string]interface{}{
			"UpdateMode": putItemInput.UpdateMode,
		}
	}

	// prepare the query path
	_, err := c.putItem(&putItemInput.DataPlaneInput,
		putItemInput.Path,
		putItemFunctionName,
		putItemInput.Attributes,
		putItemInput.Condition,
		putItemHeaders,
		body)

	return err
}

// PutItems
func (c *context) PutItems(putItemsInput *v3io.PutItemsInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(putItemsInput, context, responseChan)
}

// PutItemsSync
func (c *context) PutItemsSync(putItemsInput *v3io.PutItemsInput) (*v3io.Response, error) {

	response := c.allocateResponse()
	if response == nil {
		return nil, errors.New("Failed to allocate response")
	}

	putItemsOutput := v3io.PutItemsOutput{
		Success: true,
	}

	for itemKey, itemAttributes := range putItemsInput.Items {

		// try to post the item
		_, err := c.putItem(&putItemsInput.DataPlaneInput,
			putItemsInput.Path+"/"+itemKey,
			putItemFunctionName,
			itemAttributes,
			putItemsInput.Condition,
			putItemHeaders,
			nil)

		// if there was an error, shove it to the list of errors
		if err != nil {

			// create the map to hold the errors since at least one exists
			if putItemsOutput.Errors == nil {
				putItemsOutput.Errors = map[string]error{}
			}

			putItemsOutput.Errors[itemKey] = err

			// clear success, since at least one error exists
			putItemsOutput.Success = false
		}
	}

	response.Output = &putItemsOutput

	return response, nil
}

// UpdateItem
func (c *context) UpdateItem(updateItemInput *v3io.UpdateItemInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(updateItemInput, context, responseChan)
}

// UpdateItemSync
func (c *context) UpdateItemSync(updateItemInput *v3io.UpdateItemInput) error {
	var err error

	if updateItemInput.Attributes != nil {

		// specify update mode as part of body. "Items" will be injected
		body := map[string]interface{}{
			"UpdateMode": "CreateOrReplaceAttributes",
		}

		if updateItemInput.UpdateMode != "" {
			body["UpdateMode"] = updateItemInput.UpdateMode
		}

		_, err = c.putItem(&updateItemInput.DataPlaneInput,
			updateItemInput.Path,
			putItemFunctionName,
			updateItemInput.Attributes,
			updateItemInput.Condition,
			putItemHeaders,
			body)

	} else if updateItemInput.Expression != nil {

		_, err = c.updateItemWithExpression(&updateItemInput.DataPlaneInput,
			updateItemInput.Path,
			updateItemFunctionName,
			*updateItemInput.Expression,
			updateItemInput.Condition,
			updateItemHeaders,
			updateItemInput.UpdateMode)
	}

	return err
}

// GetObject
func (c *context) GetObject(getObjectInput *v3io.GetObjectInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(getObjectInput, context, responseChan)
}

// GetObjectSync
func (c *context) GetObjectSync(getObjectInput *v3io.GetObjectInput) (*v3io.Response, error) {
	return c.sendRequest(&getObjectInput.DataPlaneInput,
		http.MethodGet,
		getObjectInput.Path,
		"",
		nil,
		nil,
		false)
}

// PutObject
func (c *context) PutObject(putObjectInput *v3io.PutObjectInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(putObjectInput, context, responseChan)
}

// PutObjectSync
func (c *context) PutObjectSync(putObjectInput *v3io.PutObjectInput) error {
	_, err := c.sendRequest(&putObjectInput.DataPlaneInput,
		http.MethodPut,
		putObjectInput.Path,
		"",
		nil,
		putObjectInput.Body,
		true)

	return err
}

// DeleteObject
func (c *context) DeleteObject(deleteObjectInput *v3io.DeleteObjectInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(deleteObjectInput, context, responseChan)
}

// DeleteObjectSync
func (c *context) DeleteObjectSync(deleteObjectInput *v3io.DeleteObjectInput) error {
	_, err := c.sendRequest(&deleteObjectInput.DataPlaneInput,
		http.MethodDelete,
		deleteObjectInput.Path,
		"",
		nil,
		nil,
		true)

	return err
}

// CreateStream
func (c *context) CreateStream(createStreamInput *v3io.CreateStreamInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(createStreamInput, context, responseChan)
}

// CreateStreamSync
func (c *context) CreateStreamSync(createStreamInput *v3io.CreateStreamInput) error {
	body := fmt.Sprintf(`{"ShardCount": %d, "RetentionPeriodHours": %d}`,
		createStreamInput.ShardCount,
		createStreamInput.RetentionPeriodHours)

	_, err := c.sendRequest(&createStreamInput.DataPlaneInput,
		http.MethodPost,
		createStreamInput.Path,
		"",
		createStreamHeaders,
		[]byte(body),
		true)

	return err
}

// DescribeStream
func (c *context) DescribeStream(describeStreamInput *v3io.DescribeStreamInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(describeStreamInput, context, responseChan)
}

// DescribeStreamSync
func (c *context) DescribeStreamSync(describeStreamInput *v3io.DescribeStreamInput) (*v3io.Response, error) {
	response, err := c.sendRequest(&describeStreamInput.DataPlaneInput,
		http.MethodPut,
		describeStreamInput.Path,
		"",
		describeStreamHeaders,
		nil,
		false)
	if err != nil {
		return nil, err
	}

	describeStreamOutput := v3io.DescribeStreamOutput{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &describeStreamOutput)
	if err != nil {
		return nil, err
	}

	// set the output in the response
	response.Output = &describeStreamOutput

	return response, nil
}

// DeleteStream
func (c *context) DeleteStream(deleteStreamInput *v3io.DeleteStreamInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(deleteStreamInput, context, responseChan)
}

// DeleteStreamSync
func (c *context) DeleteStreamSync(deleteStreamInput *v3io.DeleteStreamInput) error {

	// get all shards in the stream
	response, err := c.GetContainerContentsSync(&v3io.GetContainerContentsInput{
		DataPlaneInput: deleteStreamInput.DataPlaneInput,
		Path:           deleteStreamInput.Path,
	})

	if err != nil {
		return err
	}

	defer response.Release()

	// delete the shards one by one
	// TODO: paralellize
	for _, content := range response.Output.(*v3io.GetContainerContentsOutput).Contents {

		// TODO: handle error - stop deleting? return multiple errors?
		c.DeleteObjectSync(&v3io.DeleteObjectInput{ // nolint: errcheck
			DataPlaneInput: deleteStreamInput.DataPlaneInput,
			Path:           "/" + content.Key,
		})
	}

	// delete the actual stream
	return c.DeleteObjectSync(&v3io.DeleteObjectInput{
		DataPlaneInput: deleteStreamInput.DataPlaneInput,
		Path:           "/" + path.Dir(deleteStreamInput.Path) + "/",
	})
}

// SeekShard
func (c *context) SeekShard(seekShardInput *v3io.SeekShardInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(seekShardInput, context, responseChan)
}

// SeekShardSync
func (c *context) SeekShardSync(seekShardInput *v3io.SeekShardInput) (*v3io.Response, error) {
	var buffer bytes.Buffer

	buffer.WriteString(`{"Type": "`)
	buffer.WriteString(seekShardsInputTypeToString[seekShardInput.Type])
	buffer.WriteString(`"`)

	if seekShardInput.Type == v3io.SeekShardInputTypeSequence {
		buffer.WriteString(`, "StartingSequenceNumber": `)
		buffer.WriteString(strconv.FormatUint(seekShardInput.StartingSequenceNumber, 10))
	} else if seekShardInput.Type == v3io.SeekShardInputTypeTime {
		buffer.WriteString(`, "TimestampSec": `)
		buffer.WriteString(strconv.Itoa(seekShardInput.Timestamp))
		buffer.WriteString(`, "TimestampNSec": 0`)
	}

	buffer.WriteString(`}`)

	response, err := c.sendRequest(&seekShardInput.DataPlaneInput,
		http.MethodPut,
		seekShardInput.Path,
		"",
		seekShardsHeaders,
		buffer.Bytes(),
		false)
	if err != nil {
		return nil, err
	}

	seekShardOutput := v3io.SeekShardOutput{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &seekShardOutput)
	if err != nil {
		return nil, err
	}

	// set the output in the response
	response.Output = &seekShardOutput

	return response, nil
}

// PutRecords
func (c *context) PutRecords(putRecordsInput *v3io.PutRecordsInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(putRecordsInput, context, responseChan)
}

// PutRecordsSync
func (c *context) PutRecordsSync(putRecordsInput *v3io.PutRecordsInput) (*v3io.Response, error) {

	// TODO: set this to an initial size through heuristics?
	// This function encodes manually
	var buffer bytes.Buffer

	buffer.WriteString(`{"Records": [`)

	for recordIdx, record := range putRecordsInput.Records {
		buffer.WriteString(`{"Data": "`)
		buffer.WriteString(base64.StdEncoding.EncodeToString(record.Data))
		buffer.WriteString(`"`)

		if record.ClientInfo != nil {
			buffer.WriteString(`,"ClientInfo": "`)
			buffer.WriteString(base64.StdEncoding.EncodeToString(record.ClientInfo))
			buffer.WriteString(`"`)
		}

		if record.ShardID != nil {
			buffer.WriteString(`, "ShardId": `)
			buffer.WriteString(strconv.Itoa(*record.ShardID))
		}

		if record.PartitionKey != "" {
			buffer.WriteString(`, "PartitionKey": `)
			buffer.WriteString(`"` + record.PartitionKey + `"`)
		}

		// add comma if not last
		if recordIdx != len(putRecordsInput.Records)-1 {
			buffer.WriteString(`}, `)
		} else {
			buffer.WriteString(`}`)
		}
	}

	buffer.WriteString(`]}`)
	str := buffer.String()
	fmt.Println(str)

	response, err := c.sendRequest(&putRecordsInput.DataPlaneInput,
		http.MethodPost,
		putRecordsInput.Path,
		"",
		putRecordsHeaders,
		buffer.Bytes(),
		false)
	if err != nil {
		return nil, err
	}

	putRecordsOutput := v3io.PutRecordsOutput{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &putRecordsOutput)
	if err != nil {
		return nil, err
	}

	// set the output in the response
	response.Output = &putRecordsOutput

	return response, nil
}

// GetRecords
func (c *context) GetRecords(getRecordsInput *v3io.GetRecordsInput,
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	return c.sendRequestToWorker(getRecordsInput, context, responseChan)
}

// GetRecordsSync
func (c *context) GetRecordsSync(getRecordsInput *v3io.GetRecordsInput) (*v3io.Response, error) {
	body := fmt.Sprintf(`{"Location": "%s", "Limit": %d}`,
		getRecordsInput.Location,
		getRecordsInput.Limit)

	response, err := c.sendRequest(&getRecordsInput.DataPlaneInput,
		http.MethodPut,
		getRecordsInput.Path,
		"",
		getRecordsHeaders,
		[]byte(body),
		false)
	if err != nil {
		return nil, err
	}

	getRecordsOutput := v3io.GetRecordsOutput{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &getRecordsOutput)
	if err != nil {
		return nil, err
	}

	// set the output in the response
	response.Output = &getRecordsOutput

	return response, nil
}

func (c *context) putItem(dataPlaneInput *v3io.DataPlaneInput,
	path string,
	functionName string,
	attributes map[string]interface{},
	condition string,
	headers map[string]string,
	body map[string]interface{}) (*v3io.Response, error) {

	// iterate over all attributes and encode them with their types
	typedAttributes, err := c.encodeTypedAttributes(attributes)
	if err != nil {
		return nil, err
	}

	// create an empty body if the user didn't pass anything
	if body == nil {
		body = map[string]interface{}{}
	}

	// set item in body (use what the user passed as a base)
	body["Item"] = typedAttributes

	if condition != "" {
		body["ConditionExpression"] = condition
	}

	jsonEncodedBodyContents, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return c.sendRequest(dataPlaneInput,
		http.MethodPut,
		path,
		"",
		headers,
		jsonEncodedBodyContents,
		false)
}

func (c *context) updateItemWithExpression(dataPlaneInput *v3io.DataPlaneInput,
	path string,
	functionName string,
	expression string,
	condition string,
	headers map[string]string,
	updateMode string) (*v3io.Response, error) {

	body := map[string]interface{}{
		"UpdateExpression": expression,
		"UpdateMode":       "CreateOrReplaceAttributes",
	}

	if updateMode != "" {
		body["UpdateMode"] = updateMode
	}

	if condition != "" {
		body["ConditionExpression"] = condition
	}

	jsonEncodedBodyContents, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return c.sendRequest(dataPlaneInput,
		http.MethodPost,
		path,
		"",
		headers,
		jsonEncodedBodyContents,
		false)
}

func (c *context) sendRequestAndXMLUnmarshal(dataPlaneInput *v3io.DataPlaneInput,
	method string,
	path string,
	query string,
	headers map[string]string,
	body []byte,
	output interface{}) (*v3io.Response, error) {

	response, err := c.sendRequest(dataPlaneInput, method, path, query, headers, body, false)
	if err != nil {
		return nil, err
	}

	// unmarshal the body into the output
	err = xml.Unmarshal(response.Body(), output)
	if err != nil {
		response.Release()

		return nil, err
	}

	// set output in response
	response.Output = output

	return response, nil
}

func (c *context) sendRequest(dataPlaneInput *v3io.DataPlaneInput,
	method string,
	path string,
	query string,
	headers map[string]string,
	body []byte,
	releaseResponse bool) (*v3io.Response, error) {

	var success bool
	var statusCode int
	var err error

	if dataPlaneInput.ContainerName == "" {
		return nil, errors.New("ContainerName must not be empty")
	}

	request := fasthttp.AcquireRequest()
	response := c.allocateResponse()

	uri, err := c.buildRequestURI(dataPlaneInput.URL, dataPlaneInput.ContainerName, query, path)
	if err != nil {
		return nil, err
	}
	uriStr := uri.String()

	// init request
	request.SetRequestURI(uriStr)
	request.Header.SetMethod(method)
	request.SetBody(body)

	// check if we need to an an authorization header
	if len(dataPlaneInput.AuthenticationToken) > 0 {
		request.Header.Set("Authorization", dataPlaneInput.AuthenticationToken)
	}

	if len(dataPlaneInput.AccessKey) > 0 {
		request.Header.Set("X-v3io-session-key", dataPlaneInput.AccessKey)
	}

	for headerName, headerValue := range headers {
		request.Header.Add(headerName, headerValue)
	}

	//c.logger.DebugWithCtx(dataPlaneInput.Ctx,
	//	"Tx",
	//	"uri", uriStr,
	//	"method", method,
	//	"body-length", len(body))

	if dataPlaneInput.Timeout <= 0 {
		err = c.httpClient.Do(request, response.HTTPResponse)
	} else {
		err = c.httpClient.DoTimeout(request, response.HTTPResponse, dataPlaneInput.Timeout)
	}

	if err != nil {
		goto cleanup
	}

	statusCode = response.HTTPResponse.StatusCode()

	{
		//contentLength := response.HTTPResponse.Header.ContentLength()
		//if contentLength < 0 {
		//	contentLength = 0
		//}
		//c.logger.DebugWithCtx(dataPlaneInput.Ctx,
		//	"Rx",
		//	"statusCode", statusCode,
		//	"Content-Length", contentLength)
	}

	// did we get a 2xx response?
	success = statusCode >= 200 && statusCode < 300

	// make sure we got expected status
	if !success {
		var re = regexp.MustCompile(".*X-V3io-Session-Key:.*")
		sanitizedRequest := re.ReplaceAllString(request.String(), "X-V3io-Session-Key: SANITIZED")
		err = v3ioerrors.NewErrorWithStatusCode(
			fmt.Errorf("Expected a 2xx response status code: %s\nRequest details:\n%s",
				response.HTTPResponse.String(), sanitizedRequest),
			statusCode)
		goto cleanup
	}

cleanup:

	// we're done with the request - the response must be released by the user
	// unless there's an error
	fasthttp.ReleaseRequest(request)

	if err != nil {
		response.Release()
		return nil, err
	}

	// if the user doesn't need the response, release it
	if releaseResponse {
		response.Release()
		return nil, nil
	}

	return response, nil
}

func (c *context) buildRequestURI(urlString string, containerName string, query string, pathStr string) (*url.URL, error) {
	uri, err := url.Parse(urlString)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse cluster endpoint URL %s", c.clusterEndpoints[0])
	}
	uri.Path = path.Clean(path.Join("/", containerName, pathStr))
	if strings.HasSuffix(pathStr, "/") {
		uri.Path += "/" // retain trailing slash
	}
	uri.RawQuery = strings.Replace(query, " ", "%20", -1)
	return uri, nil
}

func (c *context) allocateResponse() *v3io.Response {
	return &v3io.Response{
		HTTPResponse: fasthttp.AcquireResponse(),
	}
}

// {"age": 30, "name": "foo"} -> {"age": {"N": 30}, "name": {"S": "foo"}}
func (c *context) encodeTypedAttributes(attributes map[string]interface{}) (map[string]map[string]interface{}, error) {
	typedAttributes := make(map[string]map[string]interface{})

	for attributeName, attributeValue := range attributes {
		typedAttributes[attributeName] = make(map[string]interface{})
		switch value := attributeValue.(type) {
		default:
			return nil, fmt.Errorf("unexpected attribute type for %s: %T", attributeName, reflect.TypeOf(attributeValue))
		case int:
			typedAttributes[attributeName]["N"] = strconv.Itoa(value)
		case uint64:
			typedAttributes[attributeName]["N"] = strconv.FormatUint(value, 10)
		case int64:
			typedAttributes[attributeName]["N"] = strconv.FormatInt(value, 10)
			// this is a tmp bypass to the fact Go maps Json numbers to float64
		case float64:
			typedAttributes[attributeName]["N"] = strconv.FormatFloat(value, 'E', -1, 64)
		case string:
			typedAttributes[attributeName]["S"] = value
		case []byte:
			typedAttributes[attributeName]["B"] = base64.StdEncoding.EncodeToString(value)
		case bool:
			typedAttributes[attributeName]["BOOL"] = value
		case time.Time:
			nanos := value.UnixNano()
			typedAttributes[attributeName]["TS"] = fmt.Sprintf("%v:%v", nanos/1000000000, nanos%1000000000)
		}
	}

	return typedAttributes, nil
}

// {"age": {"N": 30}, "name": {"S": "foo"}} -> {"age": 30, "name": "foo"}
func (c *context) decodeTypedAttributes(typedAttributes map[string]map[string]interface{}) (map[string]interface{}, error) {
	var err error
	attributes := map[string]interface{}{}

	for attributeName, typedAttributeValue := range typedAttributes {

		typeError := func(attributeName string, attributeType string, value interface{}) error {
			return errors.Errorf("Stated attribute type '%s' for attribute '%s' did not match actual attribute type '%T'", attributeType, attributeName, value)
		}

		// try to parse as number
		if value, ok := typedAttributeValue["N"]; ok {
			numberValue, ok := value.(string)
			if !ok {
				return nil, typeError(attributeName, "N", value)
			}

			// try int
			if intValue, err := strconv.Atoi(numberValue); err != nil {

				// try float
				floatValue, err := strconv.ParseFloat(numberValue, 64)
				if err != nil {
					return nil, fmt.Errorf("value for %s is not int or float: %s", attributeName, numberValue)
				}

				// save as float
				attributes[attributeName] = floatValue
			} else {
				attributes[attributeName] = intValue
			}
		} else if value, ok := typedAttributeValue["S"]; ok {
			stringValue, ok := value.(string)
			if !ok {
				return nil, typeError(attributeName, "S", value)
			}

			attributes[attributeName] = stringValue
		} else if value, ok := typedAttributeValue["B"]; ok {
			byteSliceValue, ok := value.(string)
			if !ok {
				return nil, typeError(attributeName, "B", value)
			}

			attributes[attributeName], err = base64.StdEncoding.DecodeString(byteSliceValue)
			if err != nil {
				return nil, err
			}
		} else if value, ok := typedAttributeValue["BOOL"]; ok {
			boolValue, ok := value.(bool)
			if !ok {
				return nil, typeError(attributeName, "BOOL", value)
			}

			attributes[attributeName] = boolValue
		} else if value, ok := typedAttributeValue["TS"]; ok {
			timestampValue, ok := value.(string)
			if !ok {
				return nil, typeError(attributeName, "TS", value)
			}

			timeParts := strings.Split(timestampValue, ":")
			if len(timeParts) != 2 {
				return nil, fmt.Errorf("incorrect format of timestamp value: %v", timestampValue)
			}

			seconds, err := strconv.ParseInt(timeParts[0], 10, 64)
			if err != nil {
				return nil, err
			}
			nanos, err := strconv.ParseInt(timeParts[1], 10, 64)
			if err != nil {
				return nil, err
			}
			timeValue := time.Unix(seconds, nanos)

			attributes[attributeName] = timeValue
		}
	}

	return attributes, nil
}

func (c *context) sendRequestToWorker(input interface{},
	context interface{},
	responseChan chan *v3io.Response) (*v3io.Request, error) {
	id := atomic.AddUint64(&requestID, 1)

	// create a request/response (TODO: from pool)
	requestResponse := &v3io.RequestResponse{
		Request: v3io.Request{
			ID:                  id,
			Input:               input,
			Context:             context,
			ResponseChan:        responseChan,
			SendTimeNanoseconds: time.Now().UnixNano(),
		},
	}

	// point to container
	requestResponse.Request.RequestResponse = requestResponse

	// send the request to the request channel
	c.requestChan <- &requestResponse.Request

	return &requestResponse.Request, nil
}

func (c *context) workerEntry(workerIndex int) {
	for {
		var response *v3io.Response
		var err error

		// read a request
		request := <-c.requestChan

		// according to the input type
		switch typedInput := request.Input.(type) {
		case *v3io.PutObjectInput:
			err = c.PutObjectSync(typedInput)
		case *v3io.GetObjectInput:
			response, err = c.GetObjectSync(typedInput)
		case *v3io.DeleteObjectInput:
			err = c.DeleteObjectSync(typedInput)
		case *v3io.GetItemInput:
			response, err = c.GetItemSync(typedInput)
		case *v3io.GetItemsInput:
			response, err = c.GetItemsSync(typedInput)
		case *v3io.PutItemInput:
			err = c.PutItemSync(typedInput)
		case *v3io.PutItemsInput:
			response, err = c.PutItemsSync(typedInput)
		case *v3io.UpdateItemInput:
			err = c.UpdateItemSync(typedInput)
		case *v3io.CreateStreamInput:
			err = c.CreateStreamSync(typedInput)
		case *v3io.DescribeStreamInput:
			response, err = c.DescribeStreamSync(typedInput)
		case *v3io.DeleteStreamInput:
			err = c.DeleteStreamSync(typedInput)
		case *v3io.GetRecordsInput:
			response, err = c.GetRecordsSync(typedInput)
		case *v3io.PutRecordsInput:
			response, err = c.PutRecordsSync(typedInput)
		case *v3io.SeekShardInput:
			response, err = c.SeekShardSync(typedInput)
		case *v3io.GetContainersInput:
			response, err = c.GetContainersSync(typedInput)
		case *v3io.GetContainerContentsInput:
			response, err = c.GetContainerContentsSync(typedInput)
		default:
			c.logger.ErrorWith("Got unexpected request type", "type", reflect.TypeOf(request.Input).String())
		}

		// TODO: have the sync interfaces somehow use the pre-allocated response
		if response != nil {
			request.RequestResponse.Response = *response
		}

		response = &request.RequestResponse.Response

		response.ID = request.ID
		response.Error = err
		response.RequestResponse = request.RequestResponse
		response.Context = request.Context

		// write to response channel
		request.ResponseChan <- &request.RequestResponse.Response
	}
}

func readAllCapnpMessages(reader io.Reader) []*capnp.Message {
	var capnpMessages []*capnp.Message
	for {
		msg, err := capnp.NewDecoder(reader).Decode()
		if err != nil {
			break
		}
		capnpMessages = append(capnpMessages, msg)
	}
	return capnpMessages
}

func getSectionAndIndex(values []attributeValuesSection, idx int) (section int, resIdx int) {
	if len(values) == 1 {
		return 0, idx
	}
	for i := 1; i < len(values); i++ {
		if values[i].accumulatedPreviousSectionsLength > idx {
			return i, idx - values[i-1].accumulatedPreviousSectionsLength
		}
	}
	return 0, idx
}

func decodeCapnpAttributes(keyValues node_common_capnp.VnObjectItemsGetMappedKeyValuePair_List, values []attributeValuesSection, attributeNames []string) (map[string]interface{}, error) {
	attributes := map[string]interface{}{}
	for j := 0; j < keyValues.Len(); j++ {
		attrPtr := keyValues.At(j)
		valIdx := int(attrPtr.ValueMapIndex())
		attrIdx := int(attrPtr.KeyMapIndex())

		attributeName := attributeNames[attrIdx]
		sectIdx, valIdx := getSectionAndIndex(values, valIdx)
		value, err := values[sectIdx].data.At(valIdx).Value()
		if err != nil {
			return attributes, errors.Wrapf(err, "values[%d].data.At(%d).Value", sectIdx, valIdx)
		}
		switch value.Which() {
		case node_common_capnp.ExtAttrValue_Which_qword:
			attributes[attributeName] = int(value.Qword())
		case node_common_capnp.ExtAttrValue_Which_uqword:
			attributes[attributeName] = int(value.Uqword())
		case node_common_capnp.ExtAttrValue_Which_blob:
			attributes[attributeName], err = value.Blob()
			if err != nil {
				return attributes, errors.Wrapf(err, "unable to get value of BLOB attribute '%s'", attributeName)
			}
		case node_common_capnp.ExtAttrValue_Which_str:
			attributes[attributeName], err = value.Str()
			if err != nil {
				return attributes, errors.Wrapf(err, "unable to get value of String attribute '%s'", attributeName)
			}
		case node_common_capnp.ExtAttrValue_Which_dfloat:
			attributes[attributeName] = value.Dfloat()
		case node_common_capnp.ExtAttrValue_Which_boolean:
			attributes[attributeName] = value.Boolean()
		case node_common_capnp.ExtAttrValue_Which_time:
			t, err := value.Time()
			if err != nil {
				return nil, err
			}
			attributes[attributeName] = time.Unix(t.TvSec(), t.TvNsec())
		case node_common_capnp.ExtAttrValue_Which_notExists:
			continue // skip
		default:
			return attributes, errors.Errorf("getItemsCapnp: %s type for %s attribute is not expected", value.Which().String(), attributeName)
		}
	}
	return attributes, nil
}

func (c *context) getItemsParseJSONResponse(response *v3io.Response, getItemsInput *v3io.GetItemsInput) (*v3io.GetItemsOutput, error) {

	getItemsResponse := struct {
		Items            []map[string]map[string]interface{}
		NextMarker       string
		LastItemIncluded string
	}{}

	// unmarshal the body into an ad hoc structure
	err := json.Unmarshal(response.Body(), &getItemsResponse)
	if err != nil {
		return nil, err
	}

	//validate getItems response to avoid infinite loop
	if getItemsResponse.LastItemIncluded != "TRUE" && (getItemsResponse.NextMarker == "" || getItemsResponse.NextMarker == getItemsInput.Marker) {
		errMsg := fmt.Sprintf("Invalid getItems response: lastItemIncluded=false and nextMarker='%s', "+
			"startMarker='%s', probably due to object size bigger than 2M. Query is: %+v", getItemsResponse.NextMarker, getItemsInput.Marker, getItemsInput)
		c.logger.Warn(errMsg)
	}

	getItemsOutput := v3io.GetItemsOutput{
		NextMarker: getItemsResponse.NextMarker,
		Last:       getItemsResponse.LastItemIncluded == "TRUE",
	}

	// iterate through the items and decode them
	for _, typedItem := range getItemsResponse.Items {

		item, err := c.decodeTypedAttributes(typedItem)
		if err != nil {
			return nil, err
		}

		getItemsOutput.Items = append(getItemsOutput.Items, item)
	}
	// attach the output to the response
	return &getItemsOutput, nil
}

func (c *context) getItemsParseCAPNPResponse(response *v3io.Response, withWildcard bool) (*v3io.GetItemsOutput, error) {
	responseBodyReader := bytes.NewReader(response.Body())
	capnpSections := readAllCapnpMessages(responseBodyReader)
	if len(capnpSections) < 2 {
		return nil, errors.Errorf("getItemsCapnp: Got only %v capnp sections. Expecting at least 2", len(capnpSections))
	}
	cookie := string(response.HeaderPeek("X-v3io-cookie"))
	getItemsOutput := v3io.GetItemsOutput{
		NextMarker: cookie,
		Last:       len(cookie) == 0,
	}
	if len(capnpSections) < 2 {
		return nil, errors.Errorf("getItemsCapnp: Got only %v capnp sections. Expecting at least 2", len(capnpSections))
	}

	metadataPayload, err := node_common_capnp.ReadRootVnObjectItemsGetResponseMetadataPayload(capnpSections[len(capnpSections)-1])
	if err != nil {
		return nil, errors.Wrap(err, "ReadRootVnObjectItemsGetResponseMetadataPayload")
	}
	//Keys
	attributeMap, err := metadataPayload.KeyMap()
	if err != nil {
		return nil, errors.Wrap(err, "metadataPayload.KeyMap")
	}
	attributeMapNames, err := attributeMap.Names()
	if err != nil {
		return nil, errors.Wrap(err, "attributeMap.Names")
	}
	attributeNamesPtr, err := attributeMapNames.Arr()
	if err != nil {
		return nil, errors.Wrap(err, "attributeMapNames.Arr")
	}
	//Values
	valueMap, err := metadataPayload.ValueMap()
	if err != nil {
		return nil, errors.Wrap(err, "metadataPayload.ValueMap")
	}
	values, err := valueMap.Values()
	if err != nil {
		return nil, errors.Wrap(err, "valueMap.Values")
	}

	// Items
	items, err := metadataPayload.Items()
	if err != nil {
		return nil, errors.Wrap(err, "metadataPayload.Items")
	}
	valuesSections := make([]attributeValuesSection, len(capnpSections)-1)

	accLength := 0
	//Additional data sections "in between"
	for capnpSectionIndex := 1; capnpSectionIndex < len(capnpSections)-1; capnpSectionIndex++ {
		data, err := node_common_capnp.ReadRootVnObjectAttributeValueMap(capnpSections[capnpSectionIndex])
		if err != nil {
			return nil, errors.Wrap(err, "node_common_capnp.ReadRootVnObjectAttributeValueMap")
		}
		dv, err := data.Values()
		if err != nil {
			return nil, errors.Wrap(err, "data.Values")
		}
		accLength = accLength + dv.Len()
		valuesSections[capnpSectionIndex-1].data = dv
		valuesSections[capnpSectionIndex-1].accumulatedPreviousSectionsLength = accLength
	}
	accLength = accLength + values.Len()
	valuesSections[len(capnpSections)-2].data = values
	valuesSections[len(capnpSections)-2].accumulatedPreviousSectionsLength = accLength

	//Read in all the attribute names
	attributeNamesNumber := attributeNamesPtr.Len()
	attributeNames := make([]string, attributeNamesNumber)
	for attributeIndex := 0; attributeIndex < attributeNamesNumber; attributeIndex++ {
		attributeNames[attributeIndex], err = attributeNamesPtr.At(attributeIndex).Str()
		if err != nil {
			return nil, errors.Wrapf(err, "attributeNamesPtr.At(%d) size %d", attributeIndex, attributeNamesNumber)
		}
	}

	// iterate through the items and decode them
	for itemIndex := 0; itemIndex < items.Len(); itemIndex++ {
		itemPtr := items.At(itemIndex)
		item, err := itemPtr.Item()
		if err != nil {
			return nil, errors.Wrap(err, "itemPtr.Item")
		}
		itemAttributes, err := item.Attrs()
		if err != nil {
			return nil, errors.Wrap(err, "item.Attrs")
		}
		ditem, err := decodeCapnpAttributes(itemAttributes, valuesSections, attributeNames)
		if err != nil {
			return nil, errors.Wrap(err, "decodeCapnpAttributes")
		}
		if withWildcard {
			name, err := item.Name()
			if err != nil {
				return nil, errors.Wrap(err, "item.Name")
			}
			ditem["__name"] = name
		}
		getItemsOutput.Items = append(getItemsOutput.Items, ditem)
	}
	return &getItemsOutput, nil
}
