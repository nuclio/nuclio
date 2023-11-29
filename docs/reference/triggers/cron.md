# Cron trigger

## In this document
- [Overview](#overview)
- [Attributes](#attributes)
- [Examples](#examples)

<a id="overview"></a>
## Overview

Triggers the function according to a schedule or interval, with an optional body.

<a id="attributes"></a>
## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| <a id="attr-schedule"></a>schedule | string | A cron-like schedule (for example, `*/5 * * * *`). |
| <a id="attr-interval"></a>interval | string | An interval (for example, `1s`, `30m`). |
| <a id="attr-concurrencyPolicy"></a>concurrencyPolicy | string | Concurrency policy - `"Allow"`, `"Forbid"`, or `"Replace"`; (default: `"Forbid"`). Applicable only when using CronJobs on Kubernetes platforms (see the [Kubernetes notes](#k8s-notes)). |
| <a id="attr-jobBackoffLimit"></a>jobBackoffLimit | int32 | The number of retries before failing a job; (default: `2`). Applicable only when using CronJobs on Kubernetes platforms (see the [Kubernetes notes](#k8s-notes)). |
| event.body | string | The body passed in the event. |
| event.headers | map of string/int | The headers passed in the event. |

<a id="attr-notes"></a>
> **Note:**
> 1. <a id="schedule-or-interval-attr-set-note"></a>You must set either the [`schedule`](#attr-schedule) or [`interval`](#attr-interval) mutually-exclusive attributes.
> 2. <a id="event-attrs-note"></a>The `event.*` attributes are optional.
> 3. <a id="k8s-notes"></a>**[Tech Preview]** On Kubernetes platforms, you can set the `cronTriggerCreationMode` platform-configuration field to `"kube"` to run the triggers as Kubernetes CronJobs instead of the default implementation of running Cron triggers from the Nuclio processor.
>        For more information, see [Configuring a Platform](../../tasks/configuring-a-platform.md#cronTriggerCreationMode).
>        When running Cron triggers as CronJobs &mdash;
>    - The created CronJob uses `wget` to call the default HTTP trigger of the function according to the configured interval or schedule.
>        (This means that worker-related attributes are irrelevant.)
>    - The `wget` request is sent with the header `"x-nuclio-invoke-trigger: cron"`.
>    - You can use the [`concurrencyPolicy`](#attr-concurrencyPolicy) and [`jobBackoffLimit`](#attr-jobBackoffLimit) attributes to configure the CronJobs.

<a id="examples"></a>
### Examples

```yaml
triggers:
  myCronTrigger:
    kind: cron
    attributes:
      interval: 3s
```

The following example is demonstrates a configuration for running Cron triggers as Kubernetes CronJobs, as it sets the `concurrencyPolicy` and `jobBackoffLimit` attributes.
Remember that this implementation requires setting the `cronTriggerCreationMode` platform-configuration field to `"kube"`.
See the [Kubernetes notes](#k8s-notes).
```yaml
triggers:
  myCronTrigger:
    kind: cron
    attributes:
      interval: 10s
      concurrencyPolicy: "Allow"
      jobBackoffLimit: 2
```

