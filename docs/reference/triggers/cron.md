# cron: Cron Trigger

Triggers the function according to a schedule or interval, with an optional body.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| <a id="attr-schedule"></a>schedule | string | A cron-like schedule (for example, `*/5 * * * *`). |
| <a id="attr-interval"></a>interval | string | An interval (for example, `1s`, `30m`). |
| concurrencyPolicy | string | Concurrency policy - `"Allow"`, `"Forbid"`, or `"Replace"`; (default: `"Forbid"`). Applicable only to Kubernetes platforms. |
| jobBackoffLimit | int32 | The number of retries before failing a job; (default: `2`). Applicable only to Kubernetes platforms. |
| event.body | string | The body passed in the event. |
| event.headers | map of string/int | The headers passed in the event. |

<a id="attr-notes"></a>
> **Note:**
> 1. <a id="schedule-or-interval-attr-set-note"></a>You must set either the [`schedule`](#attr-schedule) or [`interval`](#attr-interval) mutually-exclusive attributes.
> 2. <a id="event-attrs-note"></a>The `event.*` attributes are optional.
> 3. <a id="k8s-notes"></a>On Kubernetes platforms, the Cron trigger is implemented as a Kubernetes CronJob (instead of the standard implementation, which runs the trigger within the processor).
>    Note:
>    - On Kubernetes, you must set the `cronTriggerCreationMode` platform-configuration field to `"kube"`, to implement Cron triggers as CronJobs.
>        For more information, see [Configuring a Platform](/docs/tasks/configuring-a-platform.md#cronTriggerCreationMode).
>    - The created CronJob uses `wget` to call the default HTTP trigger of the function according to the configured interval or schedule.
>        (This means that worker-related attributes are irrelevant.)
>    - The `wget` request is sent with the header `"x-nuclio-invoke-trigger: cron"`.

### Examples

```yaml
triggers:
  myCronTrigger:
    kind: cron
    attributes:
      interval: 3s
```

The following example is specific to Kubernetes platforms, as it sets the Kubernetes-specific `concurrencyPolicy` and `jobBackoffLimit` attributes.
Remember that when running on Kubernetes, you also need to set the `cronTriggerCreationMode` platform-configuration field to `"kube"` (see the [Kubernetes notes](#k8s-notes)).
```yaml
triggers:
  myCronTrigger:
    kind: cron
    attributes:
      interval: 10s
      concurrencyPolicy: "Allow"
      jobBackoffLimit: 2
```
