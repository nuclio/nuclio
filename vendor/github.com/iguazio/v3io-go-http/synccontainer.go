package v3io

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
)

// function names
const (
	setObjectFunctionName    = "ObjectSet"
	putItemFunctionName      = "PutItem"
	updateItemFunctionName   = "UpdateItem"
	getItemFunctionName      = "GetItem"
	getItemsFunctionName     = "GetItems"
	createStreamFunctionName = "CreateStream"
	putRecordsFunctionName   = "PutRecords"
	getRecordsFunctionName   = "GetRecords"
	seekShardsFunctionName   = "SeekShard"
)

// headers for set object
var setObjectHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": setObjectFunctionName,
}

// headers for put item
var putItemHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": putItemFunctionName,
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

// headers for update item
var getItemsHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": getItemsFunctionName,
}

// headers for create stream
var createStreamHeaders = map[string]string{
	"Content-Type":    "application/json",
	"X-v3io-function": createStreamFunctionName,
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

type SyncContainer struct {
	logger    Logger
	session   *SyncSession
	alias     string
	uriPrefix string
}

func newSyncContainer(parentLogger Logger, session *SyncSession, alias string) (*SyncContainer, error) {
	return &SyncContainer{
		logger:    parentLogger.GetChild(alias).(Logger),
		session:   session,
		alias:     alias,
		uriPrefix: fmt.Sprintf("http://%s/%s", session.context.clusterURL, alias),
	}, nil
}

func (sc *SyncContainer) ListAll() (*Response, error) {
	output := ListAllOutput{}

	return sc.sendRequestAndXMLUnmarshal("GET", sc.uriPrefix, nil, nil, &output)
}

func (sc *SyncContainer) ListBucket(input *ListBucketInput) (*Response, error) {
	output := ListBucketOutput{}

	// prepare the query path
	fullPath := sc.uriPrefix
	if input.Path != "" {
		fullPath += "?prefix=" + input.Path
	}

	return sc.sendRequestAndXMLUnmarshal("GET", fullPath, nil, nil, &output)
}

func (sc *SyncContainer) GetObject(input *GetObjectInput) (*Response, error) {
	response, err := sc.sendRequest("GET", sc.getPathURI(input.Path), nil, nil, false)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send request")
	}

	return response, nil
}

func (sc *SyncContainer) DeleteObject(input *DeleteObjectInput) error {
	_, err := sc.sendRequest("DELETE", sc.getPathURI(input.Path), nil, nil, true)
	if err != nil {
		return errors.Wrap(err, "Failed to send request")
	}

	return nil
}

func (sc *SyncContainer) PutObject(input *PutObjectInput) error {
	_, err := sc.sendRequest("PUT", sc.getPathURI(input.Path), nil, input.Body, true)
	if err != nil {
		return errors.Wrap(err, "Failed to send request")
	}

	return nil
}

func (sc *SyncContainer) GetItem(input *GetItemInput) (*Response, error) {

	// no need to marshal, just sprintf
	body := fmt.Sprintf(`{"AttributesToGet": "%s"}`, strings.Join(input.AttributeNames, ","))

	response, err := sc.sendRequest("POST", sc.getPathURI(input.Path), getItemHeaders, []byte(body), false)
	if err != nil {
		return nil, err
	}

	// ad hoc structure that contains response
	item := struct {
		Item map[string]map[string]string
	}{}

	sc.logger.InfoWith("Body", "body", string(response.Body()))

	// unmarshal the body
	err = json.Unmarshal(response.Body(), &item)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal get item")
	}

	// decode the response
	attributes, err := sc.decodeTypedAttributes(item.Item)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	// attach the output to the response
	response.Output = &GetItemOutput{attributes}

	return response, nil
}

