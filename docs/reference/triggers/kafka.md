# kafka: Kafka Trigger

Reads records from [Apache Kafka](https://kafka.apache.org/) streams.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| topic | string | The topic on which to listen |
| partitions | list of int | List of partitions on which this function receives events |
| sasl | object | An object with the following attirbutes: `enable` (bool), `user` (string), `password` (string) |

### Example

```yaml
triggers:
  myKafkaTopic:
    kind: kafka-cluster
    attributes:
      initialOffset: earliest
      topics:
        - mytopic
      brokers:
        - 10.0.0.2:9092
      consumerGroup: my-consumer-group
      sasl:
        enable: true
        user: "nuclio"
        password: "s3rv3rl3ss"
```
