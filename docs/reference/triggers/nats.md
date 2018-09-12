# nats: NATS Trigger

Reads messages from [NATS](https://nats.io/) topics. Function replicas are subscribed to a worker group (queue), and messaged are load-balanced across replicas. To join a specific worker group, specify a queue name attribute in the trigger configuration.

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| topic | string | The topic on which to listen |
| queueName | string | The name of a shared worker queue to join (defaults to an auto-generated name per trigger) |

### Example

```yaml
triggers:
  myNatsTopic:
    kind: "nats"
    url: "10.0.0.3:4222"
    attributes:
      "topic": "my.topic"
      "queue": "my-worker-queue"
```
