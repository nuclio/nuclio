![Periodic](https://github.com/nuclio/nuclio/workflows/Periodic/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/nuclio/nuclio)](https://goreportcard.com/report/github.com/nuclio/nuclio)
[![Slack](https://img.shields.io/badge/slack-join%20chat%20%E2%86%92-e01563.svg)](https://lit-oasis-83353.herokuapp.com/)

<p align="center"><img src="/docs/assets/images/logo.png" width="180"/></p>

# Nuclio - 用于实时事件和数据处理的 "serverless" 框架

<p align="center">
访问 <a href="https://nuclio.io">nuclio.io</a> 获取更多产品信息和新闻，以及一个体验友好的 Web 版 Nuclio <a href="https://nuclio.io/docs/latest/">文档</a>.
</p>

翻译：

- [English](./README.md)

#### 关于本文
- [概览](#概览)
- [为什么需要另一个 "serverless" 项目？](#为什么需要另一个-serverless-项目)
- [快速开始](#快速开始)
- [架构设计](#架构设计)
- [函数示例](#函数示例)
- [推荐阅读](#推荐阅读)

## 概览

Nuclio 是一个高性能的 "serverless" 框架，专注于数据、I/O和计算密集型的工作负载. 它与流行的数据科学工具集成地很好，例如 [Jupyter](https://jupyter.org/) 和 [Kubeflow](https://www.kubeflow.org/); 支持多种类型的数据和流式数据源; 并且支持在CPU和GPU上执行任务。Nuclio 项目于 2017 年启动，并处于持续不断的快速发展中；现如今，许多初创企业已将 Nuclio 应用于生产。

你可以将 Nuclio 以一个独立的 Docker 容器运行或者运行在一个已有的 [Kubernetes](https://kubernetes.io) 集群中; 在 Nuclio 文档中查看具体的部署指南。 也可以通过 [Iguazio 数据科学平台](https://www.iguazio.com/) 中的全权托管应用服务平台（云上或者本地）来使用 Nuclio，它提供[免费的试用](https://go.iguazio.com/start-your-free-trial).

如果想通过编码的方式创建或者管理 Nuclio 函数（functions）- 例如, 使用 Jupyter Notebook - 请参阅 [Nuclio Jupyter 项目](https://github.com/nuclio/nuclio-jupyter), 它包含一个完整的 Python 包和软件开发工具包（SDK）用于通过 Jupyter Notebook 创建和部署 Nuclio 函数。 Nuclio 作为新开源项目 [MLRun](https://github.com/mlrun/mlrun) library，以及开源项目 [Kubeflow Pipelines](https://www.kubeflow.org/docs/components/pipelines/) 中不可或缺的部分，分别提供数据科学自动化及追踪能力，以及构建和部署弹性可迁移机器学习（ML）工作流的能力。 

Nuclio 的运行速度非常快: 单个函数实例每秒钟可以处理成十万的 HTTP 请求或者数据记录。这比其他的框架快了10-100倍。了解更多 Nuclio 的设计和运行原理，请查看 Nuclio [架构](/docs/concepts/architecture.md) 文档, 阅读这篇文章 [Nuclio vs. AWS Lambda](https://theburningmonk.com/2019/04/comparing-nuclio-and-aws-lambda/), 或观看这个视频 [Nuclio serverless and AI webinar](https://www.youtube.com/watch?v=pTCx569Kd4A).你也可以在  [Nuclio 官方网站](https://nuclio.io/) 获取到更多其他文章和指南的链接。

Nuclio 是很安全的: Nuclio 与 [Kaniko](https://github.com/GoogleContainerTools/kaniko) 集成在运行时以一种安全且生产可用的方式构建 Docker 镜像。 

更多疑问或支持， [点击加入](https://lit-oasis-83353.herokuapp.com) [Nuclio Slack](https://nuclio-io.slack.com) 工作空间.

## 为什么需要另一个 "serverless" 项目？

目前云厂商和开源 Serverless 解决方案都没有真正解决一个 Serverless 框架所必须的所有能力：

- 以最小的 CPU/GPU 及 I/O 负载和最大的并行度进行实时处理
- 原生的与各种数据源、触发器、处理模型和机器学习框架集成
- 能够提供数据路径加速的有状态函数
- 具有跨这种设备的可移植性包括，低功耗设备、笔记本电脑、边缘节点、本地集群以及公有云
- 开源同时专注于企业级应用场景（包括日志记录、监控、安全性和可用性）

Nuclio 项目就是为满足这些需求而启动的。它的设计哲学就是作为一个可扩展的开源框架，基于模块化和分层的理念使得可以不断的添加各类触发器和运行时，希望越来越多的人能够参与到 Nuclio 项目，为 Nuclio 生态开发新的模块、工具和平台。

## 快速开始

All you need to run the dashboard is Docker:

探索尝试 Nuclio 的最简单方式是通过运行项目提供的图形用户界面（Nuclio [仪表盘](#dashboard)。你可以很轻松的通过 Docker 将它运行起来：

```sh
docker run -p 8070:8070 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /tmp:/tmp \
  --name nuclio-dashboard \
  quay.io/nuclio/dashboard:stable-amd64
```

![dashboard](/docs/assets/images/dashboard.png)

在浏览器中访问 http://localhost:8070, 新建一个项目，并在其中添加一个函数。 当你在一些编排平台（例如，Kubernetes）之外运行时，仪表盘将会被直接运行在本地的 Docker 进程中。

假设你已经在 Docker 中运行了 Nuclio，作为一个简单的例子，新建一个项目，并部署预先已经存在的模板"dates (nodejs)"。执行 `docker ps` 命令, 你将能看到这个函数已经被部署到他自己的容器中。然后，你可以通过 `curl` 工具调用你的函数；（请示用 `docker ps` 指令或者 Nuclio 的仪表盘来检查端口号是否正确）：


```sh
curl -X POST \
  -H "Content-Type: application/text" \
  -d '{"value":2,"unit":"hours"}' \
  http://localhost:37975
```

关于如何在 Kubernetes 集群、或者是 Nuclio 的 UI或和命令行工具（`nuctl`）中正确使用 Nuclio 的完整步骤，请参阅一下学习路径：

- [在 Kubernetes 中使用 Nuclio](/docs/setup/k8s/getting-started-k8s.md)
- [在 GKE（Google Kubernetes Engine） 中使用 Nuclio](/docs/setup/gke/getting-started-gke.md)
- [在 AKS（Azure Container Services） 中使用 Nuclio](/docs/setup/aks/getting-started-aks.md)
- [在 Katacode 中使用免费的 Kubernetes 沙箱环境中，按步骤运行 Nuclio](https://katacoda.com/javajon/courses/kubernetes-serverless/nuclio)

## 架构设计

“当发生这种事情时，请这样做”。Nuclio 试图抽象所有围绕事件已经发生的脚手架工具（例如，将消息记录写入 Kafka，发起一个 HTTP 请求，计时器到期等）并将此信息发送给一段代码逻辑来处理。为了实现这个目标，Nuclio 希望用户可以提供（至少）关于什么可以触发一个事件并且在发生此类事件时应该由哪一段代码逻辑来进行处理的详细信息。用户可以通过命令行工具（`nuctl`）、REST API或者一个可视化的 Web 端应用程序。

![architecture](/docs/assets/images/architecture-3.png)

Nuclio 获取这些信息（通常称为函数处理程序 `handler` 和函数配置 `configuration`）并发送给构建器。构建器将会制作函数的容器镜像，其中包含用户提供的函数处理程序以及一个可以在接收到事件后执行该函数处理程序的软件工具（稍后有更详细的介绍）。然后，构建器将该该容器镜像发布到容器镜像注册表中。

一旦发布完成，这个函数的容器镜像就可以被部署了。部署器将从函数的配置中生成编排平台所需的特定配置文件。例如，如果是部署在 Kubernetes 集群中，部署器将会读取配置文件中的副本数量、自动扩缩容时间、函数需要的 GPU 数量等参数，并将它们转化为 Kubernetes 的资源配置（例如， Deployment, Service, Ingress 等）。

> 注意: 部署器不会直接创建 Kubernetes 的原生资源，而是会创建一个叫作 “NuclioFunction” 的自定义资源（CRD）。一个被称为”控制器“的 Nuclio 服务将会监听到 NuclioFunction 这个 CRD 的变化，并创建、修改或者删除可变更的 Kubernetes 原生资源（(Deployment, Service等），这符合标准的 Kubernetes 控制器模式。

编排器将会从已经发布的容器镜像中启动容器并执行它们，并将函数的配置文件传递到容器中。这些容器的入口点（entrypoint）就是”处理器“，负责读取配置文件、监听事件触发器（例如，连接到 Kafka、监听 HTTP 端口等），当事件发生时，读取事件并调用用户的函数处理程序。处理器还负责很多其他的事情，包括处理指标、编码响应以及优雅地处理崩溃等。
The entrypoint of these containers is the "processor", responsible for reading the configuration, listening to event triggers (e.g. connecting to Kafka, listening for HTTP), reading events when they happen and calling the user's handler. The processor is responsible for many, many other things including handling metrics, marshaling responses, gracefully handling crashes, etc. 

### 缩容至零

一旦构建并部署到 Kubernetes 这样的编排平台中，Nuclio 函数（即处理器）就可以处理事件，并根据性能指标、发送日志和指标等进行扩缩容——所有这些都不需要任何外部实体的帮助。部署完成后，就可以关闭 Nuclio 的仪表盘和控制器，Nuclio 函数依然可以完成的运行和扩缩容。

但是，缩容到零的能力，单单依靠函数自身是无法完成的。相反地——一旦缩容到零，当一个新的事件到达时，Nuclio 函数无法完成自己的扩容操作。为此， Nuclio 有一个扩缩器服务。它解决了将函数缩容到零，以及从零开始扩容的问题。

## 函数示例

如下示例函数实现了使用 `Event` 和 `Context` 接口来处理输入和日志，并返回结构化的 HTTP 响应；（也可以使用简单字符串作为返回值）。

以 Go 语言为例：

```golang
package handler

import (
    "github.com/nuclio/nuclio-sdk-go"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
    context.Logger.Info("Request received: %s", event.GetPath())

    return nuclio.Response{
        StatusCode:  200,
        ContentType: "application/text",
        Body: []byte("Response from handler"),
    }, nil
}
```

以 Python 语言为例：

```python
def handler(context, event):
    response_body = f'Got {event.method} to {event.path} with "{event.body}"'

    # log with debug severity
    context.logger.debug('This is a debug level message')

    # just return a response instance
    return context.Response(body=response_body,
                            headers=None,
                            content_type='text/plain',
                            status_code=201)
```

更多的示例请访问 Nuclio 项目中的 **[hack/examples](hack/examples/README.md)** 目录。


## 推荐阅读

- 安装入门
  - [在 Docker 中运行 Nuclio](/docs/setup/docker/getting-started-docker.md)
  - [在 Minikube 中运行 Nuclio](/docs/setup/minikube/getting-started-minikube.md)
  - [在 Kubernetes 中运行 Nuclio](/docs/setup/k8s/getting-started-k8s.md)
  - [在 Azure Kubernetes Service (AKS) 中运行 Nuclio](/docs/setup/aks/getting-started-aks.md)
  - [在 Google Kubernetes Engine (GKE) 中运行 Nuclio](/docs/setup/gke/getting-started-gke.md)
  - 在树莓派中运行 Nuclio (coming soon)
- 部署任务
  - [部署函数](/docs/tasks/deploying-functions.md)
  - [在 Dockerfile 中部署函数](/docs/tasks/deploy-functions-from-dockerfile.md)
  - [部署预编译的函数](/docs/tasks/deploying-pre-built-functions.md)
  - [在特定平台中配置函数](/docs/tasks/configuring-a-platform.md)
- 相关概念
  - [最佳实践和常见陷阱](/docs/concepts/best-practices-and-common-pitfalls.md)
  - [架构设计](/docs/concepts/architecture.md)
  - Kubernetes
    - [基于 Kubernetes Ingress 通过名称调用函数](/docs/concepts/k8s/function-ingress.md)
- 其他引文
  - [命令行工具 `nuctl`](/docs/reference/nuctl/nuctl.md)
  - [函数配置参考文档](/docs/reference/function-configuration/function-configuration-reference.md)
  - [触发器](/docs/reference/triggers)
  - [运行时 - .NET Core 3.1](/docs/reference/runtimes/dotnetcore/writing-a-dotnetcore-function.md)
  - [运行时 - Shell](/docs/reference/runtimes/shell/writing-a-shell-function.md)
- [示例](hack/examples/README.md)
- 沙箱环境
  - [在免费的 Kubernetes 集群中安装 Nuclio 并运行函数，以进行各种探索和试验](https://katacoda.com/javajon/courses/kubernetes-serverless/nuclio)
- 贡献指南
  - [代码规范](/docs/devel/coding-conventions.md)
  - [像 Nuclio 做贡献](/docs/devel/contributing.md)
- 媒体平台
  - [使用 Nuclio 运行告诉的 Serverless (PPT)](https://www.slideshare.net/iguazio/running-highspeed-serverless-with-nuclio)
  - [CNCF 网络研讨会 – Serverless and AI (视频)](https://www.youtube.com/watch?v=pTCx569Kd4A)
  - [使用 Servreless 加快 AI 开发 (指南)](https://dzone.com/articles/tutorial-faster-ai-development-with-serverless)
  - [Nuclio 即 Serverless 的未来 (博客)](https://thenewstack.io/whats-next-serverless/)
  - [Nuclio: 新兴的 Serverless 超级英雄 (博客))](https://hackernoon.com/nuclio-the-new-serverless-superhero-3aefe1854e9a)
  - [为实时应用程序而生的 Serverless 框架 (博客)](https://www.rtinsights.com/serverless-framework-for-real-time-apps-emerges/)

[加入](https://lit-oasis-83353.herokuapp.com) 活跃的 [Nuclio Slack](https://nuclio-io.slack.com) 空间，以获取支持或更多产品信息。
