/*Package python implement Python runtime

The Go Python runtime opens a Unix socket and starts the wrapper Python script
(`wrapper.py`) with path to the socket and the entry point to run. The Python
wrapper connects to this socket upon startup.

The wite protocol is line oriented where every line is a JSON object.
- Go sends events (encoded using `EventJSONEncoder`)
- Python sends
    - Log messages (JSON formatted log records, see `JSONFormatter` in `wrapper.py`)
    - Handler output encoded as JSON in the format `{"handler_output": <data>}`
*/
package python