func (sc *SyncContainer) GetItems(input *GetItemsInput) (*Response, error) {

	// create GetItem Body
	body := map[string]interface{}{
		"AttributesToGet": strings.Join(input.AttributeNames, ","),
	}

	if input.Filter != "" {
		body["FilterExpression"] = input.Filter
	}

	if input.Marker != "" {
		body["Marker"] = input.Marker
	}

	if input.Limit != 0 {
		body["Limit"] = input.Limit
	}

	if input.TotalSegments != 0 {
		body["TotalSegment"] = input.TotalSegments
		body["Segment"] = input.Segment
	}

	marshalledBody, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal body")
	}

	response, err := sc.sendRequest("POST",
		sc.getPathURI(input.Path),
		getItemsHeaders,
		[]byte(marshalledBody),
		false)

	if err != nil {
		return nil, err
	}

	sc.logger.InfoWith("Body", "body", string(response.Body()))

	getItemsResponse := struct {
		Items            []map[string]map[string]string
		NextMarker       string
		LastItemIncluded string
	}{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &getItemsResponse)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal get item")
	}

	getItemsOutput := GetItemsOutput{
		NextMarker: getItemsResponse.NextMarker,
		Last:       getItemsResponse.LastItemIncluded == "TRUE",
	}

	// iterate through the items and decode them
	for _, typedItem := range getItemsResponse.Items {

		item, err := sc.decodeTypedAttributes(typedItem)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to decode attributes")
		}

		getItemsOutput.Items = append(getItemsOutput.Items, item)
	}

	// attach the output to the response
	response.Output = &getItemsOutput

	return response, nil
}

func (sc *SyncContainer) GetItemsCursor(input *GetItemsInput) (*SyncItemsCursor, error) {
	response, err := sc.GetItems(input)
	if err != nil {
		return nil, err
	}

	return newSyncItemsCursor(sc, input, response), nil
}

func (sc *SyncContainer) PutItem(input *PutItemInput) error {
	var body map[string]interface{}

	// create body if required w/condition
	body = sc.encodeConditionExpression(input.Condition, body)

	// prepare the query path
	_, err := sc.postItem(input.Path, putItemFunctionName, input.Attributes, putItemHeaders, body)
	return err
}

func (sc *SyncContainer) PutItems(input *PutItemsInput) (*Response, error) {
	response := sc.allocateResponse()
	if response == nil {
		return nil, errors.New("Failed to allocate response")
	}

	putItemsOutput := PutItemsOutput{
		Success: true,
	}

	for itemKey, itemAttributes := range input.Items {

		// try to post the item
		_, err := sc.postItem(input.Path+"/"+itemKey, putItemFunctionName, itemAttributes, putItemHeaders, nil)

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

func (sc *SyncContainer) UpdateItem(input *UpdateItemInput) error {
	var err error

	var body map[string]interface{}

	if input.Attributes != nil {

		// specify update mode as part of body. "Items" will be injected
		body = map[string]interface{}{
			"UpdateMode": "CreateOrReplaceAttributes",
		}

		// set condition to body, if required
		body = sc.encodeConditionExpression(input.Condition, body)

		_, err = sc.postItem(input.Path, putItemFunctionName, input.Attributes, updateItemHeaders, body)

	} else if input.Expression != nil {

		// set condition to body, if required
		body = sc.encodeConditionExpression(input.Condition, body)

		_, err = sc.putItem(input.Path, putItemFunctionName, *input.Expression, updateItemHeaders)
	}

	return err
}

func (sc *SyncContainer) CreateStream(input *CreateStreamInput) error {
	body := fmt.Sprintf(`{"ShardCount": %d, "RetentionPeriodHours": %d}`,
		input.ShardCount,
		input.RetentionPeriodHours)

	_, err := sc.sendRequest("POST", sc.getPathURI(input.Path), createStreamHeaders, []byte(body), true)
	if err != nil {
		return errors.Wrap(err, "Failed to send request")
	}

	return nil
}

func (sc *SyncContainer) DeleteStream(input *DeleteStreamInput) error {

	// get all shards in the stream
	response, err := sc.ListBucket(&ListBucketInput{
		Path: input.Path,
	})

	if err != nil {
		return errors.Wrap(err, "Failed to list shards in stream")
	}

	defer response.Release()

	// delete the shards one by one
	for _, content := range response.Output.(*ListBucketOutput).Contents {

		// TODO: handle error - stop deleting? return multiple errors?
		sc.DeleteObject(&DeleteObjectInput{
			Path: content.Key,
		})
	}

	// delete the actual stream
	return sc.DeleteObject(&DeleteObjectInput{
		Path: path.Dir(input.Path) + "/",
	})
}

func (sc *SyncContainer) PutRecords(input *PutRecordsInput) (*Response, error) {

	// TODO: set this to an initial size through heuristics?
	// This function encodes manually
	var buffer bytes.Buffer

	buffer.WriteString(`{"Records": [`)

	for recordIdx, record := range input.Records {
		buffer.WriteString(`{"Data": "`)
		buffer.WriteString(base64.StdEncoding.EncodeToString(record.Data))
		buffer.WriteString(`"`)

		if record.ShardID != nil {
			buffer.WriteString(`, "ShardId": `)
			buffer.WriteString(strconv.Itoa(*record.ShardID))
		}

		// add comma if not last
		if recordIdx != len(input.Records)-1 {
			buffer.WriteString(`}, `)
		} else {
			buffer.WriteString(`}`)
		}
	}

	buffer.WriteString(`]}`)
	str := string(buffer.Bytes())
	fmt.Println(str)

	response, err := sc.sendRequest("POST", sc.getPathURI(input.Path), putRecordsHeaders, buffer.Bytes(), false)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send request")
	}

	putRecordsOutput := PutRecordsOutput{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &putRecordsOutput)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal put record")
	}

	// set the output in the response
	response.Output = &putRecordsOutput

	return response, nil
}

