MQTT trigger
============

Reads messages from `MQTT <http://mqtt.org/>`_ topics.

Attributes
----------

.. csv-table::
   :header: Path, Type, Description
   :delim: |

   subscriptions | subscription (topic, qos) | An MQTT subscription

Example
-------

.. code-block:: yaml

    triggers:
      myMqttTrigger:
        kind: "mqtt"
        url: "10.0.0.3:1883"
        attributes:
            subscriptions:
            - topic: house/living-room/temperature
              qos: 0
            - topic: weather/humidity
              qos: 0

Event
-----

The mqtt trigger emits an event object with the following attributes:

-  ``URL``: The URL of the MQTT broker
- ``topic``: The topic of the message
- ``path``: The topic of the message (alias for ``topic``)
- ``body``: The message payload
- ``id``: The message id
