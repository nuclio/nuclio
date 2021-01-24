# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import argparse
import json
import logging
import os
import pathlib
import shlex
import subprocess

logging.basicConfig(level=logging.INFO)

"""
benchmark.py provides a simple yet quick way to run  HTTP load benchmarking tests against Nuclio function runtimes,
 such as (go, python, ...etc).

Benchmarking output contains:
 - HTML file containing a graph comparing each runtime latency (ms) over time
 - Binary files containing the requests were made during benchmarking
 - Descriptive information about total requests, throughput, duration, etc.

Prerequisites:
 - vegeta                  - https://github.com/tsenart/vegeta
 - python                  - https://realpython.com/intro-to-pyenv (tl;dr: pyenv install 3.8)       

Usage: `python benchmark.py --help`

Or remotely: `wget -qO- \
 https://raw.githubusercontent.com/nuclio/nuclio/development/hack/scripts/benchmark/benchmark.py \
 | python3 /dev/stdin --help`
"""


class Constants(object):
    function_examples_dir = os.path.join("hack", "examples")
    default_workdir = ".benchmarking"


class Runtimes(object):
    golang = "golang"
    python36 = "python:3.6"
    java = "java"
    nodejs = "nodejs"
    dotnetcore = "dotnetcore"

    # NOTE: python is just a reference to python3.6
    python = "python"

    # TODO: support benchmarking - add "empty function'
    # shell = "shell"
    # ruby = "ruby"

    @staticmethod
    def runtime_to_bm_function_handler(name, runtime):
        return {
            "empty": {
                Runtimes.python: "empty:handler",
                Runtimes.python36: "empty:handler",
                Runtimes.golang: "empty:Handler",
                Runtimes.java: "EmptyHandler",
                Runtimes.nodejs: "empty:handler",
                Runtimes.dotnetcore: "nuclio:empty",
            }
        }[name][runtime]

    @staticmethod
    def runtime_to_bm_function_handler_filename(name, runtime):
        return {
            "empty": {
                Runtimes.python: "empty.py",
                Runtimes.python36: "empty.py",
                Runtimes.golang: "empty.go",
                Runtimes.java: "EmptyHandler.java",
                Runtimes.nodejs: "empty.js",
                Runtimes.dotnetcore: "empty.cs",
            }
        }[name][runtime]

    @staticmethod
    def all():
        return [
            Runtimes.golang,
            Runtimes.python36,
            Runtimes.java,
            Runtimes.nodejs,
            Runtimes.dotnetcore,
        ]


class Vegeta(object):
    def __init__(self, logger, workdir):
        self._workdir = pathlib.Path(workdir)
        self._logger = logger.getChild("vegeta_client")

    def attack(self, function_name, function_url, concurrent_requests):
        vegeta_cmd = "vegeta attack" \
                     f" -name {function_name}" \
                     " -duration 10s" \
                     " -rate 0" \
                     f" -connections {concurrent_requests}" \
                     f" -workers {concurrent_requests}" \
                     f" -max-workers {concurrent_requests}" \
                     f" -output {function_name}.bin"

        self._logger.debug(f"Attacking command - {vegeta_cmd}")
        subprocess.run(shlex.split(vegeta_cmd),
                       cwd=self._workdir,
                       check=True,
                       stdout=subprocess.PIPE,
                       input=f"GET {function_url}".encode(),
                       timeout=30)

    def plot(self, bin_names):
        encoded_bin_names = " ".join(f"{bin_name}.bin" for bin_name in bin_names)
        plot_cmd = f"vegeta plot --title 'Nuclio functions benchmarking' {encoded_bin_names}"
        self._logger.debug(f"Plotting command - {plot_cmd}")
        with open(self._workdir / "plot.html", "w") as outfile:
            subprocess.run(shlex.split(plot_cmd), cwd=self._workdir, check=True, stdout=outfile)

    def report(self, bin_name):
        report_cmd = f"vegeta report {bin_name}.bin"
        self._logger.debug(f"Reporting command - {report_cmd}")
        subprocess.run(shlex.split(report_cmd), cwd=self._workdir, check=True)


class Nuctl(object):
    def __init__(self, logger, exec_path, platform):
        self._logger = logger.getChild("nuctl")
        self._platform = platform
        self._exec_path = exec_path

    def deploy(self,
               handler,
               function_name,
               runtime,
               path,
               triggers):
        deploy_cmd = f"deploy {function_name}"
        if self._logger.getEffectiveLevel() < logging.INFO:
            deploy_cmd += " --verbose"
        deploy_cmd += f" --runtime {runtime}"
        deploy_cmd += f" --platform {self._platform}"
        deploy_cmd += f" --path {path}"
        deploy_cmd += f" --triggers {shlex.quote(json.dumps(triggers))}"
        deploy_cmd += f" --handler {handler}"
        deploy_cmd += " --http-trigger-service-type nodePort"
        deploy_cmd += " --no-pull"
        return self._exec(deploy_cmd)

    def get_function_port(self, function_name):
        cmd = f"get function --platform {self._platform} --output json {function_name}"
        get_function_response = self._exec(cmd, capture_output=True)
        return json.loads(get_function_response.stdout.decode())["status"]["httpPort"]

    def _exec(self, cmd, check=True, capture_output=False):
        return subprocess.run(shlex.split(f"{self._exec_path} {cmd}"), check=check, capture_output=capture_output)


