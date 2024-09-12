# RabbitMQ trigger

Reads messages from [RabbitMQ](https://www.rabbitmq.com/) queues.

## Attributes

| **Path**          | **Type**           | **Description**                                                                                |
|:------------------|:-------------------|:-----------------------------------------------------------------------------------------------|
| exchangeName      | string             | The exchange that contains the queue                                                           |
| queueName         | string             | If specified, the trigger reads messages from this queue                                       |
| topics            | list of strings    | If specified, the trigger creates a queue with a unique name and subscribes it to these topics |
| reconnectDuration | string of duration | The duration to wait before reconnecting to RabbitMQ. Default is 5 minutes.                    |
| reconnectInterval | string of duration | The interval to wait before reconnecting to RabbitMQ. Default is 11 seconds.                   |
| prefetchCount     | int                | The prefetch count of the broker channel. Default is 0.                                        |
| durableExchange   | bool               | Define if the exchange is durable. Default is false.                                           |
| durableQueue      | bool               | Define if the queue is durable. Default is false.                                              |

> **Note:** `topics` and `queueName` are mutually exclusive.
> The trigger can either create to an existing queue specified by `queueName` or create its own queue, subscribing it to `topics` 

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
```
