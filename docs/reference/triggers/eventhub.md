# eventhub: Azure Event Hubs Trigger

Reads events from [Microsoft Azure Event Hubs](https://azure.microsoft.com/services/event-hubs/).

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| sharedAccessKeyName | string | Required by Azure Event Hubs |
| sharedAccessKeyValue | string | Required by Azure Event Hubs |
| namespace | string | Required by Azure Event Hubs |
| eventHubName | string | Required by Azure Event Hubs |
| consumerGroup | string | Required by Azure Event Hubs |
| partitions | list of int | List of partitions on which this function receives events |

### Example

```yaml
triggers:
  eventhub:
    kind: eventhub
    attributes:
      sharedAccessKeyName: < your value here >
      sharedAccessKeyValue: < your value here >
      namespace: < your value here >
      eventHubName: fleet
      consumerGroup: < your value here >
      partitions:
      - 0
      - 1
```

