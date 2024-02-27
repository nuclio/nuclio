Kinesis trigger
===============

Reads records from `Amazon Kinesis <https://aws.amazon.com/kinesis/>`_ streams.

In this document
----------------
- `Attributes <#attributes>`__
- `Examples <#example>`__
- `IAM Configuration <#iam-configuration>`__

.. _attributes:

Attributes
----------

.. csv-table::
   :header: Path, Type, Description
   :delim: |

   accessKeyID | string | Required by AWS Kinesis
   secretAccessKey | string | Required by AWS Kinesis
   regionName | string | Required by AWS Kinesis
   streamName | string | Required by AWS Kinesis
   shards | string | List of shards on which this function receives events

.. _example:

Example
-------

.. code-block:: yaml

    triggers:
      myKinesisStream:
        kind: kinesis
        attributes:
          accessKeyID: "my-key"
          secretAccessKey: "my-secret"
          regionName: "eu-west-1"
          streamName: "my-stream"
          shards: [shard-0, shard-1, shard-2]

.. _iam-configuration:

IAM Configuration
-----------------

The minimal policy-actions needed for Kinesis trigger to consume messages are:

- ``kinesis:GetShardIterator``
- ``kinesis:GetRecords``
- ``kinesis:DescribeStream``

For example:

.. code-block:: json

    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Sid": "VisualEditor0",
          "Effect": "Allow",
          "Action": [
            "kinesis:GetShardIterator",
            "kinesis:GetRecords",
            "kinesis:DescribeStream"
          ],
          "Resource": "arn:aws:kinesis:<region-name>:<user-unique-id>:stream/<specific-stream>"
        }
      ]
    }
