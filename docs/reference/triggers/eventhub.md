# Eventhub Trigger

Reads events from Azure eventhub.

## Attributes

| Path | Type | Description | 
| --- | --- | --- |  
| sharedAccessKeyName | string | Required by Azure event hub |
| sharedAccessKeyValue | string | Required by Azure event hub |
| namespace | string | Required by Azure event hub |
| eventHubName | string | Required by Azure event hub |
| consumerGroup | string | Required by Azure event hub |
| partitions | list of int | List of partitions on which this function receives events |

#### Example

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
