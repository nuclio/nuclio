# v3io-stream: v3io Stream Trigger

**In This Document**
- [Overview](#overview)
- [How Nuclio consumes messages through a consumer group](#consume-messages)
  - [Example](#example)
- [Configuring through the UI](#config-via-ui)

<a id="overview"></a>
## Overview

The Nuclio v3io stream trigger allows users to process messages sent to a v3io stream. To simplify, you send messages to a v3io stream, tell Nuclio to read from this stream, and your function handler is then called once for every stream message.

In the real world, however, you may want to have multiple replicas of your function reading from the same stream to spread message processing across them. These function replicas must work together to split the stream messages among themselves as fairly as possible without losing any messages and without processing the same message more than once (to the best of their ability).

To this end, Nuclio leverages consumer groups built into the v3io-go library. When one or more Nuclio replicas joins a consumer group it will receive its equal share of the shards, based on how many replicas are defined in the function (this is discussed in more detail later). 

When a Nuclio replica is assigned its set of shards, it can start using Nuclio workers to read from the shards and handle them. It's currently guaranteed that a given shard is handled only by one replica and that the messages are processed sequentially; that is, a message will only be read and handled after the handling of the previous message in the shard is completed. 

<a id="consume-messages"></a>
## How Nuclio consumes messages through a consumer group
Once a function replica with a v3io stream trigger starts up, it reads a stream state object stored alongside the stream shards. This object stores, per consumer group, the following information:
- Which **members** (Nuclio function replicas in this case) are currently active in the consumer group
- When was the last time the member refreshed its keep alive field
- Which shards are being handled by each member

The replica will check for stale members (members who haven't refreshed their keep alive field in the timeframe given by the session timeout) and remove them. It will then check if there are any shards not handled by any member, take a fair portion of these shards and write itself into the consumer group state object. 

> Note: The consumer group state object is a point of contention - multiple replicas may want to modify it concurrenctly. To protect against this, each replica performs read-modify-write with mtime protection enforced by v3io (meaning it will write the object _only_ if the mtime since read has not changed). It will retry to do read-modify-write with a random exponential backoff until successful

Upon receiving its shard allocation (which shards the replica must handle), the replica will spawn a go routine ("thread") for each shard. Each go routine will check which offset the shard is at for this consumer group (stored as an attribute on the shard) and start pulling messages from that offset (if the offset doesn't exist, like when this is the first read for the shard/consume group, a seek to earliest or latest will be performed according to the configuration). For each message read, the function's handler is called.

For every message read, Nuclio will "mark" the sequence number as handled. Periodically, the latest marked sequence numbers per shards are "committed" - written to the attribute on the shard. This allows future replicas to pick up where the previous replica left off without affecting performance.

<a id="example"></a>
### Example
Let's illustrate the above with an example.

A function with min/max replicas set to `3` is deployed and configured to read from stream `/my-stream` (which has 12 shards) through consumer group `my-consumer-group`. The first replica comes up and reads the stream state object, but finds it contains no information about the consumer group. It therefore creates the state object, registering itself as a member taking up a third of the shards (`12 / 3 = 4`):

```
[
  {
    member_id: replica1
    shards: 0-3
    last_heartbeat: t0
  }
]
```

The first replica then spawns 4 go routines to read from the 4 shards. Each go routine reads the offset attribute stored at the shard only to find that it doesn't exist (since this is the first time the shard is read through the consumer group `my-consumer-group`). It therefore seeks to earliest/latest (as per configuration) and starts reading batches of messages - sending each message to the function handler as an event. Periodically, the replica will:

- Commit the offsets back to the shard offset attributes
- Update the "last_heartbeat" field to "now()" - indicating that it's alive (it will also look for and remove stale members)

The second and third replicas come up and register themselves in a similar manner (and also create go routines, read messages, commit offsets and update the last_heartbeat field):
```
[
  {
    member_id: replica1
    shards: 0-3
    last_heartbeat: t0
  },
  {
    member_id: replica2
    shards: 4-7
    last_heartbeat: t1
  },
  {
    member_id: replica3
    shards: 8-11
    last_heartbeat: t2
  },
]
```

#### Function is redeployed
At a certain point in time, the user decides to redeploy the function. Since by default Nuclio uses the rolling update deployment strategy, Kubernetes will terminate the replicas one by one. The replica1 pod stops, a new replica1 pod is brought up and follows the same startup procedure described above. It will read the state object and look for free shards to take over. Initially, it will find none - since the last_heartbeat field of replica1 is still within session timeout and replica2 and replica3 keep updating their last_heartbeat field. 

At this point replica1 will back off and retry periodically. During one of the retries, replica1 will detect that the time passed since replica1's last_heartbeat exceeds session timeout. It will then remove replica1's record and then detect that there are free shards and take over them.

> Note: both replica2 and replica3 might be the ones that delete replica1's state. They clean up stale records when they update their last_heartbeat

The new instance of replica1 will then read the last offset attribute on shards 0-3, seek the stream to that point and continue reading messages from where the previous instance of replica1 left off. This process repeats for replica2 and replica3.

<a id="config-via-ui"></a>
## Configuring through the UI

As of 1.1.33 / 1.3.20 the configuration parameters the user can configure are:
- URL: In the form of `http://v3io-webapi:8081/<container name>/<stream path>@<consumer group name>`. For example: ` http://v3io-webapi:8081/bigdata/my-stream/@my-consumer-group`
- Max Workers: How many workers should handle the messages across the incoming shards. Whenever a worker is available and a message reads a shard - it will be handled in that worker 
- Worker Availability Timeout: Ignored
- Partitions: Ignored (as described above, this is automatic)
- SeekTo: Indicates from where to read the shard when an offset has not yet been committed. Once an offset has been committed it for a consumer group it will always be used and this parameter is ignored
- Read Batch Size: How many messages are read per request to v3io (unchanged from previous versions)
- Polling Interval (ms): How many milliseconds to wait between reading messages from the v3io stream (unchanged from previous versions)
- Username: Ignored
- Password: The access key with which to access the data (in previous versions this is the password)
- Worker allocator name: Ignored

> Note: In future versions of Nuclio, the UI will better reflect the configuration paramters and add more knobs to configure (e.g. session timeout and heartbeat interval, which are 10s and 3s respectively)
