Cron trigger
=============

In this document
----------------
- `Overview <#overview>`__
- `Attributes <#attributes>`__
- `Examples <#examples>`__

.. _overview:

Overview
--------
Triggers the function according to a schedule or interval, with an optional body.

.. _attributes:

Attributes
----------
.. csv-table::
   :header: Path,Type,Description
   :delim: |

   schedule | string | A cron-like schedule (for example, ``*/5 * * * *``).
   interval | string | An interval (for example, ``1s``, ``30m``).
   concurrencyPolicy | string | Concurrency policy - ``"Allow"``, ``"Forbid"``, or ``"Replace"``; (default: ``"Forbid"``). Applicable only when using CronJobs on Kubernetes platforms.
   jobBackoffLimit | int32 | The number of retries before failing a job; (default: ``2``). Applicable only when using CronJobs on Kubernetes platforms.
   event.body | string | The body passed in the event.
   event.headers | map of string/int | The headers passed in the event.

Note:
~~~~~
1. You must set either the ``schedule`` or ``interval`` mutually-exclusive attributes.
2. The ``event.*`` attributes are optional.
3. **[Tech Preview]** On Kubernetes platforms, you can set the ``cronTriggerCreationMode`` platform-configuration field to ``"kube"`` to run the triggers as Kubernetes CronJobs instead of the default implementation of running Cron triggers from the Nuclio processor.
   For more information, see `Configuring a Platform <../../tasks/configuring-a-platform.html#cron-trigger-creation-mode-crontriggercreationmode>`_.
   When running Cron triggers as CronJobs:
- The created CronJob uses `wget` to call the default HTTP trigger of the function according to the configured interval or schedule. (This means that worker-related attributes are irrelevant.)
- The `wget` request is sent with the header ``"x-nuclio-invoke-trigger: cron"``.
- You can use the ``concurrencyPolicy`` and ``jobBackoffLimit`` attributes to configure the CronJobs.

.. _examples:

Examples
--------

.. code-block:: yaml

   triggers:
     myCronTrigger:
       kind: cron
       attributes:
         interval: 3s

The following example is demonstrates a configuration for running Cron triggers as Kubernetes CronJobs, as it sets the ``concurrencyPolicy`` and ``jobBackoffLimit`` attributes.
Remember that this implementation requires setting the ``cronTriggerCreationMode`` platform-configuration field to ``"kube"``.

.. code-block:: yaml

   triggers:
     myCronTrigger:
       kind: cron
       attributes:
         interval: 10s
         concurrencyPolicy: "Allow"
         jobBackoffLimit: 2