# nats: NATS Trigger

Reads messages from [NATS](https://nats.io/) topics.

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| topic | string | The topic on which to listen on |

### Example

```yaml
triggers:
  myNatsTopic:
    kind: "nats"
    url: "10.0.0.3:4222"
    attributes:
      "topic": "my.topic"
```
