# kinesis-cli

kinesis-cli is a tool for interacting with kinesis from the command line.

## Setup

You can either install the kinesis-cli using `go get` or `go install`:
(TODO: verify this works)

    $ go install github.com/sendgridlabs/go-kinesis/kinesis-cli

or build it and run it from the kinesis-cli folder:

```
$ go get github.com/sendgridlabs/go-kinesis/kinesis-cli
$ cd $GOPATH/src/github.com/sendgridlabs/go-kinesis/kinesis-cli
$ go build
$ ./kinesis-cli
Usage: ./kinesis-cli <command> [<arg>, ...]
(Note: expects $AWS_ACCESS_KEY and $AWS_SECRET_KEY to be set)
Commands:
       create   <streamName> [<numShards>]
       delete   <streamName>
       describe <streamName> [<startShardId> <limit>]
       split    <streamName> <shardId> [<hash>]
       merge    <streamName> <shardId> <adjacentShardId>
```

Note that you'll need to store your access/secret key in the proper env vars:

    $ export AWS_ACCESS_KEY=123myaccesskey456; export AWS_SECRET_KEY=789myVerySecretKey432

## Usage

For all commands except `describe`, you will be prompted for confirmation before the aws request is sent.

##### Create a new stream: (only a single shard is created if num shards is not specified)

	$ ./kinesis-cli create somestream 2

##### Delete an existing stream:

    $ ./kinesis-cli delete somestream

##### Describe a stream:

```
$ ./kinesis-cli describe somestream
{
    "StreamDescription": {
        "HasMoreShards": false,
        "Shards": [
            {
                "AdjacentParentShardId": "",
                "HashKeyRange": {
                    "EndingHashKey": "170141183460469231731687303715884105727",
                    "StartingHashKey": "0"
                },
                "ParentShardId": "",
                "SequenceNumberRange": {
                    "EndingSequenceNumber": "",
                    "StartingSequenceNumber": "49540491727041816751370913972624375777284624614827229185"
                },
                "ShardId": "shardId-000000000000"
            },
            {
                "AdjacentParentShardId": "",
                "HashKeyRange": {
                    "EndingHashKey": "340282366920938463463374607431768211455",
                    "StartingHashKey": "170141183460469231731687303715884105728"
                },
                "ParentShardId": "",
                "SequenceNumberRange": {
                    "EndingSequenceNumber": "",
                    "StartingSequenceNumber": "49540491727064117496569444595765911495557272976333209617"
                },
                "ShardId": "shardId-000000000001"
            }
        ],
        "StreamARN": "arn:aws:kinesis:us-east-1:123456789:stream/somestream",
        "StreamName": "somestream",
        "StreamStatus": "ACTIVE"
    }
}

```

##### Split a shard: (it will suggest a new hash key that evenly splits the shard)

```
$ ./kinesis-cli split somestream shardId-000000000000
Shard's current hash key range (0 - 170141183460469231731687303715884105727)
Default (even split) key: 85070591730234615865843651857942052863
Type new key or press [enter] to choose default: 
Are you sure you want to split shard shardId-000000000000 at hash key 85070591730234615865843651857942052863?
(y/N): y
```

##### Merge two adjacent shards: (must be specified in low->high order)

    $ go build && ./kinesis-cli merge somestream shardId-000000000003 shardId-000000000001