func (sc *SyncContainer) SeekShard(input *SeekShardInput) (*Response, error) {
	var buffer bytes.Buffer

	buffer.WriteString(`{"Type": "`)
	buffer.WriteString(seekShardsInputTypeToString[input.Type])
	buffer.WriteString(`"`)

	if input.Type == SeekShardInputTypeSequence {
		buffer.WriteString(`, "StartingSequenceNumber": `)
		buffer.WriteString(strconv.Itoa(input.StartingSequenceNumber))
	} else if input.Type == SeekShardInputTypeTime {
		buffer.WriteString(`, "TimeStamp": `)
		buffer.WriteString(strconv.Itoa(input.Timestamp))
	}

	buffer.WriteString(`}`)

	response, err := sc.sendRequest("POST", sc.getPathURI(input.Path), seekShardsHeaders, buffer.Bytes(), false)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send request")
	}

	seekShardOutput := SeekShardOutput{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &seekShardOutput)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal seek shard")
	}

	// set the output in the response
	response.Output = &seekShardOutput

	return response, nil
}

func (sc *SyncContainer) GetRecords(input *GetRecordsInput) (*Response, error) {
	body := fmt.Sprintf(`{"Location": "%s", "Limit": %d}`,
		input.Location,
		input.Limit)

	response, err := sc.sendRequest("POST", sc.getPathURI(input.Path), getRecordsHeaders, []byte(body), false)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to send request")
	}

	getRecordsOutput := GetRecordsOutput{}

	// unmarshal the body into an ad hoc structure
	err = json.Unmarshal(response.Body(), &getRecordsOutput)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal get records")
	}

	// set the output in the response
	response.Output = &getRecordsOutput

	return response, nil
}

func (sc *SyncContainer) postItem(path string,
	functionName string,
	attributes map[string]interface{},
	headers map[string]string,
	body map[string]interface{}) (*Response, error) {

	// iterate over all attributes and encode them with their types
	typedAttributes, err := sc.encodeTypedAttributes(attributes)
	if err != nil {
		return nil, err
	}

	// create an empty body if the user didn't pass anything
	if body == nil {
		body = map[string]interface{}{}
	}

	// set item in body (use what the user passed as a base)
	body["Item"] = typedAttributes

	jsonEncodedBodyContents, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal body contents")
	}

	return sc.sendRequest("POST", sc.getPathURI(path), headers, jsonEncodedBodyContents, false)
}

func (sc *SyncContainer) putItem(path string,
	functionName string,
	expression string,
	headers map[string]string) (*Response, error) {

	body := map[string]interface{}{
		"UpdateExpression": expression,
		"UpdateMode":       "CreateOrReplaceAttributes",
	}

	jsonEncodedBodyContents, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal body contents")
	}

	return sc.sendRequest("PUT", sc.getPathURI(path), headers, jsonEncodedBodyContents, false)
}

