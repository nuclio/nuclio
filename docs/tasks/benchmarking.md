# Benchmarking

This guide walks through a few steps to benchmark nuclio from scratch.

#### In this document
- [Setting up a benchmark system](#setting-up-a-benchmark-system)
- [Benchmark Golang](#benchmark-golang)
- [Benchmark Python 2.7](#benchmark-python-27)
- [Benchmark PyPy](#benchmark-pypy)
- [Benchmark .NET Core](#benchmark-net-core)
- [Benchmark Java](#benchmark-java)
- [Benchmark NodeJS](#benchmark-nodejs)

## Setting up a benchmark system

To benchmark nuclio, you will need three components:

1. [Docker](https://www.docker.com): You'll use the "local" platform to benchmark so all you'll need is Docker
2. [wrk](https://github.com/wg/wrk/wiki/Installing-Wrk-on-Linux): A benchmarking utility
3. [`nuctl`](https://github.com/nuclio/nuclio/releases): All you'll need is the `nuctl` CLI. It will pull all the necessary components

Obviously nuclio will only be as fast as the hardware it runs on. In this case you'll showcase benchmarks on an AWS `c5.9xlarge` - a 36 core machine. With `nuclio` you leverage parallelism, so adding cores contributes to performance. In these examples you'll set the # of workers to the # of cores - experiment on your platform to get the most performance.

## Benchmark Golang

Deploy an empty Go function with 36 workers:
```sh
nuctl deploy helloworld-go -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/development/hack/examples/golang/empty/empty.go --platform local --triggers '{"mh": {"kind": "http", "maxWorkers": 36}}'
```

Run the benchmark:
```sh
wrk -c 36 -t 36 -d 10 http://172.17.0.1:39150
Running 10s test @ http://172.17.0.1:39150
  36 threads and 36 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   105.23us  104.13us  11.52ms   91.01%
    Req/Sec    10.74k   329.50    11.95k    68.97%
  3882355 requests in 10.10s, 336.93MB read
Requests/sec: 384418.84
Transfer/sec:     33.36MB
```

## Benchmark Python 2.7

Deploy an empty Python function with 36 workers:
```sh
nuctl deploy helloworld-py -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/development/hack/examples/python/empty/empty.py --platform local --triggers '{"mh": {"kind": "http", "maxWorkers": 36}}' --runtime python:2.7 --handler empty:handler
```

Run the benchmark:
```sh
wrk -c 36 -t 36 -d 10 http://172.17.0.1:36724
Running 10s test @ http://172.17.0.1:36724
  36 threads and 36 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     0.93ms    1.04ms  16.76ms   93.78%
    Req/Sec     1.29k   115.26     3.85k    74.71%
  461755 requests in 10.10s, 51.52MB read
Requests/sec:  45718.31
Transfer/sec:      5.10MB
```

## Benchmark PyPy

Deploy an empty Python function with 36 workers:
```sh
nuctl deploy helloworld-py -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/development/hack/examples/python/empty/empty.py --platform local --triggers '{"mh": {"kind": "http", "maxWorkers": 36}}' --runtime pypy --handler empty:handler
```

Run the benchmark:
```sh
wrk -c 36 -t 36 -d 10 http://172.17.0.1:36724
Running 10s test @ http://172.17.0.1:36724
  36 threads and 36 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     1.68ms    4.08ms 111.06ms   94.24%
    Req/Sec     1.80k   632.74    12.83k    87.67%
  646945 requests in 10.10s, 56.14MB read
  Socket errors: connect 0, read 2, write 0, timeout 0
Requests/sec:  64056.55
Transfer/sec:      5.56MB
```

## Benchmark .NET Core

Deploy an empty C# function with 36 workers:
``` sh
nuctl deploy helloworld-dotnetcore -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/development/hack/examples/dotnetcore/empty/empty.cs --platform local --triggers '{"mh": {"kind": "http", "maxWorkers": 36}}' --runtime dotnetcore --handler nuclio:empty
```

Run the benchmark:
```sh
wrk -c 36 -t 36 -d 10 http://172.17.0.1:39741
Running 10s test @ http://172.17.0.1:45044
  36 threads and 36 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     1.22ms    2.78ms  37.28ms   96.15%
    Req/Sec     1.37k   229.02     4.29k    77.13%
  492328 requests in 10.10s, 57.75MB read
Requests/sec:  48746.13
Transfer/sec:      5.72MB
```

## Benchmark Java

Deploy an empty Java function with 36 workers:
```sh
nuctl deploy helloworld-java -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/development/hack/examples/java/empty/EmptyHandler.java --platform local --triggers '{"mh": {"kind": "http", "maxWorkers": 36}}' --runtime java --handler EmptyHandler
```

Run the benchmark:
```sh
wrk -c 36 -t 36 -d 10 http://172.17.0.1:45906
Running 10s test @ http://172.17.0.1:45906
  36 threads and 36 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency     1.41ms    4.75ms 120.09ms   96.88%
    Req/Sec     1.45k   492.42     2.51k    69.82%
  520925 requests in 10.02s, 58.12MB read
Requests/sec:  51999.28
Transfer/sec:      5.80MB
```

## Benchmark NodeJS

Deploy an empty NodeJS function with 36 workers:
```sh
nuctl deploy helloworld-njs -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/development/hack/examples/nodejs/empty/empty.js --platform local --triggers '{"mh": {"kind": "http", "maxWorkers": 36}}' --runtime nodejs --handler empty:handler
```

Run the benchmark:
```sh
wrk -c 36 -t 36 -d 10 http://172.17.0.1:39061
Running 10s test @ http://172.17.0.1:39061
  36 threads and 36 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   804.12us    1.08ms  19.78ms   93.50%
    Req/Sec     1.64k   228.41     4.83k    79.54%
  589646 requests in 10.10s, 65.79MB read
Requests/sec:  58384.11
Transfer/sec:      6.51MB
```

