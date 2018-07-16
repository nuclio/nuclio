# kafka: Kafka Trigger

Reads records from [Apache Kafka](https://kafka.apache.org/) streams.

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| topic | string | The topic on which to listen on |
| partitions | list of int | List of partitions on which this function receives events |
| sasl | object | Has the following attirbute: `enable` (bool), `user` (string), `password` (string) |

### Example

```yaml
triggers:
  myKafkaTopic:
    kind: "kafka"
    url: "10.0.0.2"
    attributes:
      topic: "my.topic"
      partitions: [0, 5, 10]
      sasl:
        enable: true
        user: "nuclio"
        password: "s3rv3rl3ss"
```