// {"age": 30, "name": "foo"} -> {"age": {"N": 30}, "name": {"S": "foo"}}
func (sc *SyncContainer) encodeTypedAttributes(attributes map[string]interface{}) (map[string]map[string]string, error) {
	typedAttributes := make(map[string]map[string]string)

	for attributeName, attributeValue := range attributes {
		typedAttributes[attributeName] = make(map[string]string)
		switch value := attributeValue.(type) {
		default:
			return nil, fmt.Errorf("Unexpected attribute type for %s: %T", attributeName, reflect.TypeOf(attributeValue))
		case int:
			typedAttributes[attributeName]["N"] = strconv.Itoa(value)
			// this is a tmp bypass to the fact Go maps Json numbers to float64
		case float64:
			typedAttributes[attributeName]["N"] = strconv.FormatFloat(value, 'E', -1, 64)
		case string:
			typedAttributes[attributeName]["S"] = value
		case []byte:
			typedAttributes[attributeName]["B"] = base64.StdEncoding.EncodeToString(value)
		}
	}

	return typedAttributes, nil
}

// {"age": {"N": 30}, "name": {"S": "foo"}} -> {"age": 30, "name": "foo"}
func (sc *SyncContainer) decodeTypedAttributes(typedAttributes map[string]map[string]string) (map[string]interface{}, error) {
	var err error
	attributes := map[string]interface{}{}

	for attributeName, typedAttributeValue := range typedAttributes {

		// try to parse as number
		if numberValue, ok := typedAttributeValue["N"]; ok {

			// try int
			if intValue, err := strconv.Atoi(numberValue); err != nil {

				// try float
				floatValue, err := strconv.ParseFloat(numberValue, 64)
				if err != nil {
					return nil, fmt.Errorf("Value for %s is not int or float: %s", attributeName, numberValue)
				}

				// save as float
				attributes[attributeName] = floatValue
			} else {
				attributes[attributeName] = intValue
			}
		} else if stringValue, ok := typedAttributeValue["S"]; ok {
			attributes[attributeName] = stringValue
		} else if byteSliceValue, ok := typedAttributeValue["B"]; ok {
			attributes[attributeName], err = base64.StdEncoding.DecodeString(byteSliceValue)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to decode %s", attributeName)
			}
		}
	}

	return attributes, nil
}

func (sc *SyncContainer) sendRequest(method string,
	uri string,
	headers map[string]string,
	body []byte,
	releaseResponse bool) (*Response, error) {

	var success bool
	request := fasthttp.AcquireRequest()
	response := sc.allocateResponse()

	// init request
	request.SetRequestURI(uri)
	request.Header.SetMethod(method)
	request.SetBody(body)

	if headers != nil {
		for headerName, headerValue := range headers {
			request.Header.Add(headerName, headerValue)
		}
	}

	// execute the request
	err := sc.session.sendRequest(request, response.response)
	if err != nil {
		err = errors.Wrapf(err, "Failed to send request %s", uri)
		goto cleanup
	}

	// did we get a 2xx response?
	success = response.response.StatusCode() >= 200 && response.response.StatusCode() < 300

	// make sure we got expected status
	if !success {
		err = fmt.Errorf("Failed %s with status %d", method, response.response.StatusCode())
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

func (sc *SyncContainer) sendRequestAndXMLUnmarshal(method string,
	uri string,
	headers map[string]string,
	body []byte,
	output interface{}) (*Response, error) {

	response, err := sc.sendRequest(method, uri, headers, body, false)
	if err != nil {
		return nil, err
	}

	// unmarshal the body into the output
	err = xml.Unmarshal(response.response.Body(), output)
	if err != nil {
		response.Release()

		return nil, errors.Wrap(err, "Failed to unmarshal")
	}

	// set output in response
	response.Output = output

	return response, nil
}

func (sc *SyncContainer) allocateResponse() *Response {
	return &Response{
		response: fasthttp.AcquireResponse(),
	}
}

func (sc *SyncContainer) getContext() *SyncContext {
	return sc.session.context
}

func (sc *SyncContainer) getPathURI(path string) string {
	return sc.uriPrefix + "/" + path
}

// will create the body with the condition expression if body is nil, otherwise
// will simply add the conditional
func (sc *SyncContainer) encodeConditionExpression(conditionExpression *string,
	body map[string]interface{}) map[string]interface{} {

	if conditionExpression == nil {
		return body
	}

	// create the body if it wasn't created yet
	if body == nil {
		body = map[string]interface{}{}
	}

	body["ConditionExpression"] = *conditionExpression

	return body
}
