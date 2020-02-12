# Nuclio and Kafka

**In This Document**
- [Overview](#overview)
- [How a message travels through Nuclio until it reaches the handler](#how-a-message-travels-through-Nuclio-until-it-reaches-the-handler)
- [Offset management](#offset-management)
- [Rebalancing](#rebalancing)

## Overview
The Nuclio Kafka trigger allows users to process messages sent to Kafka. To simplify, you send messages to a Kafka stream (across topics and partitions), tell Nuclio to read from this stream and your function handler will be called once for every stream message. 

In the real world, however, you may want to scale your processing up and down based on how busy your Nuclio function is processing these messages. Doing so would mean that several instances (or _replicas_) of your function must work together to split the messages in the stream between them as fairly as they can without losing any messages and, to the best of their ability, without processing the same message more than once.

To this end, Nuclio leverages Kafka Consumer Groups. When one or more Nuclio replicas joins a Consumer Group, Kafka will communicate with Nuclio to tell it which part of the stream it should handle. It does so by assigning each Nuclio replica one or more Kafka partitions to read from and handle in a process called rebalancing (more on how Nuclio handles this later).

<p align="center"><img src="/docs/assets/images/kafka-high-level.png" width="400"/></p>

When a Nuclio replica is assigned its set of partitions it can then start reading from them and handling them in Nuclio workers. It is currently guaranteed that the same partition will only be handled by one replica, sequentially (that is, a message will only be read and handled once the previous message of that partition has been handled). During rebalancing, however, the responsibility for a partition may be migrated to another Nuclio replica - but the guarantee of sequential processing (in-order-execution) is still maintained. 

### Workers and Worker Allocation modes
When a partition is assigned to a replica, its messages will be handled one by one by a worker. A user can configure how many workers a single replica contains and how these are allocated when a message arrives.

#### How many workers should my replica have?
Currently the number of workers is statically determined by the user. The fewer workers there are, the less memory a replica consumes, but the more time a partition will have to wait until a worker becomes available to handle its messages. A good rule of thumb is to set this to `ceil((number of partitions / max number of replicas) * 1.2)`.

For example, if you have 16 partitions and you set your max number of replicas to 4 then during steady state each replica will handle `16 / 4 = 4` partitions. However, if one of the replicas goes down, each replica will handle `16 / 3 = 5 or 6` partitions. Following the formula above we set max workers to `ceil((16 / 4) * 1.2) = 5`. We will spend an extra worker during steady state but allow for a single replica to go down without adding too much stalling during processing.

#### How are workers allocated to a partition?
Nuclio supports two worker allocation modes:

* Pool mode: In this mode, partitions are allocated a worker based on "first come first served". Whenever a worker is available and a partition handled by this replica has a message - it will receive the worker. The benefit here is that a worker will never be idle if there are messages to process across the replica partitions. The cost is that messages of a given partition may be handled by different workers (albeit *always* sequentially). For stateless functions, this is not a problem. However, if your function retains state - you may benefit from "pinning" specific workers to specific partitions using the `static` mode
* Static mode: When `static` mode is configured, the number of workers are statically assigned to the partitions the replica is currently handling. For example, if the replica is handling 20 partitions and has 5 workers - partitions `0..3` will be handled by worker 0, `3..6` by worker 1, ..., `16..19` by worker 4. The benefits here are inverse to `pool` mode - there very well may be a scenario where workers are available but processing is stalled (because these workers aren't mapped to handle the busy partitions), but it is guaranteed that a given partition is *always* handled by the same worker

### Multiple topics
The overview above discussed partitions and workers, but a Nuclio replica supports reading from multiple topics as well. Rather than handling partitions of a single topic with a given set of workers, Nuclio will handle the partitions of multiple topics in that same given set of workers. If you have 10 topics with 100 partitions each and 10 workers, your replica will essentially be handling 1000 partitions with 10 workers.

### Configuration parameters
* Session Timeout (`trigger.attributes.sessionTimeout`, `nuclio.io/kafka-session-timeout`): The timeout used to detect consumer failures when using Kafka's group management facility. The consumer sends periodic heartbeats to indicate its liveness to the broker. If no heartbeats are received by the broker before the expiration of this session timeout, then the broker will remove this consumer from the group and initiate a rebalance. Note that the value must be in the allowable range as configured in the broker configuration by `group.min.session.timeout.ms` and `group.max.session.timeout.ms` (default 10s)
* Heartbeat Interval (`trigger.attributes.heartbeatInterval`, `nuclio.io/kafka-hearbeat-interval`): The expected time between heartbeats to the consumer coordinator when using Kafka's group management facilities. Heartbeats are used to ensure that the consumer's session stays active and to facilitate rebalancing when new consumers join or leave the group. The value must be set lower than Consumer.Group.Session.Timeout, but typically should be set no higher than 1/3 of that value. It can be adjusted even lower to control the expected time for normal rebalances (default 3s)
* Worker Allocation Mode (`trigger.attributes.workerAllocationMode`, `nuclio.io/kafka-worker-allocation-mode`): One of `pool` or `static` as described above. Defaults to `pool`

## How a message travels through Nuclio until it reaches the handler
Nuclio leverages the Sarama Kafka Client Library (`sarama`) to read from Kafka. This library takes care of reading messages from Kafka partitions and distributing them to a consumer - in this case the Nuclio trigger. A Nuclio replica has exactly one instance of `sarama` and one instance of Nuclio trigger for each Kafka trigger configured for the Nuclio function.

When created, the Nuclio trigger configures `sarama` to start reading messages from a given broker/topics/consumer group. At this point `sarama` will do its thing to understand which partitions the Nuclio replica must take care of and communicate that back to the Nuclio trigger. `sarama` will then start dispatching messages.

<p align="center"><img src="/docs/assets/images/kafka-message-flow.png" width="400"/></p>

The first step `sarama` performs is to read a chunk of data from all partitions it is assigned to, across all topics (1). The amount read per partition is determined in bytes and controlled by the configuration. Ideally, each read will return data across all partitions but this is highly dependant on configuration and the size of messages in the partitions (see below). 

When Kafka responds with a set of messages (per topic/partition), `sarama` sends this information to all of its partition feeders through a queue (2). The size of this queue is exactly 1 and is not configurable. The partition feeder (running in a separate "thread"), reads the response and plucks/parses the relevant messages for the topic/partition it is handling. For each message parsed, it writes this to the partition consumer queue (3) whose size is determined by `channelBufferSize`. If there is no space in the queue it will wait approximately `maxProcessingTime` before giving up and killing the child. This partition consumer queue allows `sarama` to queue messages from Kafka so that the partition consumer never waits for reads from Kafka.

A large partition consumer queue will reduce processing delays (as there will almost always be messages waiting in the queue to be processed) but costs memory and the processing time reading this from Kafka if the replica goes down.

The Nuclio trigger reads directly from this partition consumer queue (remember - there is one such message queue per partition) and for each message allocates a worker and sends the message to be handled. When the handler returns, a new message is read from the queue and handled.

### Configuration parameters
* Fetch Min (`trigger.attributes.fetchMin`, `nuclio.io/kafka-fetch-min`): The minimum number of message bytes to fetch in a request - the broker will wait until at least this many are available. The default is 1, as 0 causes the consumer to spin when no messages are available. Equivalent to the JVM's `fetch.min.bytes`.
* Fetch Default (`trigger.attributes.fetchDefault`, `nuclio.io/kafka-fetch-default`): The default number of message bytes to fetch from the broker in each request (default 1MB). This should be larger than the majority of your messages, or else the consumer will spend a lot of time negotiating sizes and not actually consuming. Similar to the JVM's `fetch.message.max.bytes`.
* Fetch Max (`trigger.attributes.fetchMax`, `nuclio.io/kafka-fetch-max`): The maximum number of message bytes to fetch from the broker in a single request. Messages larger than this will return ErrMessageTooLarge and will not be consumable, so you must be sure this is at least as large as your largest message. Defaults to 0 (no limit). Similar to the JVM's `fetch.message.max.bytes`. The global `sarama.MaxResponseSize` still applies.
* Channel Buffer Size (`trigger.attributes.channelBufferSize`, `nuclio.io/kafka-channel-buffer-size`): The number of events to buffer in internal and external channels. This permits the producer and consumer to continue processing some messages in the background while user code is working, greatly improving throughput. Defaults to 256.
* Max Processing Time (`trigger.attributes.maxProcessingTime`, `nuclio.io/kafka-max-processing-time`): The maximum amount of time the consumer expects a message takes to process for the user. If writing to the Messages channel takes longer than this, that partition will stop fetching more messages until it can proceed again. Note that, since the Messages channel is buffered, the actual grace time is (MaxProcessingTime * ChannelBufferSize). Defaults to 100ms. If a message is not written to the Messages channel between two ticks of the expiryTicker then a timeout is detected. Using a ticker instead of a timer to detect timeouts should typically result in many fewer calls to Timer functions which may result in a significant performance improvement if many messages are being sent and timeouts are infrequent. The disadvantage of using a ticker instead of a timer is that timeouts will be less accurate. That is, the effective timeout could be between `MaxProcessingTime` and `2 * MaxProcessingTime`. For example, if `MaxProcessingTime` is 100ms then a delay of 180ms between two messages being sent may not be recognized as a timeout. 

## Offset management
Nuclio replicas can come up and go down on a whim (mostly due to auto-scaling) and the responsibility for a given partition migrates from one replica to the other. It is important to make sure that the new replica picks up where the previous replica left off (so as to not lose messages or re-process message). Kafka offers a persistent "offset" per partition that indicates at which point in the partition the consumer group is. New Nuclio replicas can read this and simply start reading the partition from there.

However, it is the Nuclio replica's responsibility to update this offset. Naively, whenever a message is handled Nuclio can contact Kafka and tell it to increment the offset of the partition. This would carry a large overhead per message and therefore be very slow. 

The `sarama` library offers an "auto-commit" feature wherein Nuclio replicas need only "mark" the message as handled and `sarama` will, in the background and periodically, update Kafka about the current offsets of all partitions. The default interval is 1s and cannot be configured at this time. 

In addition to periodically committing offsets, Nuclio and `sarama` will make sure to "flush" the marked offsets to Kafka whenever a replica stops handling a partition - either due to a rebalancing process or some other condition which caused a graceful shutdown of the replica.

## Rebalancing
A rebalance process is triggered whenever there is a change in the number of consumer group members. This can happen when:
* The Nuclio function comes up and all Nuclio replicas are spawned (note: since replicas don't come up at the same time, several rebalancing processes may initially occur)
* A new Nuclio replica joins due to auto-scaling spinning it up 
* A Nuclio replica goes down due to failure or auto-scaling terminating it

When Kafka detects a change in members, it will first tell all existing members to stop their processing and "return" their partitions. When the membership stabilizes, Kafka will split the partitions across all existing members (Nuclio replicas) and each replica can then start the consumption process described above.

This process is handled by `sarama` but requires very careful logic on the Nuclio end because `sarama` is very strict with regards to timelines here. For example, the Nuclio partition consumer must finish up handling messages well before the `Rebalance Timeout` expires because `sarama` has to do clean up of its own.

However, Nuclio may be busy waiting for the user's code to finish processing an event and this can take an undetermined amount of time, out of Nuclio's control. If the `Rebalance Timeout` expires, `sarama` will exit the membership and may return only when the messages stored in the partition consumer queue are handled. This is very problematic since when this happens, it triggers _another_ rebalance (member leaving the group) which may cause this condition on another replica.

To prevent this, Nuclio has a hard limit to how long it will wait for handlers to complete processing the event (`MaxWaitHandlerDuringRebalance`). If a rebalance occurs while a handler is still processing an event, Nuclio will wait `MaxWaitHandlerDuringRebalance` before forcefully restarting the worker (in runtimes that support this, e.g. Python) or the replica (in runtimes that don't support worker restart, like Golang).

This aggressive termination helps the consumer groups stabilize in a deterministic time frame, at the expense of re-processing the message. To reduce this occurrence, consider setting a high value for `RebalanceTimeout` and `MaxWaitHandlerDuringRebalance`.

### Configuration parameters
* RebalanceTimeout (`trigger.attributes.rebalanceTimeout`, `nuclio.io/kafka-rebalance-timeout`): The maximum allowed time for each worker to join the group once a rebalance has begun. This is basically a limit on the amount of time needed for all tasks to flush any pending data and commit offsets. If the timeout is exceeded, then the worker will be removed from the group, which will cause offset commit failures (default 60s).
* RebalanceRetryMax (`trigger.attributes.rebalanceRetryMax`, `nuclio.io/kafka-rebalance-retry-max`): When a new consumer joins a consumer group the set of consumers attempt to "rebalance" the load to assign partitions to each consumer. If the set of consumers changes while this assignment is taking place the rebalance will fail and retry. This setting controls the maximum number of attempts before giving up (default 4).
* RebalanceRetryBackoff (`trigger.attributes.rebalanceRetryBackoff`, `nuclio.io/kafka-rebalance-retry-backoff`): Back-off time between retries during rebalance (default 2s)
* MaxWaitHandlerDuringRebalance (`trigger.attributes.maxWaitHandlerDuringRebalance`, `nuclio.io/kafka-max-wait-handler-during-rebalance`): The time to wait for a handler to return when a rebalancing occurs before restarting the worker or replica (default: 5s). 

### Choosing the right configuration for rebalancing
In a perfect world, Nuclio would be configured out of the box to perform in the most optimal way across all use cases. In fact, if your worst case event processing time is short (a few seconds) then Nuclio does just that - you can leave the defaults as is and Nuclio should perform optimally under normal network conditions. However, if your worst case event processing time is in the order of tens of seconds or minutes, you must choose through configuration between *minimizing duplicate processing* and *higher overall throughput*.

#### I prefer higher throughput (the default)
Under rebalancing, replicas stop processing while the new generation of the consumer group stabilizes and all members are allocated their partitions. This means that while rebalancing is taking place, messages from Kafka are not processed causing the average pipeline throughput to be reduced. Ideally, once a rebalancing process is initiated due to one of the reasons explained above - all replicas immediately stop what they're doing and join the process. However, when event processing is long, the workers in the replica may be busy processing an event for a while (i.e. the user's handler received an event and has not returned yet). 

To join the rebalancing process as quickly as possible, you would have to stop processing the event immediately (or after a short, deterministic grace periodic). Obviously under this condition Nuclio would not mark this event as processed seeing how it didn't complete processing and as such it will be processed *again* by whichever replica receives this partition in the new consumer group generation. 

To configure for this scenario you would simply have to set `MaxWaitHandlerDuringRebalance` to something short like 5 or 10 seconds. Nuclio will only wait this short amount of time before stopping the event processing, joining the rebalance process and causing a duplicate.

#### I prefer to minimize duplicates
Under many scenarios, like when duplicate processing incurs a high cost, users might choose to simply instruct Nuclio to wait until all events currently being processed are complete. Doing so means blocking the rebalancing process until this happens and effectively halting event processing until it's done.

To configure for this scenario you would have to set `MaxWaitHandlerDuringRebalance` to your worst case event processing time and `RebalanceTimeout` to around 120% of that. For example, if your worse case event processing time is 4 minutes, you would set `MaxWaitHandlerDuringRebalance` to 4 minutes and the `RebalanceTimeout` to 5 minutes. Upping the rebalance timeout guarantees that the replica or replicas waiting (up to) 4 minutes for the event to process will not be ejected from the consumer group for 5 minutes (causing another rebalance).

> Note: Nuclio's Kafka client `sarama` performs pre-fetching of `Channel Buffer Size` messages from Kafka into the partition consumer queue. It does so to reduce the number of times it needs to contact Kafka for messages and to allow workers to (almost) always have a set of messages waiting to be processed without having to wait a round trip time to Kafka fetching them. Under rebalancing, regardless whether you prefer minimizing duplicates or higher throughput - the messages in this queue will be discarded and have no effect on rebalancing time (i.e. it doesn't matter if you have one message in the queue or 256, they will all simply be discarded and re-fetched by the replica handling this partition in the new generation)
