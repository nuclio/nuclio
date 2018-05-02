# v3ioStream: v3io Stream Trigger

Reads records from [iguazio Continuous Data Platform](https://www.iguazio.com) v3io streams.

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| partitions | list of int | List of partitions on which this function receives events |
| numContainerWorkers | int | Number of workers the v3io container uses (defaults to half the number of partitions + 1) |
| seekTo | string | At which point in the stream to read. One of "earliest", "latest" (defaults to "latest") |
| readBatchSize | int | How many records to read from the stream in a single request (defaults to 64) |
| pollingIntervalMs | int | How many milliseconds to wait between reads of the partition (defaults to 500) |

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
```
