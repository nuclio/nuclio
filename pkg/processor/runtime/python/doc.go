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

/*
Package python implement Python runtime

The Go Python runtime opens a Unix socket and starts the wrapper Python script
(`wrapper.py`) with path to the socket and the handler to run. The Python
wrapper connects to this socket upon startup.

The wite protocol is line oriented where every line is a JSON object.
- Go sends events (encoded using `EventJSONEncoder`)
- Python sends lines starting with letter specifying type and then JSON object
    - 'r' Handler reply
    - 'l' Log messages
	- 'm' Metric messages
*/

// Package python implmenets Python runtime
package python
