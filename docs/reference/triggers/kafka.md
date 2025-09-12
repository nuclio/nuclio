# Kafka trigger

## In this document

- [Overview](#overview)
  - [Workers and Worker Allocation modes](#workers-and-worker-allocation-modes)
  - [Multiple topics](#multiple-topics)
- [Configuration parameters](#configuration-parameters)
  - [Passing configuration via secrets](#passing-configuration-via-secrets)
- [How a message travels through Nuclio to the handler](#how-a-message-travels-through-nuclio-to-the-handler)
  - [Message-course configuration parameters](#message-course-configuration-parameters)
- [Offset management](#offset-management)
  - [Explicit offset commits](#explicit-offset-commits)
- [Rebalancing](#rebalancing)
  - [Rebalancing configuration parameters](#rebalancing-configuration-parameters)
  - [Choosing the right configuration for rebalancing](#choosing-the-right-configuration-for-rebalancing)
  - [Rebalancing notes](#rebalancing-notes)
- [Configuration example](#configuration-example)

## Overview

The Nuclio Kafka trigger allows users to process messages sent to Kafka. To simplify, you send messages to a Kafka stream (across topics and partitions), tell Nuclio to read from this stream, and your function handler is then called once for every stream message.

In the real world, however, you may want to scale your message processing up and down based on how much the processing occupies your Nuclio function. To support dynamic scaling, several instances (**"replicas"**) of your function must work together to split the stream messages among themselves as fairly as possible without losing any messages and without processing the same message more than once (to the best of their ability).

To this end, Nuclio leverages Kafka consumer groups. When one or more Nuclio replica joins a consumer group, Kafka informs Nuclio which part of the stream it should handle. It does so by using a process known as "rebalancing" to assign each Nuclio replica one or more Kafka partitions to read from and handle; Nuclio's role in the rebalancing process is discussed later in this document (see [Rebalancing](#rebalancing)).

```{figure} ../../assets/images/kafka-high-level.png
:align: center
:width: 400px
Nuclio and Kafka consumer groups illustration
```

When a Nuclio replica is assigned its set of partitions, it can start using Nuclio workers to read from the partitions and handle them. It's currently guaranteed that a given partition is handled only by one replica and that the messages are processed sequentially; that is, a message will only be read and handled after the handling of the previous message in the partition is completed. During rebalancing, however, the responsibility for a partition may be migrated to another Nuclio replica while still preserving the guarantee of sequential processing (in-order-execution).

- [Workers and Worker Allocation modes](#workers-and-worker-allocation-modes)
- [Multiple topics](#multiple-topics)

### Workers and Worker Allocation modes

When a partition is assigned to a replica, the partition messages are handled sequentially by one or more workers; each message is handled by a single worker. You can configure how many workers a single replica contains and how to allocate the workers to process partition messages.

- [How many workers to allocate for your replica?](#how-many-workers-to-allocate-for-your-replica)
- [How are workers allocated to a partition?](#how-are-workers-allocated-to-a-partition)

#### How many workers to allocate for your replica?

Currently, the number of workers for a given replica is statically determined by the user. Fewer workers mean less memory consumption by the replica but a longer wait time before a worker becomes available to process a new message.
A good rule of thumb is to set the number of workers to `ceil((<number of partitions> / <max number of replicas>) * 1.2)`.

For example, if you have 16 partitions and you set the maximum number of replicas to 4, then during steady state each replica handles `16 / 4 = 4` partitions. But if one of the replicas goes down, each replica handles `16 / 3 = 5 or 6` partitions. According to the recommended formula, the maximum number of workers should be `ceil((16 / 4) * 1.2) = 5`. This means that there's an extra unused worker during steady state, but the message processing won't be stalled significantly if a replica goes down.

#### How are workers allocated to a partition?

Nuclio supports two modes of worker allocation, which can be configured via the [`workerAllocationMode`](#how-are-workers-allocated-to-a-partition) configuration parameter:

- **Pool mode** (`"pool"`) - In this mode, partitions are allocated to workers dynamically on a first-come, first served basis. Whenever one of the replica's partitions receives a message, the message is allocated to the first available worker. The benefit here is that a worker is never idle while there are messages to process across the replica's partitions. The cost is that messages of a given partition may be handled by different workers (albeit always sequentially). For stateless functions, this is not a problem. However, if your function retains state, you might benefit from "pinning" specific workers to specific partitions by using the `"static"` allocation mode.
- **Static mode** (`"static"`) - In this mode, partitions are allocated statically to specific workers; each worker is assigned to handle the messages for specific partitions. For example, if the replica is handling 20 partitions and has 5 workers - partitions 0&ndash;3 are handled by worker 0, partitions 3&ndash;6 by worker 1, ..., and partitions 16&ndash;19 by worker 4. The benefit and cost of this mode are inverse to the `"pool"` mode: it's entirely possible to encounter stalled processing despite having available workers (because the available workers aren't allocated to the busy partitions), but it's guaranteed that each partition is always handled by the same worker.

### Multiple topics

Up until now, the overview discussed partitions and workers. But a Nuclio replica can also read from multiple topics. A Nuclio replica can use its workers to handle the partitions of multiple topics instead of only those of a single topic.
For example, if your replica has 10 workers and is configured to handle 10 topics, each with 100 partitions, the replica is essentially using 10 workers to handle 1,000 partitions.

## Configuration parameters
<!-- See https://kafka.apache.org/documentation/#consumerconfigs + types and
  default values in pkg/processor/trigger/kafka/types.go. -->

Use the following trigger attributes for basic configurations of your Kafka trigger.
You can configure each attribute either in the `triggers.<trigger>.attributes.<attribute>` function `spec` element (for example, `triggers.myKafkaTrigger.attributes.sessionTimeout`) or by setting the matching `nuclio.io` annotation key (for example, `nuclio.io/kafka-session-timeout`); (note that not all attributes have matching annotation keys).
For more information on Nuclio function configuration, see the [function-configuration reference](../../reference/function-configuration/function-configuration-reference.md).

> **Note:** For more advanced configuration parameters, see the configuration sections under [How a message travels through Nuclio to the handler](#message-course-configuration-parameters) and [Rebalancing](#rebalancing-configuration-parameters). For an example, see [Configuration example](#configuration-example).

- <a id="topics"></a>**`topics`** - The name of the topic(s) on which to listen.
  <br/>
  **Type:** `[]string`

- <a id="version"></a>**`version`** - The version of Kafka that Sarama will assume it is running against (by default `3.5.2`).
  Version string should be in the formats `0.11.0.3` for pre-1.0.0 versions and `1.0.0` for 1.0.0 and above. Minimal supported version is `0.11.0`.
  <br/>
  **Type:** `string`

- <a id="brokers"></a>**`brokers`** - A list of broker IP addresses.
  <br/>
  **Type:** `[]string`

- <a id="consumerGroup"></a>**`consumerGroup`** - The name of the Kafka consumer group to use.
  <br/>
  **Type:** `string`

- <a id="initialOffset"></a>**`initialOffset`** - The location (offset) within the partition at which to begin the message processing when first reading from a partition.
  Currently, you can begin the processing either with the earliest or latest ingested messages.
  <br/>
  Note that once a partition is read from and connected to a consumer group, subsequent reads are always done from the offset at which the previous read stopped, and the `initialOffset` configuration is ignored.
  <br/>
  **Type:** `string`
  <br/>
  **Valid Values:** `"earliest" | "latest"`
  <br/>
  **Default Value:** `"earliest"`

- <a id="sasl"></a>**`sasl`** - A simple authentication and security layer (SASL) object.
  <br/>
  **Type:** `object` with the following attributes -

  - **`enable`** (`bool`) - Enable authentication.
  - **`handshake`** (`bool`) - Whether to send Kafka SASL handshake first. (default to: `true`)
  - **`user`** (`string`) - Username to be used for authentication.
  - **`password`** (`string`) - Password to be used for authentication.
  - **`mechanism`** (`string`) - Name of SASL mechanism to use for authentication. (default to: `plain`, see [here](https://pkg.go.dev/github.com/Shopify/sarama#SASLMechanism) for options)
    > `GSSAPI` is yet to be supported by Nuclio. (read: Kerberos)

  - <a id="sasl.oauth"></a>**`sasl.oauth`** - SASL OAuth configuration object.
    <br/>
    **Type:** `object` with the following attributes -
    - **`clientID`** (`string`) - The client ID to use for OAuth authentication.
    - **`clientSecret`** (`string`) - The client secret to use for OAuth authentication.
    - **`tokenURL`** (`string`) - The URL of the OAuth token endpoint.
    - **`scopes`** (`[]string`) - A list of OAuth scopes to request.

- <a id="tls"></a>**`tls`** - TLS configuration object.
  <br/>
  **Type:** `object` with the following attributes -
  - **`enable`** (`bool`) - Enable TLS.
  - **`insecureSkipVerify`** (`bool`) - Allow insecure server connections when TLS enabled. (default to: `false`)
  - **`minimumVersion`** (`string`) - The default minimum TLS version that is acceptable. (default to: `1.2`)

- <a id="cacert"></a>**`caCert`** - The certificate authority (CA) certificate used for TLS authentication.
  <br/>
  **Type:** `string`
  > When filled, the certificate is used to authenticate the Kafka broker.
  TLS Authentication is enabled by default when this field is filled.

- <a id="accesskey"></a>**`accessKey`** - The private key used for TLS authentication.
  <br/>
  **Type:** `string`
  > In conjunction with the `accessCertificate` & `caCert`, the certificate is used to authenticate the Kafka broker.

- <a id="accesscertificate"></a>**`accessCertificate`** - The public key used for TLS authentication.
  <br/>
  **Type:** `string`
  > In conjunction with the `accessKey` & `caCert`, the certificate is used to authenticate the Kafka broker.

- <a id="sessionTimeout"></a>**`sessionTimeout`** (`kafka-session-timeout`) - The timeout used to detect consumer failures when using Kafka's group management facility. The consumer sends periodic heartbeats to indicate its liveness to the broker. If no heartbeats are received by the broker before the expiration of this session timeout, the broker removes this consumer from the group and initiates rebalancing. Note that the value must be in the allowable range, as configured in the `group.min.session.timeout.ms` and `group.max.session.timeout.ms` broker configuration parameters.
  <br/>
  **Type:** `string` - a string containing one or more duration strings of the format `"[0-9]+[ns|us|ms|s|m|h]"`; for example, `"300ms"` (300 milliseconds) or `"2h45m"` (2 hours and 45 minutes). See the [`ParseDuration`](https://golang.org/pkg/time/#ParseDuration) Go function.
  <br/>
  **Default Value:** `"10s"` (10 seconds)<!-- 10 * time.Second -->
  <!-- Kafka `session.timeout.ms` -->

- <a id="heartbeatInterval"></a>**`heartbeatInterval`** (**`kafka-heartbeat-interval`**) - The expected time between heartbeats to the consumer coordinator when using Kafka's group management facilities. Heartbeats are used to ensure that the consumer's session stays active and to facilitate rebalancing when new consumers join or leave the group. The value must be set lower than the [`sessionTimeout`](#configuration-parameters) configuration, but typically should be set no higher than 1/3 of that value. It can be adjusted even lower to control the expected time for normal rebalancing.
  <br/>
  **Type:** `string` - a string containing one or more duration strings of the format `"[0-9]+[ns|us|ms|s|m|h]"`; for example, `"300ms"` (300 milliseconds) or `"2h45m"` (2 hours and 45 minutes). See the [`ParseDuration`](https://golang.org/pkg/time/#ParseDuration) Go function.
  <br/>
  **Default Value:** `"3s"` (3 seconds)<!-- 3 * time.Second -->
  <!-- Kafka `heartbeat.interval.ms` -->

- <a id="workerAllocationMode"></a>**`workerAllocationMode`** (**`kafka-worker-allocation-mode`**) - The [worker allocation mode](#how-are-workers-allocated-to-a-partition).
  <br/>
  **Type:** `string`
  <br/>
  **Valid Values:** `"pool" | "static"`
  <br/>
  **Default Value:** `"pool"`

<a id="configuration-via-secret"></a>
### Passing configuration via secrets

Nuclio allows passing sensitive configuration values (such as Kafka credentials) via secrets.
To do that, follow the following steps:
1. Create a secret with the sensitive data (e.g. `access-key`)
2. Mount the secret as a volume to the function (in `spec.Volumes`)
3. Specify the path to the mounted values, either in the function's spec or in the function's annotations, with:
    1. Either specify the full path in the spec/annotation (e.g. `nuclio.io/kafka-access-key = /path/to/secret/access-key`)
    2. Or, add the secret mount path to the secretPath filed (or the nuclio.io/kafka-secret-path annotation), and the sub paths to the other annotations. Nuclio will resolve the full paths according to the existing annotations.
e.g:
```
nuclio.io/kafka-secret-path = /etc/nuclio/kafka-secret
nuclio.io/kafka-access-key = accessKey
```

The current configurations supported via secrets are: `accessKey`, `accessCertificate`, `caCert`, `SASL.OAuth.clientSecret`, `SASL.password`.

## How a message travels through Nuclio to the handler

Nuclio leverages the [Sarama](https://pkg.go.dev/github.com/Shopify/sarama) Go client library (`sarama`) to read from Kafka. This library takes care of reading messages from Kafka partitions and distributing them to a consumer - in this case, the Nuclio trigger. A Nuclio replica has exactly one instance of Sarama and one instance of the Nuclio trigger for each Kafka trigger configured for the Nuclio function.

Upon its creation, the Nuclio trigger configures Sarama to start reading messages from a given broker, topics, or consumer group. At this point, Sarama calculates which partitions the Nuclio replica must handle, communicates the results to the Nuclio trigger, and then starts dispatching messages.

```{figure} ../../assets/images/kafka-message-flow.png
:align: center
:width: 400px
Nuclio Kafka-trigger message flow
```

As the first step, Sarama reads a chunk of data from all partitions that are assigned to it, across all topics `(1)`. The amount of data to read per partition is determined in bytes and controlled by the function configuration. Ideally, each read returns data across all partitions, but this is highly dependant on the configuration and the size of messages in the partitions (see the following explanation).

When Kafka responds with a set of messages (per topic or partition), Sarama sends this information to all of its partition feeders through a queue `(2)`. The size of this queue is exactly one and is not configurable. The partition feeder (which is running in a separate "thread") reads the response and plucks and parses the relevant messages for the topic or partition that it's handling. For each parsed message, the feeder writes the processed data to the partition consumer queue `(3)`; the size of this queue is determined by the [`channelBufferSize`](#message-course-configuration-parameters) configuration . If there's no space in the queue, Sarama waits approximately for the duration of the [`maxProcessingTime`](#message-course-configuration-parameters) configuration before giving up and killing the child. This partition consumer queue allows Sarama to queue messages from Kafka so that the partition consumer never waits for reads from Kafka.

A large partition consumer queue reduces processing delays (as there are almost always messages waiting in the queue to be processed), but it costs memory and the processing time that's required to read the data from Kafka if the replica goes down.

The Nuclio trigger reads directly from this partition consumer queue (remember that there's one such message queue per partition), and for each message it allocates a worker and sends the message to be handled. When the handler returns, a new message is read from the queue and handled.

### Message-course configuration parameters
<!-- See https://pkg.go.dev/github.com/Shopify/sarama /
  vendor/github.com/Shopify/sarama/config.go, and
  https://kafka.apache.org/documentation/#consumerconfigs + types and default
  values in pkg/processor/trigger/kafka/types.go. -->

Use the following trigger attributes for message-course trigger configurations.
You can configure each attribute either in the `triggers.<trigger>.attributes.<attribute>` function `spec` element (for example, `triggers.myKafkaTrigger.attributes.fetchMin`) or by setting the matching `nuclio.io` annotation key (for example, `nuclio.io/kafka-fetch-min`).
`string` duration type is a string containing one or more duration strings of the format `"[0-9]+[ns|us|ms|s|m|h]"`; for example, `"300ms"` (300 milliseconds) or `"2h45m"` (2 hours and 45 minutes). See the [`ParseDuration`](https://golang.org/pkg/time/#ParseDuration) Go function.

| Parameter                                         | Annotation key                        | Type              | Default          | Description                                                                                                                                                                            |
|---------------------------------------------------|---------------------------------------|-------------------|------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| <a id="fetchMin"></a>`fetchMin`                   | `nuclio.io/kafka-fetch-min`           | `int`             | `1`              | Minimum number of bytes to fetch in a request. `0` causes spinning when no messages are available; higher values may improve throughput at the cost of latency.                        |
| <a id="fetchDefault"></a>`fetchDefault`           | `nuclio.io/kafka-fetch-default`       | `int`             | `1048576` (1 MB) | Default bytes to fetch per request; should exceed most message sizes to avoid frequent size renegotiation.                                                                             |
| <a id="fetchMax"></a>`fetchMax`                   | `nuclio.io/kafka-fetch-max`           | `int`             | `0` (no limit)   | Maximum bytes per fetch request. Messages larger than this yield `ErrMessageTooLarge`. Global `sarama.MaxResponseSize` still applies.                                                  |
| <a id="channelBufferSize"></a>`channelBufferSize` | `nuclio.io/kafka-channel-buffer-size` | `int`             | `256`            | Number of events buffered in internal/external channels. Accepts `1..256`, or `0` to apply the default. Improves throughput by allowing background processing.                         |
| <a id="maxProcessingTime"></a>`maxProcessingTime` | `nuclio.io/kafka-max-processing-time` | `string` duration | `"5m"`           | Max time the consumer expects a message to take to process. Actual grace is `maxProcessingTime * channelBufferSize`. Uses a ticker for timeouts (less accurate but fewer timer calls). |

## Offset management

Nuclio replicas can come up and go down on a whim (mostly due to auto-scaling), and the responsibility for a given partition migrates from one replica to the other. It's important to ensure that the new replica picks up where the previous replica left off (to avoid losing or re-processing messages). Kafka offers a persistent "offset" per partition, which indicates the consumer group's location in the partition. New Nuclio replicas can read this offset and start reading the partition from the relevant location.

However, the Nuclio replica is responsible for updating this offset. Naively, whenever a message is handled, Nuclio can contact Kafka and tell it to increment the offset of the partition. This would carry a large overhead per message and therefore be very slow.

The Sarama library offers an "auto-commit" feature wherein Nuclio replicas need only "mark" the message as handled to trigger Sarama to update Kafka periodically, in the background, about the current offsets of all partitions. The default interval is one second and cannot be configured at this time.

In addition to periodically committing offsets, Nuclio and Sarama "flush" the marked offsets to Kafka whenever a replica stops handling a partition, either because of a rebalancing process or some other condition that caused a graceful shutdown of the replica.

### Explicit offset commits

In some cases, the "auto-commit" feature can be problematic.
One example are stateful functions that might need to go and consume already being received records upon the function failure.

For that, Nuclio offers a way to accept new events without committing them, and explicitly commit offsets of the partition, when the processing is done.
This enables the function to receive and process more events simultaneously.

To enable this feature, set the `ExplicitAckMode` in the trigger's spec to `enable` or `explicitOnly`, where the optional modes are:
* `enable` - allows explicit and implicit ACK according to the "x-nuclio-stream-no-ack" header
* `disable`- disables the explicit ACK feature and allows only implicit acks (default)
* `explicitOnly`- allows only explicit acks and disables implicit acks

To receive more events without committing them, your function handler must respond with a nuclio response object, set the `x-nuclio-stream-no-ack` header to `true` in the request.
This can be done by calling the response's `ensure_no_ack()` method, like this:

```py
response = nuclio_sdk.Response()
response.ensure_no_ack()
```

To explicitly commit the offset on an event, save the relevant event information in the `QualifiedOffset` object, 
and pass it to async function `explicit_ack()` method of the context's response object, like so:
```py
qualified_offset = nuclio.QualifiedOffset.from_event(event)
await context.platform.explicit_ack(qualified_offset)
```

### Drain callback
During [rebalance](#rebalancing), the function can still be processing events. 
We can register a callback to run before the workers are drained, e.g. to drop or commit events being handled when the rebalancing is about to happen, 
using the following method (Note that the registered callback is a nullary callback (doesn't accept arguments)):
```py
context.platform.set_drain_callback(callback)
```

This feature includes a customizable timeout  `WaitExplicitAckDuringRebalanceTimeout`. Its purpose is to prevent processing the same message twice.
This timeout allows to configure the waiting time for a control message from runtime after a rebalance happened and before we unsubscribe from control messages from runtime and completely disconnect.
Default value is `100ms`. It can be also set via function annotation `nuclio.io/wait-explicit-ack-during-rebalance-timeout`.


**NOTES**:
* Currently, the Explicit Ack feature is only available for python runtime and functions that have a stream trigger (kafka/v3io).
* The explicit ack feature can be enabled only when using a static worker allocation mode. Meaning that the function metadata must have the following annotation: `"nuclio.io/kafka-worker-allocation-mode":"static"`.
* The `QualifiedOffset` object can be saved in a persistent storage and used to commit the offset on later invocation of the function.

## Rebalancing

A rebalance process (**"rebalancing"**) is triggered whenever there's a change in the number of consumer group members. This can happen in the following situations:

- The Nuclio function comes up and all Nuclio replicas are spawned. (Note that because replicas don't come up at the same time, several rebalancing processes may initially occur.)
- A new Nuclio replica joins as a result of an auto-scaling spin-up.
- A Nuclio replica goes down as a result of a failure or an auto-scaling spin-down.

When Kafka detects a change in members, it first instructs all existing members to stop their processing and "return" their partitions. When the membership stabilizes, Kafka splits the partitions across all existing members (Nuclio replicas), and each replica can then start the previously described consumption process.

This process is handled by Sarama but requires very careful logic on the Nuclio end, because Sarama is very strict with regard to time lines in this context. For example, the Nuclio partition consumer must finish handling messages well before the rebalancing timeout period ([`rebalanceTimeout`](#rebalancing-configuration-parameters)) elapses, because Sarama needs to do clean-up of its own.

However, Nuclio might be busy waiting for the user's code to finish processing an event, which can take an undetermined amount of time that's out of Nuclio's control. When the `rebalanceTimeout` period elapses, Sarama exits the membership and may return only when the messages stored in the partition consumer queue are handled. This is very problematic because when this happens, it triggers another rebalancing process (a member leaving the group), which might cause this condition on another replica.

To prevent this, Nuclio has a hard limit on how long it waits for handlers to complete processing the event ([`maxWaitHandlerDuringRebalance`](#rebalancing-configuration-parameters). If rebalancing occurs while a handler is still processing an event, Nuclio waits for a duration of `maxWaitHandlerDuringRebalance` before forcefully restarting the worker (in runtimes that support this, such as Python) or the replica (in runtimes that don't support worker restart, such as Golang).

This aggressive termination helps the consumer groups stabilize in a deterministic time frame, at the expense of re-processing the message. To reduce this occurrence, consider setting a high value for the [`rebalanceTimeout`](#rebalancing-configuration-parameters) and [`maxWaitHandlerDuringRebalance`](#rebalancing-configuration-parameters) configurations.

- [Rebalancing configuration parameters](#rebalancing-configuration-parameters)
- [Choosing the right configuration for rebalancing](#choosing-the-right-configuration-for-rebalancing)
- [Rebalancing notes](#rebalancing-notes)

### Rebalancing configuration parameters
<!-- See https://pkg.go.dev/github.com/Shopify/sarama /
  vendor/github.com/Shopify/sarama/config.go, and
  https://kafka.apache.org/documentation/#consumerconfigs + types and default
  values in pkg/processor/trigger/kafka/types.go. -->
Use the following trigger attributes for rebalancing trigger configurations.
You can configure each attribute either in the `triggers.<trigger>.attributes.<attribute>` function `spec` element (for example, `triggers.myKafkaTrigger.attributes.rebalanceTimeout`) or by setting the matching `nuclio.io` annotation key (for example, `nuclio.io/kafka-rebalance-timeout`).
`string` duration type is a string containing one or more duration strings of the format `"[0-9]+[ns|us|ms|s|m|h]"`; for example, `"300ms"` (300 milliseconds) or `"2h45m"` (2 hours and 45 minutes). See the [`ParseDuration`](https://golang.org/pkg/time/#ParseDuration) Go function.

| Parameter                       | Annotation key                                      | Type              | Default | Description                                                                                                                                                                         |
|---------------------------------|-----------------------------------------------------|-------------------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `rebalanceTimeout`              | `nuclio.io/kafka-rebalance-timeout`                 | `string` duration | `"60s"` | Maximum allowed time for each worker to join the group after rebalancing starts. Limits time to flush pending data and commit offsets; exceeding removes the worker from the group. |
| `rebalanceRetryMax`             | `nuclio.io/kafka-rebalance-retry-max`               | `int`             | `4`     | Maximum number of retry attempts if rebalancing fails due to membership changes.                                                                                                    |
| `rebalanceRetryBackoff`         | `nuclio.io/kafka-rebalance-retry-backoff`           | `string` duration | `"2s"`  | Backoff time between rebalancing retry attempts.                                                                                                                                    |
| `maxWaitHandlerDuringRebalance` | `nuclio.io/kafka-max-wait-handler-during-rebalance` | `string` duration | `"5s"`  | Time to wait for an in-flight handler to finish when rebalancing occurs before restarting the worker or replica.                                                                    |
| `adminTimeout`                  | `nuclio.io/kafka-admin-timeout`                     | `string` duration | `"15s"` | Maximum duration the administrative client waits for ClusterAdmin operations (topics, brokers, configs, ACLs).                                                                      |
| `metadataTimeout`               | `nuclio.io/kafka-metadata-timeout`                  | `string` duration | `"15s"` | How long to wait for a successful metadata response.                                                                                                                                |
| `metadataRetryBackoff`          | `nuclio.io/kafka-metadata-retry-backoff`            | `string` duration | `"2s"`  | How long to wait between metadata retries.                                                                                                                                          |
| `adminRetryBackoff`             | `nuclio.io/kafka-admin-retry-backoff`               | `string` duration | `"2s"`  | How long to wait between admin retries.                                                                                                                                             |
| `metadataRetryMax`              | `nuclio.io/kafka-metadata-retry-max`                | `int`             | `10`    | Maximum retries to get metadata.                                                                                                                                                    |
| `adminRetryMax`                 | `nuclio.io/kafka-admin-retry-max`                   | `int`             | `10`    | Maximum retries to get admin data.                                                                                                                                                  |
| `netDialTimeout`                | `nuclio.io/kafka-net-dial-timeout`                  | `string` duration | `"15s"` | How long to wait for the initial connection.                                                                                                                                        |
| `netKeepAliveInterval`          | `nuclio.io/kafka-net-keep-alive-interval`           | `string` duration | `"90s"` | Keep-alive interval for Kafka connections.                                                                                                                                          |


### Choosing the right configuration for rebalancing

In a perfect world, Nuclio would be configured out of the box to perform in the most optimal way across all use cases. In fact, if your worst-case event-processing time is short (a few seconds), then Nuclio does just that: you can leave the default configurations as-is and Nuclio should perform optimally under normal network conditions. However, if your worst-case event-processing time is in the order of tens of seconds or minutes, you must choose between the following configuration alternatives:

- [Prioritizing throughput (default)](#prioritizing-throughput-default)
- [Prioritizing minimum duplicates](#prioritizing-minimum-duplicates)

#### Prioritizing throughput (default)

During rebalancing, replicas stop processing while the new generation of the consumer group stabilizes and all members are allocated partitions. This means that while rebalancing is taking place, messages from Kafka aren't processed, which reduces the average pipeline throughput. Ideally, once a rebalancing process is initiated (for any of the reasons previously explained), all replicas immediately stop their current processing and join the rebalancing process. However, if there's a long event processing in progress, the workers processing the event can only join the rebalancing process after the current event processing completes and the user handler that received the event returns, which might take a while.

To join the rebalancing process as quickly as possible, you need to stop processing the event immediately or after a short deterministic grace periodic. Obviously, in this scenario Nuclio would not mark the event as processed because it didn't complete the processing, and therefore the event will be processed again by the replica that's assigned this partition in the new consumer-group generation.

To configure this processing logic, set [`maxWaitHandlerDuringRebalance`](#rebalancing-configuration-parameters) to a short time period, like 5 or 10 seconds. Nuclio will only wait this short amount of time before stopping the event processing and joining the rebalancing process, resulting in duplicate event processing in favor of a higher throughput.

#### Prioritizing minimum duplicates

There are many scenarios in which you might prefer to instruct Nuclio to wait for the completion of all active event processing before joining a rebalancing process - for example, when duplicate processing incurs a high cost.
This means blocking the rebalancing process and effectively halting all new event processing until the current event processing is done.

To configure this processing logic, set [`maxWaitHandlerDuringRebalance`](#rebalancing-configuration-parameters) to your worst-case event-processing time, and set [`rebalanceTimeout`](#rebalancing-configuration-parameters) to approximately 120% of `maxWaitHandlerDuringRebalance`. For example, if your worst-case event-processing time is 4 minutes, set `maxWaitHandlerDuringRebalance` to 4 minutes and `rebalanceTimeout` to 5 minutes. Increasing the rebalancing timeout guarantees that the replica or replicas that are waiting for 4 minutes (or less) for the event processing to complete are guaranteed not to be removed from the consumer group for 5 minutes, thus avoiding another rebalancing process that would be triggered if the member replica left the group.

### Rebalancing notes

<a id="message-pre-fetching"></a>Note that Nuclio's Kafka client, Sarama, performs pre-fetching of [`channelBufferSize`](#message-course-configuration-parameters) messages from Kafka into the partition consumer queue. It does so to reduce the number of times it needs to contact Kafka for messages, and to allow workers to (almost) always have a set of messages waiting to be processed without having to wait a round-trip time for Kafka to fetch the messages. During rebalancing, regardless of whether you prefer a higher throughput or minimum duplicates, the messages in this queue are discarded and have no effect on the rebalancing time. (I.e., it doesn't matter if you have one message in the queue or 256; all messages are discarded and re-fetched by the replica that handles this partition in the new consumer-group generation.)

## Configuration example

```yaml
triggers:
  myKafkaTrigger:
    kind: kafka-cluster
    attributes:
      initialOffset: earliest
      topics:
        - mytopic
      brokers:
        - 10.0.0.2:9092
      consumerGroup: my-consumer-group
      sasl:
        enable: true
        user: "nuclio"
        password: "s3rv3rl3ss"
        handshake: true

        # [optional] specify mechanism
        mechanism: SCRAM-SHA-256

      # [optional] set tls if broker requires a secured communication
      tls:
        enable: true
        insecureSkipVerify: true
        minVersion: "1.2"
```

### Troubleshooting

* Timeout during rebalance
Issue: `Panic caught while trying to write into channel, which was closed because of wait for rebalance timeout` (log example)
Solution: This issue can be resolved by increasing the `trigger.<name>.workerTerminationTimeout`. For more details, refer to the [function configuration documentation](../function-configuration/function-configuration-reference).

