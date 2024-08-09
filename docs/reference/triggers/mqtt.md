# MQTT trigger

Reads messages from [MQTT](https://mqtt.org/) topics.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| subscriptions | subscription (topic, qos) | An MQTT subscription |

### Example

```yaml
triggers:
  myMqttTrigger:
    kind: "mqtt"
    url: "10.0.0.3:1883"
    attributes:
        subscriptions:
        - topic: house/living-room/temperature
          qos: 0
        - topic: weather/humidity
          qos: 0
```

### Event

The MQTT trigger emits an event object with the following attributes:
-  `URL`: The URL of the MQTT broker
- `topic`: The topic of the message
- `path`: The topic of the message (alias for `topic`)
- `body`: The message payload
- `id`: The message id