class Function(object):
    def __init__(self,
                 nuctl_client,
                 logger,
                 workdir,
                 project_dir,
                 function_http_max_workers,
                 runtime_name):
        self._nuctl_client = nuctl_client
        self._runtime_name = runtime_name
        self._stripped_runtime_name = self._runtime_name.split(":")[0]
        self._workdir = workdir
        self._project_dir = project_dir
        self._function_http_max_workers = function_http_max_workers
        self._logger = logger.getChild(f"{self.name}")

    @property
    def name(self):
        formatted_runtime = self._runtime_name.replace(":", "-").replace(".", "")
        return f"{formatted_runtime}-bm"

    @property
    def runtime_name(self):
        return self._runtime_name

    def deploy(self):
        source_function_name = "empty"
        handler = Runtimes.runtime_to_bm_function_handler(source_function_name, self._runtime_name)
        function_path = self._resolve_function_path(source_function_name)
        self._logger.info(f"Deploying function from {function_path}")
        self._nuctl_client.deploy(handler,
                                  self.name,
                                  self._runtime_name,
                                  function_path,
                                  self._compile_function_http_trigger())

    def _compile_function_http_trigger(self):
        return {"benchmark": {"kind": "http", "maxWorkers": self._function_http_max_workers}}

    def get_function_port(self):
        return self._nuctl_client.get_function_port(self.name)

    def _resolve_function_path(self, source_function_name):
        runtime_example_dir = os.path.join(Constants.function_examples_dir, self._stripped_runtime_name)
        if not self._project_dir:
            base_remote_url = "https://raw.githubusercontent.com/nuclio/nuclio/master"
            filename = Runtimes.runtime_to_bm_function_handler_filename(source_function_name, self._runtime_name)
            return f"{base_remote_url}/{runtime_example_dir}/{source_function_name}/{filename}"
        return pathlib.Path(self._project_dir) / runtime_example_dir / source_function_name


def run(args):
    function_names = []
    os.makedirs(args.workdir, exist_ok=True)
    logger = logging.getLogger("benchmark")
    if args.verbose:
        logger.setLevel(logging.DEBUG)

    project_dir = _get_nuclio_project_dir()
    if not project_dir:
        logger.debug("Failed to determine git repository top level, assuming not in a nuclio project cloned dir")
    vegeta_client = Vegeta(logger, args.workdir)
    nuctl_client = Nuctl(logger, args.nuctl_path, args.nuctl_platform)
    functions = [
        Function(nuctl_client,
                 logger,
                 args.workdir,
                 project_dir,
                 args.function_http_max_workers,
                 runtime_name)
        for runtime_name in _populate_runtime_names(args.runtimes)
    ]
    for function in functions:
        logger.info(f"Benchmarking runtime function - {function.name}")
        if not args.skip_deploy:
            function.deploy()

        # resolve function url
        function_port = function.get_function_port()
        function_url = f"http://{args.function_url}:{function_port}"

        logger.info(f"Benchmarking function {function.name} @ {function_url}")
        vegeta_client.attack(function.name, function_url, args.function_http_max_workers)
        vegeta_client.report(function.name)
        function_names.append(function.name)

    logger.info(f"Plotting {function_names}")
    vegeta_client.plot(function_names)
    logger.info(f"Finished benchmarking.")


def _get_nuclio_project_dir():
    p = subprocess.run(shlex.split("git rev-parse --show-toplevel"),
                       check=False,
                       capture_output=True)
    if p.returncode == 0:
        return p.stdout.decode().strip()
    return ""


def _populate_runtime_names(runtimes):
    encoded_runtimes = runtimes
    if runtimes == "all":
        encoded_runtimes = ",".join(Runtimes.all())
    return encoded_runtimes.split(",")


def _parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--verbose",
                        help="Verbose output",
                        action="store_true")
    parser.add_argument("--skip-deploy",
                        help="Whether to deploy functions first (Default: False)",
                        action="store_true")
    parser.add_argument("--runtimes",
                        help=f"A comma delimited (,) list of Nuclio runtimes to benchmark (Default: all)",
                        default="all")
    parser.add_argument("--workdir",
                        help=f"Workdir to store benchmarking artifacts (Default: {Constants.default_workdir})",
                        default=Constants.default_workdir)

    # function
    parser.add_argument("--function-url",
                        help="Function url to use for HTTP requests (Default: localhost)",
                        default="localhost")
    parser.add_argument("--function-http-max-workers",
                        help=f"Number of function http trigger workers. (Default: # CPUs - {os.cpu_count()})",
                        default=os.cpu_count())

    # nuctl
    parser.add_argument("--nuctl-path",
                        help=f"Nuclio CLI ('nuctl') path (Default: nuctl from $PATH)",
                        default="nuctl")
    parser.add_argument("--nuctl-platform",
                        help="Platform to deploy and benchmark on. (Default: local)",
                        choices=["local", "kube"],
                        default="local")
    return parser.parse_args()


if __name__ == '__main__':
    parsed_args = _parse_args()
    run(parsed_args)
