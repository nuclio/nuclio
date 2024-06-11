# Event batching

:bangbang: Feature is supported only for:
* runtimes:
    * `python`
* trigger kinds:
    * `http`

The event batching feature allows passing a batch of events, instead of a single event, to a function handler.
The function invocation remains the same as in the usual flow. The processor keeps each invocation (or event) until 
a batch is ready to be sent. The batched event is sent to the runtime when either the batch is full or the batching
timeout has passed. Both `batchSize` and `timeout` are configured per trigger.

Example of trigger configuration with batching enabled:

```yaml
  triggers:
    http:
      class: ""
      kind: http
      name: http
      batch:
        mode: enable
        batchSize: 10
        timeout: 1s
      maxWorkers: 10
```

A crucial difference from a single event flow lies in the necessity to manage responses for each event in a batch. 
Every event in a batch has an `event_id` field, which should be used in the response when defining the corresponding response for each single event in a batched event.
The processor uses `response.event_id` to determine the correct target for sending each individual response.
If `response.event_id` isn't set, the user won't receive a response.

Handler example:

```python
import nuclio_sdk
def handler(context, batch: list[nuclio_sdk.Event]):
    context.logger.info_with('Got batched event!')
    batched_response = []
    response = batch_processing(batch)
    for item in batch:
        event_id = item.id
        batched_response.append(nuclio_sdk.Response(
            body=response,
            headers={},
            content_type="text",
            status_code=200,
            event_id=event_id,
        ))
    return batched_response

def batch_processing(event: list[nuclio_sdk.Event]):
    return "Hello"
```