# NATS JetStream trigger

> **NOTE:**  NATS JetStream trigger is in tech-preview.

Reads messages from [NATS](https://docs.nats.io/nats-concepts/jetstream) streams. Function replicas are subscribed to a consumer, and messages are load-balanced across replicas. Messages can wait to be consumed if no function replica is available.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| stream | string | The name of the JetStream stream. |
| consumer | string | The name of the consumer associated to the stream. |

### Example

```yaml
triggers:
  myNatsJetStream:
    kind: "natsjetstream"
    url: "nats://10.0.0.3:4222"
    attributes:
      "stream": "mystream"
      "consumer": "myconsumer"
```
