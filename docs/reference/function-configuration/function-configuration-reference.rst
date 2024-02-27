Function-Configuration Reference
=================================

This document provides a reference of the Nuclio function configuration.

In This Document
----------------

- `Basic configuration structure <#basic-structure>`__
- `Function Metadata <#metadata>`__
    - `Example <#example>`__
- `Function Specification <#specification>`__
    - `Example <#spec-example>`__
- `Function Status <#status>`__
    - `Function State <#function-state-state>`__
    - `Example <#status-example>`__
- `See also <#see-also>`__

Basic configuration structure
-----------------------------

The basic structure of the Nuclio function configuration resembles Kubernetes resource definitions, and includes
the ``apiVersion``, ``kind``, ``metadata``, ``spec``, and ``status`` sections. Following is an example of a minimal definition:

.. code-block:: yaml
   :linenos:

   apiVersion: "nuclio.io/v1"
   kind: NuclioFunction
   metadata:
     name: example
   spec:
     image: example:latest

.. _metadata:

Function Metadata (``metadata``)
--------------------------------

The ``metadata`` section includes the following attributes:

.. csv-table::
   :header: Path, Type, Description
   :widths: 20, 10, 70
   :delim: |

   name|string|The name of the function
   namespace|string|A level of isolation provided by the platform (e.g., Kubernetes)
   labels|map|A list of key-value tags that are used for looking up the function (immutable\, can't update after first deployment)
   annotations|map|A list of annotations based on the key-value tags

Example
~~~~~~~

.. code-block:: yaml
   :linenos:

   metadata:
     name: example
     namespace: nuclio
     labels:
       l1: lv1
       l2: lv2
       l3: 100
     annotations:
       a1: av1

.. _specification:

Function Specification (``spec``)
------------------------------------

The `spec` section contains the requirements and attributes and has the following elements:

.. csv-table::
   :header: Path, Type, Description
   :delim: |

   description | string | A textual description of the function
   handler | string | The entry point to the function, in the form of ``package:entrypoint``; varies slightly between runtimes, see the appropriate runtime documentation for specifics
   runtime | string | The name of the language runtime - ``golang`` \ ``python:3.7`` \ ``python:3.8`` \ ``python:3.9`` \ ``python:3.10`` \ ``python:3.11`` \ ``shell`` \ ``java`` \ ``nodejs``
   image | string | The name of the function's container image; used for the ``image`` type of ``build.codeEntryType``; See :doc:`Code-Entry Types <./code-entry-types>`
   env | map | A name-value environment-variables tuple; it's also possible to reference secrets from the map elements, as demonstrated in the :ref:`specification example <spec-example>`
   envFrom | []v1.EnvFromSource | List of sources from which the function takes environment variables (ConfigMaps/Secrets). It is being merged with the correspondent platform ``runtime.common.envFrom``. The function's values have a higher priority.
   volumes | map | A map in an architecture similar to Kubernetes volumes, for Docker deployment
   replicas | int | The number of desired instances; 0 for auto-scaling.
   minReplicas | int | The minimum number of replicas
   platform.attributes.restartPolicy.name | string | The name of the restart policy for the function-image container; applicable only to Docker platforms
   platform.attributes.restartPolicy.maximumRetryCount | int | The maximum retries for restarting the function-image container; applicable only to Docker platforms
   platform.attributes.mountMode | string | Function mount mode, which determines how Docker mounts the function configurations - ``bind`` \ ``volume`` (default: ``bind``); applicable only to Docker platforms
   platform.attributes.healthCheckInterval | string,int | The interval between health checks, in seconds or as a duration string (e.g., ``5s``, ``1m``, ``1h``).
   maxReplicas | int | The maximum number of replicas
   targetCPU | int | Target CPU when auto scaling, as a percentage (default: 75%)
   dataBindings | See reference | A map of data sources used by the function ("data bindings")
   triggers.(name).numWorkers | int | The max number of concurrent requests this trigger can process
   [**Deprecated:** ] triggers.(name).maxWorkers | int | **Deprecated:** The max number of concurrent requests this trigger can process
   triggers.(name).kind | string | The trigger type (kind) - ``cron`` \ ``eventhub`` \ ``http`` \ ``kafka-cluster`` \ ``kinesis`` \ ``nats`` \ ``rabbit-mq``
   triggers.(name).url | string | The trigger specific URL (not used by all triggers)
   triggers.(name).annotations | list of strings | Annotations to be assigned to the trigger, if applicable
   triggers.(name).workerAvailabilityTimeoutMilliseconds | int | The number of milliseconds to wait for a worker if one is not available. 0 = never wait (default: 10000, which is 10 seconds)
   triggers.(name).attributes | See :doc:`reference name <../../reference/triggers/index>` | The per-trigger attributes
   build.path | string | The URL of a GitHub repository or an archive-file that contains the function code; for the ``git``, ``github`` or ``archive`` types of ``build.codeEntryType``; or the URL of a function source-code file; see :doc:`Code-Entry Types <./code-entry-types>`
   build.functionSourceCode | string | Base-64 encoded function source code for the ``sourceCode`` type of ``build.codeEntryType`` ; see :doc:`Code-Entry Types <./code-entry-types>`
   build.registry | string | The container image repository to which the built image will be pushed
   build.noBaseImagePull | string | Do not pull any base images when building, use local images only
   build.noCache | string | Do not use any caching when building container images
   build.baseImage | string | The name of a base container image from which to build the function's processor image
   build.Commands | list of string | Commands run opaquely as part of container image build
   build.onbuildImage | string | The name of an "onbuild" container image from which to build the function's processor image; the name can include :code:`{{ .Label }}` and :code:`{{ .Arch }}` for formatting
   build.image | string | The name of the built container image (default: the function name)
   build.flags | []string | Build flags to pass to the container builder-pusher. List of flags is here: List of flags is here: `Kaniko flags <https://github.com/GoogleContainerTools/kaniko?tab=readme-ov-file#additional-flags>`_,  `Docker flags <https://docs.docker.com/engine/reference/commandline/image_build/>`_
   build.codeEntryType | string | The function's code-entry type - ``archive`` \ ``git`` \ ``github`` \ ``image`` \ ``s3`` \ `sourceCode`; see :doc:`Code-Entry Types <./code-entry-types>`
   build.codeEntryAttributes | See :doc:`reference <./code-entry-types>` | Code-entry attributes, which provide information for downloading the function when using the ``github``, ``s3``, or ``archive`` types of ``build.codeEntryType``
   build.builderServiceAccount | string | The name of the service account for the builder pods (relevant for a kubernetes setup with ``kaniko`` container builder
   runRegistry | string | The container image repository from which the platform will pull the image
   runtimeAttributes | See :doc:`reference <../../reference/runtimes/index>` | Runtime-specific attributes
   resources | See `reference <https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/>`__ | Limit resources allocated to deployed function
   readinessTimeoutSeconds | int | Number of seconds that the controller will wait for the function to become ready before declaring failure (default: 60)
   waitReadinessTimeoutBeforeFailure | bool | Wait for the expiration of the readiness timeout period even if the deployment fails or isn't expected to complete before the readinessTimeout expires
   avatar | string | Base64 representation of an icon to be shown in UI for the function (Deprecated)
   eventTimeout | string | Global event timeout, in the format supported for the ``Duration`` parameter of the `time.ParseDuration <https://golang.org/pkg/time/#ParseDuration>`__ Go function
   securityContext.runAsUser | int | The user ID (UID) for running the entry point of the container process
   securityContext.runAsGroup | int | The group ID (GID) for running the entry point of the container process
   securityContext.fsGroup | int | A supplemental group to add and use for running the entry point of the container process
   serviceType | string | Describes ingress methods for a service
   affinity | v1.Affinity | Set of rules used to determine the node that schedule the pod
   nodeSelector | map | Constrain function pod to a node by key-value pairs selectors
   nodeName | string | Constrain function pod to a node by node name
   priorityClassName | string | Indicates the importance of a function pod relatively to other function pods
   preemptionPolicy | string | Function pod preemption policy (one of ``Never`` or ``PreemptLowerPriority``)
   tolerations | []v1.Toleration | Function pod tolerations
   disableSensitiveFieldsMasking | bool | Don't scrub sensitive information form the function configuration
   customScalingMetricSpecs | autosv2.MetricSpec | Custom function horizontal pod autoscaling  `metric spec <https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#metricspec-v2-autoscaling>`_, allowing to override the default
   devices | []string | List of devices to be made available to the function. Relevant for local platform only. (e.g. /dev/video0:/dev/video0:rwm)
   disableDefaultHttpTrigger | *bool | Disable default http trigger creation. If flag isn’t set, value is taken from the platform config.
   initContainers | []*v1.Container |See `kubernetes docs for init containers <https://kubernetes.io/docs/concepts/workloads/pods/init-containers/>`_ for more info
   sidecars | []*v1.Container | See `kubernetes docs for sidecars <https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/>`_ for more info

.. _spec-example:

Example
~~~~~~~~~~~~~~~~~

.. code-block:: yaml

    spec:
      description: my Go function
      handler: main:Handler
      runtime: golang
      image: myfunctionimage:latest
      platform:
        attributes:
          restartPolicy:
            name: on-failure
            maximumRetryCount: 3
          healthCheckInterval: 10s
          mountMode: volume
      env:
        - name: SOME_ENV
          value: abc
        - name: SECRET_PASSWORD_ENV_VAR
          valueFrom:
            secretKeyRef:
              name: my-secret
              key: password
      volumes:
        - volume:
            hostPath:
              path: "/var/run/docker.sock"
          volumeMount:
            mountPath: "/var/run/docker.sock"
      minReplicas: 2
      maxReplicas: 8
      targetCPU: 60
      build:
        registry: localhost:5000
        noBaseImagePull: true
        noCache: true
        commands:
          - apk --update --no-cache add curl
          - pip install simplejson
      resources:
        requests:
          cpu: 1
          memory: 128M
        limits:
          cpu: 2
          memory: 256M
          nvidia.com/gpu: 1
      securityContext:
        runAsUser: 1000
        runAsGroup: 2000
        fsGroup: 3000

.. _status:

Function Status (``status``)
------------------------------------

The `status` section contains the requirements and attributes and has the following elements:

.. csv-table::
   :header: Path,Type,Description
   :delim: |

   state | string | A textual representation of the function status
   message | string | Function state message, mostly in use to represent why a function has failed
   logs | map | The function deployment logs to be returned
   scaleToZero | object | The details of the last scale event of the function (contains event message and time)
   apiGateways | []string | A list of the function's api-gateways
   httpPort | int | The http port used to invoke the function
   containerImage | string | The name of the built function container image, including the registry.
   internalInvocationUrls | []string | A list of internal urls to invoke the function
   externalInvocationUrls | []string | A list of external urls to invoke the function, including ingresses and external-ip:function-port


Function state (``state``)
~~~~~~~~~~~~~~~~~~~~~~~~~~~

The `state` field describes the current function status, and can be one of the following:

.. csv-table::
   :header: State, Description
   :delim: |

   ready | Function is deployed successfully and ready to process events.
   imported | Function is imported but not yet deployed.
   scaledToZero | Function is scaled to zero, so the number of function replicas is zero.
   building | Function image is being built.
   waitingForResourceConfiguration | Function waits for resources to be ready. For instance, in case of k8s function waits for deployment/pods and etc.
   waitingForScaleResourceFromZero | Function is scaling up from zero replicas.
   waitingForScaleResourceToZero | Function is scaling down to zero replicas.
   error | An error occurred during function deployment that cannot be rectified without redeployment.
   unhealthy | An error occurred during function deployment, which might be resolved over time, and might require redeployment. For example, issues with insufficient resources or a missing image.

.. _status-example:

Example
~~~~~~~~~~~~~~~~~

.. code-block:: yaml

    status:
      state: ready
      scaleToZero:
        lastScaleEvent: resourceUpdated
        lastScaleEventTime: "2022-12-11T16:23:52.130851057Z"
      apiGateways:
        - some-api-gateway
      containerImage: localhost:5000/nuclio-my-function-image-processor:latest
      externalInvocationUrls:
        - ing-nuclio.my-nuclio-domain.com/function-name
      internalInvocationUrls:
        - nuclio-function-name.nuclio.svc.cluster.local:8080

See also
---------

- :doc:`Deploying Functions </tasks/deploying-functions>`
- :doc:`Code-Entry Types </reference/function-configuration/code-entry-types>`
