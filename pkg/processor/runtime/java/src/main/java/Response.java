/*
Copyright 2017 The Nuclio Authors.

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

//package io.nuclio;

import java.util.Map;

public class Response {
    private long statusCode;
    private String contentType;
    private byte[] body;
    private Map<String, Object> headers;

    public Response() {
        this.statusCode = 200;
        this.contentType = "text/plain";
        this.body = null;
        this.headers = null;
    }

    /** Set body
     * @param body Response body
     * @return Response with body set to body
     */
    public Response setBody(byte[] body) {
        this.body = body;
        return this;
    }

    /** Body
     * @return Body
     */
    public byte[] getBody() {
        return body;
    }


    /** Builder method
     * @param statusCode Response status code
     * @return Response with status code set to statusCode
     */
    public Response setStatusCode(long statusCode) {
        this.statusCode = statusCode;
        return this;
    }

    /** Status code
     * @return Status code
     */
    public long getStatusCode() {
        return statusCode;
    }


    /** Builder method
     * @param headers Response headers
     * @return Response with headers set to headers
     */
    public Response setHeaders(Map<String, Object> headers) {
        this.headers = headers;
        return this;
    }

    /** Headers
     * @return headers
     */
    public Map<String, Object> getHeaders() {
        return headers;
    }

    /** Builder method
     * @param contentType Response content type
     * @return Response with content type set to contentType
     */
    public Response setContentType(String contentType) {
        this.contentType = contentType;
        return this;
    }

    /** Content type
     * @return Content type
     */
    public String getContentType() {
        return contentType;
    }

}
