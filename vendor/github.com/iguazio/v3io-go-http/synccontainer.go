package v3io

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"

	"encoding/json"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"reflect"
	"strconv"
	"strings"
)

// function names
const (
	setObjectFunctionName  = "ObjectSet"
	putItemFunctionName    = "PutItem"
	updateItemFunctionName = "UpdateItem"
	getItemFunctionName    = "GetItem"
	getItemsFunctionName   = "GetItems"
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
		Last:       getItemsResponse.LastItemIncluded == "Y",
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

func (sc *SyncContainer) PutItem(input *PutItemInput) error {

	// prepare the query path
	_, err := sc.postItem(input.Path, putItemFunctionName, input.Attributes, putItemHeaders, nil)
	return err
}

func (sc *SyncContainer) UpdateItem(input *UpdateItemInput) error {
	var err error

	if input.Attributes != nil {

		// specify update mode as part of body. "Items" will be injected
		body := map[string]interface{}{
			"UpdateMode": "CreateOrReplaceAttributes",
		}

		_, err = sc.postItem(input.Path, putItemFunctionName, input.Attributes, updateItemHeaders, body)

	} else if input.Expression != nil {

		_, err = sc.putItem(input.Path, putItemFunctionName, *input.Expression, updateItemHeaders)
	}

	return err
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
		err = fmt.Errorf("Failed GET with status %d", response.response.StatusCode())
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
