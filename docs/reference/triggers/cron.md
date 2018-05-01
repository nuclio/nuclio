# cron: Cron Trigger

Triggers the function according to a schedule or interval, with an optional body.

## Attributes

| Path | Type | Description |
| :--- | :--- | :--- |
| schedule | string | A cron-like schedule (e.g., `*/5 * * * *`) |
| interval | string | An interval (e.g., `1s`, `30m`) |
| event.body | string | The body passed in the event |
| event.headers | map of string/int | The headers passed in the event |

> Note:
>
> 1. Either `schedule` or `interval` must be passed.
> 2. The `event.*` attributes are optional.

### Example

```yaml
triggers:
  myCronTrigger:
    kind: cron
    attributes:
      interval: 3s
```

