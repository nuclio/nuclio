# NATS JetStream trigger

> **NOTE:**  NATS JetStream trigger is in tech-preview.

Reads messages from [NATS](https://docs.nats.io/nats-concepts/jetstream) streams. Function replicas are subscribed to a consumer, and messages are load-balanced across replicas. Messages can wait to be consumed if no function replica is available.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| stream | string | The name of the JetStream stream. |
| consumer | string | The name of the consumer associated to the stream. |

### Optional
| **Path** | **Type** | **Default** | **Description** |
| :--- | :--- | :--- | :--- |
| allowReconnect | bool | false | Enables reconnection logic to be used when we encounter a disconnect from the current server. |
| maxReconnect | int | 60 | Sets the number of reconnect attempts that will be tried before giving up. If negative, then it will never give up trying to reconnect. |
| reconnectWait | duration | 2s | Sets the time to backoff after attempting a reconnect to a server that we were already connected to previously. |
| reconnectJitter | duration | 100ms | Sets the upper bound for a random delay added to reconnectWait during a reconnect when no TLS is used. |
| timeout | duration | 2s | Sets the timeout for a Dial operation on a connection. |
| drainTimeout | duration | 30s | Sets the timeout for a Drain Operation to complete. |
| flusherTimeout | duration | 1m | The maximum time to wait for write operations to the underlying connection to complete (including the flusher loop). |
| pingInterval | duration | 2m | The period at which the client will be sending ping commands to the server, disabled if 0 or negative. |
| maxPingsOut | int | 2 | The maximum number of pending ping commands that can be awaiting a response before raising an ErrStaleConnection error. |

### Example

```yaml
triggers:
  myNatsJetStream:
    kind: "natsjetstream"
    url: "nats://10.0.0.3:4222"
    attributes:
      stream: "mystream"
      consumer: "myconsumer"
      # optional
      allowReconnect: true
      maxReconnect: -1
      reconnectWait: "2m"
      reconnectJitter: "200ms"
      timeout: "5s"
      drainTimeout: "1m"
      flusherTimeout: "45s"
      pingInterval: "1m"
      maxPingsOut: 3
```
