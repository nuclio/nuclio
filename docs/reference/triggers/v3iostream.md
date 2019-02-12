# v3ioStream: v3io Stream Trigger

Reads records from [Iguazio Continuous Data Platform](https://www.iguazio.com) v3io streams.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| partitions | list of int | List of partitions on which this function receives events |
| seekTo | string | The location within the stream from which to start reading - `"earliest"` or `"latest"`; (defaults to `"latest"`) |
| readBatchSize | int | The number of records to read from the stream in a single request; (defaults to 64) |
| pollingIntervalMs | int | The duration, in milliseconds, to wait between partition reads; (defaults to 500) |
| username | string | Iguazio Continuous Data Platform username |
| password | string | Iguazio Continuous Data Platform password |

### Example

```yaml
triggers:
  myv3ioStream:
    kind: v3ioStream
    url: http://10.0.0.1:8081/1/v3io-stream-test-baqlmrr9vnp3fmf5fc60
    attributes:
      partitions: [0, 1, 2]
      numContainerWorkers: 1
      seekTo: earliest
      readBatchSize: 64
      pollingIntervalMs: 250
      username: myusername
      password: mypassword
```
