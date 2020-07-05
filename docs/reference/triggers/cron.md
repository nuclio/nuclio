# cron: Cron Trigger

Triggers the function according to a schedule or interval, with an optional body.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| schedule | string | A cron-like schedule (for example, `*/5 * * * *`) |
| interval | string | An interval (for example, `1s`, `30m`) |
| concurrencyPolicy | string | Concurrency policy [Allow, Forbid, Replace]. (optional, defaults to "Allow". Relevant only for k8s platform)
| jobBackoffLimit | int32 | The number of retries before failing a job. (optional, defaults to 2. Relevant only for k8s platform)
| event.body | string | The body passed in the event |
| event.headers | map of string/int | The headers passed in the event |

> **Note:**
> 1. Either `schedule` or `interval` must be passed.
> 2. The `event.*` attributes are optional.
> 3. When running on k8s platform, this trigger will be implemented as k8s CronJob. (instead of running inside the processor like a regular trigger)
>    1. The created CronJob uses "wget" to call the default http trigger of the function every interval/schedule. (That means that worker related attributes are irrelevant)
>    2. The "wget" request will be sent with the header "x-nuclio-invoke-trigger"="cron".

### Example


```yaml
triggers:
  myCronTrigger:
    kind: cron
    attributes:
      interval: 3s
```

On K8s platform:
```yaml
triggers:
  myCronTrigger:
    kind: cron
    attributes:
      interval: 10s
      concurrencyPolicy: "Allow"
      jobBackoffLimit: 2
```
