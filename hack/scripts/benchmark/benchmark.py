# Copyright 2023 The Nuclio Authors.
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
import re
import shlex
import subprocess
import time

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


TODO: 
 - Structured logs
 - Add README
 - Update Benchmarking.md
"""


class Constants(object):
    function_examples_dir = os.path.join("hack", "examples")
    default_workdir = ".benchmarking"


class Runtimes(object):
    golang = "golang"
    java = "java"
    nodejs = "nodejs"
    dotnetcore = "dotnetcore"
    shell = "shell"
    ruby = "ruby"
    python37 = "python:3.7"
    python38 = "python:3.8"
    python39 = "python:3.9"
    python310 = "python:3.10"
    python311 = "python:3.11"

    # NOTE: python is just an alias to python3.9
    python = "python"

    @staticmethod
    def runtime_to_bm_function_handler(name, runtime):
        return {
            "empty": {
                Runtimes.python: "empty:handler",
                Runtimes.python37: "empty:handler",
                Runtimes.python38: "empty:handler",
                Runtimes.golang: "empty:Handler",
                Runtimes.java: "EmptyHandler",
                Runtimes.nodejs: "empty:handler",
                Runtimes.dotnetcore: "nuclio:empty",
                Runtimes.shell: "empty.sh:main",
                Runtimes.ruby: "empty:main",
            }
        }[name][runtime]

    @staticmethod
    def runtime_to_bm_function_handler_filename(name, runtime):
        return {
            "empty": {
                Runtimes.python: "empty.py",
                Runtimes.python37: "empty.py",
                Runtimes.python38: "empty.py",
                Runtimes.golang: "empty.go",
                Runtimes.java: "EmptyHandler.java",
                Runtimes.nodejs: "empty.js",
                Runtimes.dotnetcore: "empty.cs",
                Runtimes.shell: "empty.sh",
                Runtimes.ruby: "empty.rb",
            }
        }[name][runtime]

    @staticmethod
    def all():
        return [
            Runtimes.golang,
            Runtimes.python37,
            Runtimes.python38,
            Runtimes.java,
            Runtimes.nodejs,
            Runtimes.dotnetcore,
            Runtimes.shell,
            Runtimes.ruby,
        ]


class Vegeta(object):
    def __init__(self, logger, workdir):
        self._workdir = pathlib.Path(workdir)
        self._logger = logger.getChild("vegeta_client")

    def attack(self, function_name, function_url, concurrent_requests, body_size):
        body_size_filename = f"{function_name}_{body_size}"
        with open(self._workdir / body_size_filename, 'w') as f:
            f.truncate(body_size)

        vegeta_cmd = "vegeta attack" \
                     f" -name {function_name}" \
                     " -duration 10s" \
                     " -rate 0" \
                     f" -body {body_size_filename}" \
                     f" -connections {concurrent_requests}" \
                     f" -workers {concurrent_requests}" \
                     f" -max-workers {concurrent_requests}" \
                     f" -output {self._resolve_bin_name(function_name, body_size)}"
        self._logger.debug(f"Attacking command - {vegeta_cmd}")
        try:
            subprocess.run(shlex.split(vegeta_cmd),
                           cwd=self._workdir,
                           check=True,
                           stdout=subprocess.PIPE,
                           input=f"POST {function_url}".encode(),
                           timeout=30)
        finally:
            os.remove(path=self._workdir / body_size_filename)

    def plot(self, function_names, body_size):
        encoded_bin_names = " ".join(f"{self._resolve_bin_name(function_name, body_size)}"
                                     for function_name in function_names)
        plot_cmd = f"vegeta plot --title 'Nuclio functions benchmarking' {encoded_bin_names}"
        self._logger.debug(f"Plotting command - {plot_cmd}")
        with open(self._workdir / "plot.html", "w") as outfile:
            subprocess.run(shlex.split(plot_cmd), cwd=self._workdir, check=True, stdout=outfile)

    def report(self, function_name, body_size):
        report_cmd = f"vegeta report {self._resolve_bin_name(function_name, body_size)}"
        self._logger.debug(f"Reporting command - {report_cmd}")
        subprocess.run(shlex.split(report_cmd), cwd=self._workdir, check=True)

    def _resolve_bin_name(self, function_name, body_size):
        return f"{function_name}_{body_size}.bin"


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
                 runtime_name,
                 base_url):
        self._nuctl_client = nuctl_client
        self._runtime_name = runtime_name
        self._stripped_runtime_name = self._runtime_name.split(":")[0]
        self._workdir = workdir
        self._project_dir = project_dir
        self._function_http_max_workers = function_http_max_workers
        self._logger = logger.getChild(f"{self.name}")
        self._base_url = base_url

        # lazy
        self._port = None

    @property
    def name(self):
        formatted_runtime = self._runtime_name.replace(":", "-").replace(".", "")
        return f"{formatted_runtime}-bm"

    @property
    def runtime_name(self):
        return self._runtime_name

    @property
    def port(self):
        if not self._port:
            self._port = self._nuctl_client.get_function_port(self.name)
        return self._port

    @property
    def url(self):
        return f"http://{self._base_url}:{self.port}"

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

    def _resolve_function_path(self, source_function_name):
        runtime_example_dir = os.path.join(Constants.function_examples_dir, self._stripped_runtime_name)
        if not self._project_dir:
            base_remote_url = "https://raw.githubusercontent.com/nuclio/nuclio/master"
            filename = Runtimes.runtime_to_bm_function_handler_filename(source_function_name, self._runtime_name)
            return f"{base_remote_url}/{runtime_example_dir}/{source_function_name}/{filename}"
        return pathlib.Path(self._project_dir) / runtime_example_dir / source_function_name


class LoggingFormatter(logging.Formatter):
    class Colors(object):
        reset = "\u001b[0m"
        white = "\u001b[37m"
        cyan = "\u001b[36m"
        blue = "\u001b[34m"
        red = "\u001b[31m"
        green = "\u001b[32m"

    def format(self, record):

        def short_color(level):
            if level == logging.NOTSET:
                return "V", LoggingFormatter.Colors.white
            if level == logging.DEBUG:
                return "D", LoggingFormatter.Colors.cyan
            if level == logging.INFO:
                return "I", LoggingFormatter.Colors.blue
            if level == logging.WARNING:
                return "W", LoggingFormatter.Colors.red
            return "E", LoggingFormatter.Colors.red

        def format_level(level):
            short, color = short_color(level)
            return color + " (%s) " % short + LoggingFormatter.Colors.reset

        output = {
            "time": time.strftime("%y.%m.%d %H:%M:%S", time.localtime(record.created)),
            "name": record.name,
            "level": format_level(record.levelno),
            "message": record.getMessage(),
        }
        return f"{LoggingFormatter.Colors.white}%(time)s{LoggingFormatter.Colors.reset} " \
               f"{LoggingFormatter.Colors.green}%(name)29s{LoggingFormatter.Colors.reset} " \
               f"%(level)s " \
               f"{LoggingFormatter.Colors.white}%(message)s{LoggingFormatter.Colors.reset}" % output


def run(args):
    os.makedirs(args.workdir, exist_ok=True)
    logger = _get_logger(args.verbose)

    project_dir = _get_nuclio_project_dir()
    if not project_dir:
        logger.debug("Failed to determine Nuclio git repository top level, assuming not in a nuclio project cloned dir")

    body_sizes = args.body_sizes.split(",")
    parsed_body_sizes = list(map(_parse_body_size, body_sizes))
    vegeta_client = Vegeta(logger, args.workdir)
    nuctl_client = Nuctl(logger, args.nuctl_path, args.nuctl_platform)
    functions = [
        Function(nuctl_client,
                 logger,
                 args.workdir,
                 project_dir,
                 args.function_http_max_workers,
                 runtime_name,
                 args.function_url)
        for runtime_name in _populate_runtime_names(args.runtimes)
    ]
    function_names = [function.name for function in functions]
    encoded_function_names = ", ".join(function_names)

    if not args.skip_deploy:
        logger.info(f"Deploying functions - {encoded_function_names}")
        for function in functions:
            function.deploy()

    for function in functions:
        for index, parsed_body_size in enumerate(parsed_body_sizes):
            logger.info(f"Benchmarking function - {function.name} @ {function.url}, size: {body_sizes[index]}")
            vegeta_client.attack(function.name,
                                 function.url,
                                 args.function_http_max_workers,
                                 parsed_body_size)
            vegeta_client.report(function.name, parsed_body_size)
            logger.info(f"Sleeping for {args.sleep_after_attack_seconds} seconds")
            time.sleep(args.sleep_after_attack_seconds)
        logger.info(f"Successfully benchmarked function - {function.name}")

    logger.info(f"Plotting benchmarking results for functions: {encoded_function_names}")
    for parsed_body_size in parsed_body_sizes:
        vegeta_client.plot(function_names, parsed_body_size)
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


def _get_logger(verbose: bool):
    logger = logging.getLogger("benchmark")
    logger.setLevel(logging.INFO if not verbose else logging.NOTSET)
    sh = logging.StreamHandler()
    sh.setFormatter(LoggingFormatter())
    logger.addHandler(sh)
    return logger


def _parse_body_size(body_size_str):
    """Can be one of 1K or 1KB or 0M or 100MB"""
    units = {"B": 1, "KB": 2 ** 10, "MB": 2 ** 20}
    number, unit = re.search(r"(\d+)(\w+)", body_size_str).groups()
    number = int(number)
    return units[unit.upper()] * number


def _parse_args():
    parser = argparse.ArgumentParser(description="Benchmark",
                                     usage="Use \"%(prog)s --help\" for more information",
                                     formatter_class=argparse.RawTextHelpFormatter)
    all_runtimes = ",".join(_populate_runtime_names("all"))
    parser.add_argument("--verbose",
                        help="Verbose output. (Default: False)",
                        action="store_true")
    parser.add_argument("--skip-deploy",
                        help="Whether to deploy functions first. (Default: False)",
                        action="store_true")
    parser.add_argument("--runtimes",
                        help=f"A comma delimited (,) list of Nuclio runtimes to benchmark or \"all\" for all runtimes. "
                             f"(Default: {all_runtimes})",
                        default="all")
    parser.add_argument("--workdir",
                        help=f"Workdir to store benchmarking artifacts (Default: {Constants.default_workdir})",
                        default=Constants.default_workdir)
    parser.add_argument("--body-sizes",
                        help=f"A comma delimited (,) list of body sizes to use during benchmarking. "
                             f"Units are B, KB, MB."
                             f"(e.g.: example: 10K is 10*1024. Default: 0K - empty file.)",
                        default="0kb")
    parser.add_argument("--sleep-after-attack-seconds",
                        help="Sleep timeout after a single attack (Default: 3 seconds)",
                        type=int,
                        default=3)

    # function
    parser.add_argument("--function-url",
                        help="Function url to use for HTTP requests. (Default: localhost)",
                        default="localhost")
    parser.add_argument("--function-http-max-workers",
                        help=f"Number of function http trigger workers. (Default: # CPUs - {os.cpu_count()})",
                        default=os.cpu_count())

    # nuctl
    parser.add_argument("--nuctl-path",
                        help=f"Nuclio CLI ('nuctl') path. (Default: nuctl from $PATH)",
                        default="nuctl")
    parser.add_argument("--nuctl-platform",
                        help="Platform to deploy and benchmark on. (Default: local)",
                        choices=["local", "kube"],
                        default="local")
    return parser.parse_args()


if __name__ == "__main__":
    parsed_args = _parse_args()
    run(parsed_args)
