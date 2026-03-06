# NATS trigger

> **NOTE:**  NATS trigger is in tech-preview.

Reads messages from [NATS](https://nats.io/) topics. Function replicas are subscribed to a worker group (queue), and messages are load-balanced across replicas. To join a specific worker group, specify a queue-name attribute in the trigger configuration.

The queue name may be a Go template, which may include any of the following fields:

| **Name** | **Type** | **Description** |
| :--- | :--- | :--- |
| Id | string |The trigger id |
| Namespace | string | The function deployment namespace |
| Name | string | The deployed function name |
| Labels | map | Labels specified in the function metadata |
| Annotations | map | Annotations specified in the function metadata |

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| topic | string | The topic on which to listen. |
| queueName | string | The name of a shared worker queue to join; (default: an auto-generated name per trigger). |
| reply | bool | When set to true, publish the handler response body to the incoming message reply subject (`msg.Reply`) if present. See [Reply encoding](#reply-encoding) for details. |

### Reply encoding

When `reply` is enabled, the trigger publishes the handler's return value to the
NATS reply subject using the following rules:

| **Response body type** | **Behavior** |
| :--- | :--- |
| `[]byte` | Published as-is. |
| `string` | Converted to bytes and published. |
| Any other type | JSON-serialized before publishing. |
| `nil` | An empty message (no payload) is published. |
| *(no reply subject)* | If the incoming NATS message has no reply subject (i.e. it was sent with `Publish` rather than `Request`), no reply is published regardless of this setting. |

### Example

```yaml
triggers:
  myNatsTopic:
    kind: "nats"
    url: "nats://10.0.0.3:4222"
    attributes:
      "topic": "my.topic"
      "queueName": "{{ .Namespace }}.{{ .Name }}.{{ .Id }}"
      "reply": true
```
