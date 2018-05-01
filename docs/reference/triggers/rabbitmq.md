# RabbitMQ trigger

Reads messages from RabbitMQ queues.

## Attributes

| Path | Type | Description | 
| --- | --- | --- |  
| exchangeName | string | The exchange which contains the queue |
| queueName | string | If specified, the trigger reads messages from this queue |
| topics | list of strings | If specified, the trigger creates a queue with a unique name and subscribes it to these topics |

Notes:
1. `topics` and `queueName` are mutually exclusive. The trigger can either create to an existing queue specified by `queueName` or create its own queue, subscribing it to `topics` 

#### Example

```yaml
triggers:
  myNatsTopic:
    kind: "rabbitmq"
    url: "amqp://user:pass@10.0.0.1:5672"
    attributes:
      exchangeName: "myExchangeName"
      queueName: "myQueueNameName"
```
