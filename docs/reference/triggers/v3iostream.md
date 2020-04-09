# v3ioStream: Iguazio Data Science Platform Stream Trigger

**In This Document**
- [Overview](#overview)
- [Consuming messages through a consumer group](#consume-messages)
  - [Example](#example)
- [Dashboard configuration](#ui-config)

<a id="overview"></a>
## Overview

The Nuclio `v3ioStream` trigger allows users to process messages that are sent to an Iguazio Data Science Platform (**"platform"**) data stream (a.k.a. **"v3io stream"**). To simplify, you send messages to a platform stream, instruct Nuclio to read from this stream, and then your function handler is called once for every stream message.

In the real world, however, you might want to divide the message-processing load across the replicas by using multiple function replicas to read from the same stream. These function replicas must work together to split the stream messages among themselves as fairly as possible, without losing any messages and without processing the same message more than once (to the best of their ability).

To this end, Nuclio leverages consumer groups that are built into the platform's Go library (`v3io-go`). When one or more Nuclio replicas join a consumer group, each replica receives its equal share of the shards, based on the number of replicas that are defined in the function (see details later in this document). 

When a Nuclio replica is assigned a set of shards, the replica can start using Nuclio workers to read from the shards and handle the records consumption. It's currently guaranteed that a given shard is handled only by one replica, and that the messages are processed sequentially; that is, a message is read and handled only after the handling of the previous message in the shard is completed. 

<a id="consume-messages"></a>
## Consuming messages through a consumer group

When a function replica with a `v3ioStream` trigger starts up, it reads a stream state object that's stored alongside the stream shards. This object has an attribute for each consumer group that contains the following information:

- The **members** (in this case, Nuclio function replicas) that are currently active in the consumer group.
- The last time that each member refreshed its keep-alive field (`last_heartbeat`).
- The shards that are being handled by each member.

The replica checks for stale members - replicas who haven't refreshed their `last_heartbeat` field within the allotted time frame, as set in the session timeout - and removes their entries from the state object's consumer-group attribute. Then, the replica checks for shards that aren't handled by any member, and registers itself with the consumer group as the owner of a fair portion of these shards by adding an entry to the consumer-group attribute.

> **Note:** It's possible that multiple replicas might simultaneously want to modify the same consumer-group attribute in the stream's state object. To protect against this, each replica performs read-modify-write with `mtime` protection (which is enforced by the `v3io-go` library); meaning, the consumer-group attribute is written only if the last-modification time (`mtime`) value hasn't changed since the read. If this condition isn't met and the attribute isn't written, the replica retries the read-modify-write process with a random exponential backoff until the write succeeds.

Upon receiving its shard assignment, the replica spawns a Go routine ("thread") for each shard. Each Go routine identifies the current shard offset within the replica's consumer group (which is stored as an attribute in the shard), and starts pulling messages from this offset. When there's no offset information (for example, for the first read from a consumer-group shard), the replica performs a seek for the earliest or latest shard record, according to the function's seek configuration. The function handler is called for each read message.

For each read message, Nuclio "marks" the sequence number as handled. Periodically, the latest marked sequence number for each shard is "committed" (written to the shard's offset attribute). This allows future replicas to pick up where the previous replica left off without affecting performance.

<a id="example"></a>
### Example

To illustrate the consumption mechanism, assume a deployed Nuclio function with minimum and maximum replicas configurations of`3`; the function is configured to read from a `/my-stream` stream with 12 shards using the consumer group `my-consumer-group`.

The first replica comes up and reads the stream-state object, but finds that it doesn't contain information about the consumer group. It therefore adds a new consumer-group attribute to the state object, registers itself as a member of the consumer group (by adding a relevant entry to the consumer-group attribute), and monopolizes a third of the shards (`12 / 3 = 4`).

The first replica spawns four Go routines to read from the four shards. Each Go routine reads the offset attribute that's stored in the shard, only to find that it doesn't exist - because this is the first time that the shard is read through consumer group `my-consumer-group`. It therefore seeks the earliest or latest shard record (depending on the function configuration) and starts reading batches of messages, sending each message to the function handler as an event. The replica then periodically does the following:

- Commits the offsets back to the shards' offset attribute.
- Updates the `last_heartbeat` field to `now()` to indicate that it's alive.
- Identifies and removes stale members.

The second and third replicas come up and register themselves in a similar manner and perform similar steps.

The following demonstrates the replica configurations for this example:
```json
[
  {
    "member_id": "replica1",
    "shards": "0-3",
    "last_heartbeat": "t0"
  },
  {
    "member_id": "replica2",
    "shards": "4-7",
    "last_heartbeat": "t1"
  },
  {
    "member_id": "replica3",
    "shards": "8-11",
    "last_heartbeat": "t2"
  }
]
```

<a id="example-function-redeployment"></a>
#### Function redeployment

At some point, the user decides to redeploy the function. Because by default, Nuclio uses the rolling-update deployment strategy, Kubernetes terminates the replicas one by one. The `replica1` pod stops, and a new `replica1` pod is brought up and follows the same startup procedure: it reads the state object's consumer-group attribute and looks for free shards to take over; initially, it won't find any, because the `last_heartbeat` field of `replica1` is still within the session timeout period and `replica2` and `replica3` keep updating their `last_heartbeat` field. 

At this stage, `replica1` backs off and retries periodically until it eventually detects that the elapsed time since `replica1`'s `last_heartbeat` value exceeds the session's timeout period. `replica1` then removes the previous `replica1` instance from the consumer group (by removing its entry from the group's attribute in the stream's state object). It then detects that there are free shards and adds a `replica1` entry to the state object's consumer-group attribute to register itself as a member and take over a fair portion of the free shards.

> **Note:** It's also possible for `replica1` to be removed from the consumer group by `replica2` or `replica3`, because each replica cleans up all stale group members when updating its `last_heartbeat` field. 

For shards 0-3, the new instance of `replica1` then reads the shard's offset attribute, which indicates the location in the shard at which the previous instance of `replica1` left off; seeks the read offset in the shard; and continues reading messages from this location. The same process is executed for `replica2` and `replica3`.

<a id="ui-config"></a>
## Dashboard configuration

As of Nuclio v1.1.33 / v1.3.20, you can configure the following configuration parameters from the Nuclio dashboard:

- **URL**: A consumer-group URL of the form `http://v3io-webapi:8081/<container name>/<stream path>@<consumer group name>`; for example, ` http://v3io-webapi:8081/bigdata/my-stream@my-consumer-group`.
- **Max Workers**: The maximum number of workers to allocate for handling the messages of incoming stream shards. Whenever a worker is available and a message reads a shard, the processing is handled by the available worker. 
- **Worker Availability Timeout**: DEPRECATED (ignored)
- **Partitions**: DEPRECATED (ignored). As explained in the previous sections, in the current release, the assignment of shards ("partitions") to replicas is handled automatically.
- **Seek To**: The location (offset) within the message from which to consume records when there's no committed offset in the shard's offset attribute. After an offset is committed for a shard in the consumer group, this offset is always used and the **Seek To** parameter is ignored for this shard.
- **Read Batch Size**: Read batch size - the number of messages to read in each read request that's submitted to the platform.
- **Polling Interval (ms)**: The time, in milliseconds, to wait between reading messages from the platform stream.
- **Username**: DEPRECATED (ignored)
- **Password**: A platform access key for accessing the data.
- **Worker allocator name**: DEPRECATED (ignored)

> **Note:** In future versions of Nuclio, it's planned that the dashboard will better reflect the role of the configuration parameters and add more parameters (such as session timeout and heartbeat interval, which are currently always set to the default values of 10s and 3s, respectively, unless you edit the function-configuration file).

