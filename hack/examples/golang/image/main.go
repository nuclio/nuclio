/*
Copyright 2023 The Nuclio Authors.

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

// Nuclio image conversion and resizing example
// Usage: send an HTTP Post request with the body containing a URL of an image or the actual image binary
//        can specify requested size and format via the URL query e.g.: /?x=50&y=50&format=png

package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"

	"github.com/nuclio/nuclio-sdk-go"
	"golang.org/x/image/draw"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {

	// Set default values
	x := 100
	y := 100
	respType := "image/jpeg"

	// Extract X, Y, Format from query args
	if xval, err := event.GetFieldInt("x"); err == nil {
		x = xval
	}

	if yval, err := event.GetFieldInt("y"); err == nil {
		y = yval
	}

	if format := event.GetFieldString("format"); format == "png" {
		respType = "image/png"
	}

	context.Logger.DebugWith("Got request", "path", event.GetPath(), "x", x, "y", y, "format", respType, "ctype", event.GetContentType())

	var img image.Image
	var err error
	if strings.HasPrefix(event.GetContentType(), "text/plain") {
		// if the body is text assume its a URL and read the image from the URL (in the text)
		response, err := http.Get(string(event.GetBody()))
		if err != nil {
			return nil, err
		}
		// Try to decode the returned body (from the HTTP request to the provided URL)
		img, _, err = image.Decode(response.Body)
	} else {
		// if the content is not text assume the Body contains the image and decode it
		r := bytes.NewReader(event.GetBody())
		img, _, err = image.Decode(r)
	}

	// If image Decode failed return an error
	if err != nil {
		context.Logger.Error("Failed to open image  %v", err)
		return nil, err
	}

	// Create a thumbnail with the specified size and format
	targetRect := image.Rect(0, 0, x, y)
	thumb := image.NewRGBA(targetRect)
	draw.CatmullRom.Scale(thumb, targetRect, img, img.Bounds(), draw.Over, nil)
	buf := new(bytes.Buffer)
	if respType == "image/png" {
		err = png.Encode(buf, thumb)
	} else {
		err = jpeg.Encode(buf, thumb, &jpeg.Options{Quality: 95})
	}

	// Return a response with an image and the proper Content Type
	return nuclio.Response{
		StatusCode:  200,
		ContentType: respType,
		Body:        buf.Bytes(),
	}, nil

}
