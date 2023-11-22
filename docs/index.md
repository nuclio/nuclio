# Nuclio Overview 
Learn how to use Nuclio to quickly develop "serverless" applications.

To download Nuclio and view full product release notes, visit the Nuclio GitHub [Releases](https://github.com/nuclio/nuclio/releases) page.

For support and additional product information, join the active [Nuclio Slack](https://nuclio-io.slack.com/) workspace.


## What is Nuclio?

Nuclio is a high-performance "serverless" framework focused on data, I/O, and compute intensive workloads. It is well integrated with popular data science tools, such as [Jupyter](https://jupyter.org/) and [Kubeflow](https://www.kubeflow.org/); supports a variety of data and streaming sources; and supports execution over CPUs and GPUs. The Nuclio project began in 2017 and is constantly and rapidly evolving; many start-ups and enterprises are now using Nuclio in production.

You can use Nuclio as a standalone Docker container or on top of an existing [Kubernetes](https://kubernetes.io) cluster; see the deployment instructions in the Nuclio documentation. You can also use Nuclio through a fully managed application service (in the cloud or on-prem) in the [Iguazio Data Science Platform](https://www.iguazio.com/), which you can [try for free](https://go.iguazio.com/start-your-free-trial).

If you wish to create and manage Nuclio functions through code - for example, from Jupyter Notebook - see the [Nuclio Jupyter project](https://github.com/nuclio/nuclio-jupyter), which features a Python package and SDK for creating and deploying Nuclio functions from Jupyter Notebook.
Nuclio is also an integral part of the new open-source [MLRun](https://github.com/mlrun/mlrun) library for data science automation and tracking and of the open-source [Kubeflow Pipelines](https://www.kubeflow.org/docs/components/pipelines/) framework for building and deploying portable, scalable ML workflows.

Nuclio is extremely fast: a single function instance can process hundreds of thousands of HTTP requests or data records per second.
This is 10-100 times faster than some other frameworks. To learn more about how Nuclio works, see the Nuclio [architecture](./concepts/architecture.md) documentation, read this review of [Nuclio vs. AWS Lambda](https://theburningmonk.com/2019/04/comparing-nuclio-and-aws-lambda/), or watch the [Nuclio serverless and AI webinar](https://www.youtube.com/watch?v=pTCx569Kd4A).
You can find links to additional articles and tutorials on the [Nuclio web site](https://nuclio.io/).

Nuclio is secure: Nuclio is integrated with [Kaniko](https://github.com/GoogleContainerTools/kaniko) to allow a secure and production-ready way of building Docker images at run time.

### Why another "serverless" project?

None of the existing cloud and open-source serverless solutions addressed all the desired capabilities of a serverless framework:

- Real-time processing with minimal CPU/GPU and I/O overhead and maximum parallelism
- Native integration with a large variety of data sources, triggers, processing models, and ML frameworks
- Stateful functions with data-path acceleration
- Portability across low-power devices, laptops, edge and on-prem clusters, and public clouds
- Open-source but designed for the enterprise (including logging, monitoring, security, and usability)

Nuclio was created to fulfill these requirements.  It was intentionally designed as an extendable open-source framework, using a modular and layered approach that supports constant addition of triggers and runtimes, with the hope that many will join the effort of developing new modules, developer tools, and platforms for Nuclio.

Try it for yourself by following the [Quick-Start](tasks/quick-start.md) guide!


