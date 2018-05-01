# kinesis: Kinesis Trigger

Reads records from [Amazon Kinesis](https://aws.amazon.com/kinesis/) streams.

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| accessKeyID | string | Required by AWS Kinesis |
| secretAccessKey | string | Required by AWS Kinesis |
| regionName | string | Required by AWS Kinesis |
| streamName | string | Required by AWS Kinesis |
| shards | string | List of shards on which this function receives events |

### Example

```yaml
triggers:
  myKinesisStream:
    accessKeyID: "my-key"
    secretAccessKey: "my-secret"
    regionName: "eu-west-1"
    streamName: "my-stream"
    shards: [shard-0, shard-1, shard-2]
```
