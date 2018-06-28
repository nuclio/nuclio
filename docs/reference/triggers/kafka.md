# kafka: Kafka Trigger

Reads records from [Apache Kafka](https://kafka.apache.org/) streams.

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| topic | string | The topic on which to listen on |
| partitions | list of int | List of partitions on which this function receives events |
| driver | object | Driver configuration. See [here]()|

### Example

```yaml
triggers:
  myKafkaTopic:
    kind: "kafka"
    url: "10.0.0.2"
    attributes:
      topic: "my.topic"
      partitions: [0, 5, 10]
      driver:
        Network:
          SASL:
            User: "iguazio"
            Password: "t0ps3cr3t"
```
