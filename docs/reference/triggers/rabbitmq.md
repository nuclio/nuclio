# RabbitMQ trigger

> **NOTE:**  RabbitMQ trigger is in tech-preview.

Reads messages from [RabbitMQ](https://www.rabbitmq.com/) queues.

## Trigger Configuration

| **Path** | **Type** | **Description**                                                                                                                                                                                                  |
|:---------|:---------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| url      | string   | The RabbitMQ connection URL in AMQP format (e.g., `amqp://host:port` or `amqp://user:pass@host:port`). If credentials are included in the URL, they will be automatically extracted and used for authentication. |
| username | string   | The username for RabbitMQ authentication. Can also be provided in the URL. If username is provided in `url`, it takes precedence.                                                                                |
| password | string   | The password for RabbitMQ authentication. Can also be provided in the URL. If password is provided in `url`, it takes precedence.                                                                                |

## Attributes

| **Path**          | **Type**           | **Description**                                                                                                                                                                      |
|:------------------|:-------------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| exchangeName      | string             | The exchange that contains the queue                                                                                                                                                 |
| queueName         | string             | If specified, the trigger reads messages from this queue                                                                                                                             |
| topics            | list of strings    | If specified, the trigger creates a queue with a unique name and subscribes it to these topics                                                                                       |
| reconnectDuration | string of duration | The timeout when trying to reconnect to RabbitMQ. Default is 5 minutes.                                                                                                              |
| reconnectInterval | string of duration | The interval to wait before reconnecting to RabbitMQ. Default is 15 seconds.                                                                                                         |
| prefetchCount     | int                | The prefetch count of the broker channel. A value of 0 means unlimited (no prefetch limit), allowing the broker to send as many messages as possible. Default is 0.                  |
| durableExchange   | bool               | Define if the exchange is durable. Default is false.                                                                                                                                 |
| durableQueue      | bool               | Define if the queue is durable. Default is false.                                                                                                                                    |
| onError           | string             | Determines the behaviour when a message processing error occurs. Possible values: `"ack"` (acknowledge and remove) or `"nack"` (reject and optionally requeue). Default is `"nack"`. |
| requeueOnError    | bool               | If `true`, messages that fail processing are requeued when `onError` is set to `"nack"`. Default is false.                                                                           |

### Queue and Exchange Configuration

If `queueName` is not specified in the attributes, it defaults to `nuclio-{namespace}-{function-name}`. If the specified queue (or the default queue name) doesn't exist, it will be created automatically.

If `topics` are not specified, no exchange or bindings are created. If `topics` are specified, the trigger creates a topic-type exchange (if it doesn't exist) and binds the queue to the specified routing keys (topics).

> **Note:** When `topics` is specified, the exchange type is always `"topic"`. This trigger only works with topic-type exchanges when automatically creating resources. If you need to use other exchange types (`direct`, `fanout`, `headers`), you must create the exchange and queue manually and set only `queueName` (without `topics`).

#### Queue Creation Parameters

When a queue is created automatically, it is created with the following parameters:

- **Queue name**: `queueName` (if specified) or `nuclio-{namespace}-{function-name}` (if not specified)
- **Durable**: `durableQueue` (default: `false`)
- **Delete when unused**: `false`
- **Exclusive**: `false`
- **No-wait**: `false`
- **Arguments**: `nil`

#### Exchange Creation Parameters

When an exchange is created automatically (only when `topics` is specified), it is created with the following parameters:

- **Exchange name**: `exchangeName`
- **Exchange type**: `"topic"` (hardcoded)
- **Durable**: `durableExchange` (default: `false`)
- **Auto-delete**: `false`
- **Internal**: `false`
- **No-wait**: `false`
- **Arguments**: `nil`

> **Note:** when running in Kubernetes / docker, the consumer name is the host name (pod name, e.g.: `my-pod-1234`)
> and the connection name is consisted of `nuclio-<func-name>-<trigger-name>` to allow differentiation between multiple functions
> consuming from the same server.

### Example

```yaml
triggers:
  myRabbit:
    kind: "rabbit-mq"
    url: "amqp://user:pass@10.0.0.1:5672"
    attributes:
      exchangeName: "myExchangeName"
      queueName: "myQueueName"
      reconnectDuration: "10m"
      reconnectInterval: "60s"
      prefetchCount: 1
      durableExchange: true
      durableQueue: true
      onError: "nack"
      requeueOnError: true
```
OR

```yaml
triggers:
  myRabbit:
    kind: "rabbit-mq"
    url: "amqp://10.0.0.1:5672"
    username: "user"
    password: "pass"
    attributes:
      exchangeName: "myExchangeName"
      queueName: "myQueueName"
      reconnectDuration: "10m"
      reconnectInterval: "60s"
      prefetchCount: 1
      durableExchange: true
      durableQueue: true
      onError: "nack"
      requeueOnError: true
```

Both configurations are supported.
During the enrichment stage, if credentials are provided within the URL, they are automatically extracted and assigned to `username` and `password` fields in the trigger configuration.
The URL is then sanitized (i.e., credentials are removed) to prevent sensitive data from being exposed in logs or configurations.

> Note: If both the URL and the trigger configuration specify credentials, the credentials from the URL take precedence and will override any existing username or password values in the trigger specification.