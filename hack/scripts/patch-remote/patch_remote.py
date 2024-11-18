#!/usr/bin/env python3
# Copyright 2024 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

import io
import json
import logging
import os
import shlex
import subprocess
import time
import typing

import click
import coloredlogs
import yaml


class Helper:
    supported_targets = ["dashboard", "controller"]

    @staticmethod
    def run_on_all_targets(func):
        """
        Decorator to run the function for each target in supported_targets
        """

        def wrapper(*args, **kwargs):
            for target in Helper.supported_targets:
                func(*args, **kwargs, target=target)

        return wrapper


class NuclioPatcher:
    class Consts:
        log_level = logging.INFO
        fmt = "%(asctime)s %(levelname)s %(message)s"
        mandatory_fields = {"HOST_IP", "SSH_USER", "DOCKER_REGISTRY"}
        nuclio_version_annotation = "nuclio.io/version"
        deployment_policy_patch_dict = {
            "spec": {
                "template": {
                    "spec": {
                        "containers": [
                            {
                                "name": "nuclio-dashboard",
                                "imagePullPolicy": "Always",
                            },
                        ],
                    },
                },
            },
        }

    def __init__(self, conf_file, private_key, targets, verbose):
        self._config = yaml.safe_load(conf_file)
        self._validate_config()
        self._logger = self._init_logger(verbose or self._config.get("VERBOSE", False))
        self._node = self._config.get("HOST_IP", "")
        self._user = self._config.get("SSH_USER", "")
        self._targets = self._resolve_targets(targets)
        self._tag = self._config.get("NUCLIO_TAG", "")
        self._arch = self._config.get("NUCLIO_ARCH", "amd64")
        self._namespace = self._config.get("NAMESPACE", "nuclio")
        self._private_key = private_key

    def patch_nuclio(self):
        self._logger.info(
            f"Patching Nuclio targets to remote system: {', '.join(self._targets)}"
        )

        # prepare
        self._resolve_tag()
        self._docker_login_if_configured()

        # build and push images
        self._build_and_push_target_images()

        # patch the deployment with the new image
        try:
            self._replace_deploy_policy()
            self._replace_deployment_images()
            self._restart_deployment()
            self._wait_deployment_ready()
        finally:
            self._log_pod_names()

        self._logger.info(
            "Successfully patched branch successfully to remote (Note this may not survive system restarts)"
        )

    def _validate_config(self):
        missing_fields = self.Consts.mandatory_fields - set(self._config.keys())
        if len(missing_fields) > 0:
            raise RuntimeError(f"Mandatory options not defined: {missing_fields}")

        registry_username = self._config.get("REGISTRY_USERNAME")
        registry_password = self._config.get("REGISTRY_PASSWORD")
        if registry_username is not None and registry_password is None:
            raise RuntimeError(
                "REGISTRY_USERNAME defined, yet REGISTRY_PASSWORD is not defined"
            )

    def _init_logger(self, verbose):
        logging.basicConfig(level=self.Consts.log_level)
        logger = logging.getLogger("nuclio-patch")
        coloredlogs.install(
            level=self.Consts.log_level, logger=logger, fmt=self.Consts.fmt
        )
        if verbose:
            coloredlogs.set_level(logging.DEBUG)
        return logger

    def _resolve_targets(self, _targets):
        targets = (
            _targets.split(",")
            if _targets
            else self._config.get("PATCH_TARGETS", ["dashboard"])
        )
        if len(targets) == 0:
            raise RuntimeError("No targets to patch")
        for target in targets:
            if target not in Helper.supported_targets:
                raise RuntimeError(f"Invalid target: {target}")
        return targets

    def _resolve_tag(self):
        if self._tag:
            self._tag = str(self._tag).strip()
            return

        # resolve the current version running in the remote system by examining the deployment of one of the targets
        self._logger.debug("Resolving current version from remote system")
        deployment_name = f"nuclio-{self._targets[0]}"
        version = self._exec_remote(
            [
                "kubectl",
                "--namespace",
                self._namespace,
                "get",
                "deployment",
                deployment_name,
                "-o",
                "yaml",
                "|",
                # will output something like: nuclio.io/version: 1.12.0-amd64
                "grep",
                self.Consts.nuclio_version_annotation,
                "|",
                # will output something like: 1.12.0-amd64
                "awk",
                "'{print $2}'",
                "|",
                # will output something like: 1.12.0
                "awk",
                "-F",
                "'-'",
                "'{print $1}'",
            ],
        )
        self._tag = version.strip()

    @staticmethod
    def _get_image_tag(tag) -> str:
        return f"{tag}"

    def _docker_login_if_configured(self):
        registry_username = self._config.get("REGISTRY_USERNAME")
        registry_password = self._config.get("REGISTRY_PASSWORD")
        if registry_username is not None:
            self._exec_local(
                [
                    "docker",
                    "login",
                    "--username",
                    registry_username,
                    "--password",
                    registry_password,
                ],
                live=True,
            )

    def _build_and_push_target_images(self):
        self._logger.info(
            f"Building nuclio docker images for targets: {self._targets}, tag: {self._tag}"
        )
        image_rules = " ".join(self._targets)
        env = {
            "DOCKER_BUILDKIT": "1",
            "NUCLIO_DOCKER_REPO": self._config["DOCKER_REGISTRY"],
            "NUCLIO_LABEL": self._tag,
            "NUCLIO_ARCH": self._arch,
            "DOCKER_IMAGES_RULES": image_rules,
        }
        cmd = ["make", "docker-images", "push-docker-images"]
        self._exec_local(cmd, live=True, env=env)

    @Helper.run_on_all_targets
    def _replace_deploy_policy(self, target="dashboard"):
        if target not in self._targets:
            return

        deployment_name = f"nuclio-{target}"
        patch_string = self._generate_patch_string(deployment_name)

        self._logger.info(f"Patching {deployment_name} deployment")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                self._namespace,
                "patch",
                "deployment",
                deployment_name,
                "-p",
                f"{patch_string}",
            ]
        )

    def _generate_patch_string(self, container_name):
        patch_dict = self.Consts.deployment_policy_patch_dict.copy()
        patch_dict["spec"]["template"]["spec"]["containers"][0]["name"] = container_name
        return shlex.quote(json.dumps(patch_dict))

    @Helper.run_on_all_targets
    def _replace_deployment_images(self, target="dashboard"):
        if target not in self._targets:
            return

        container = f"nuclio-{target}"
        image = self._get_target_image_name(target, self._tag)
        if self._config.get("OVERWRITE_IMAGE_REGISTRY"):
            image = image.replace(
                self._config["DOCKER_REGISTRY"],
                self._config["OVERWRITE_IMAGE_REGISTRY"],
            )
        self._logger.info(f"Replacing {container} in {target} deployment")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                self._namespace,
                "set",
                "image",
                f"deployment/nuclio-{target}",
                f"{container}={image}",
            ]
        )

    def _get_target_image_name(self, target, tag):
        return f"{self._config['DOCKER_REGISTRY']}/{target}:{tag}-{self._arch}"

    @Helper.run_on_all_targets
    def _restart_deployment(self, target="dashboard"):
        if target not in self._targets:
            return

        self._logger.info(f"Restarting {target} deployment")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                self._namespace,
                "rollout",
                "restart",
                "deployment",
                f"nuclio-{target}",
            ]
        )

    @Helper.run_on_all_targets
    def _wait_deployment_ready(self, target="dashboard"):
        if target not in self._targets:
            return

        self._logger.info(f"Waiting for {target} deployment to become ready")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                self._namespace,
                "rollout",
                "status",
                "deployment",
                f"nuclio-{target}",
                "--timeout=240s",
            ],
            live=True,
        )

        self._wait_for_pod_ready(target)

    def _wait_for_pod_ready(self, target):
        """
        Waits for a pod to become ready.
        Since the deployments' strategy is RollingUpdate, using 'kubectl wait --for condition=Ready' sometimes times
        out because it waits for the terminating pod to be ready. Instead, we will poll the pod status until it's ready.
        """

        self._logger.info(f"Waiting for {target} pod to become ready")
        while True:
            out = self._exec_remote(
                [
                    "kubectl",
                    "--namespace",
                    self._namespace,
                    "get",
                    "pod",
                    "-l",
                    f"nuclio.io/app={target}",
                    "-o",
                    "jsonpath={.items[*].status.phase}",
                ]
            )
            if out.strip() == "Running":
                break
            time.sleep(5)

    def _log_pod_names(self):
        out = self._exec_remote(
            [
                "kubectl",
                "--namespace",
                self._namespace,
                "get",
                "pods",
            ],
        )
        for line in out.splitlines():
            for target in self._targets:
                if f"nuclio-{target}" in line:
                    self._logger.info(line)
                    break

    @staticmethod
    def _execute_local_proc_interactive(cmd, env=None):
        env = os.environ | (env or {})
        proc = subprocess.Popen(
            cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, env=env
        )
        yield from proc.stdout
        proc.stdout.close()
        ret_code = proc.wait()
        if ret_code:
            raise subprocess.CalledProcessError(ret_code, cmd)

    def _exec_local(
        self, cmd: list[str], live: bool = False, env: typing.Optional[dict] = None
    ) -> str:
        self._logger.debug("Exec local: %s", " ".join(cmd))
        buf = io.StringIO()
        for line in self._execute_local_proc_interactive(cmd, env):
            buf.write(line)
            if live:
                print(line, end="")
        output = buf.getvalue()
        return output

    def _exec_remote(self, cmd: list[str], live=False) -> str:
        # run the command on the remote machine using ssh in a subprocess
        cmd_str = " ".join(cmd)
        self._logger.debug("Executing remote command: %s", cmd_str)
        ssh_cmd = [
            "/usr/bin/ssh",
            "-i",
            self._private_key,
            f"{self._user}@{self._node}",
        ]
        ssh_cmd.extend(cmd)
        proc = subprocess.Popen(
            ssh_cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        stdout = ""
        if live:
            for line in proc.stdout:
                stdout += line
                print(line, end="")
        else:
            stdout = proc.stdout.read()
        stderr = proc.stderr.read()
        proc.wait()
        if proc.returncode:
            raise RuntimeError(
                f"Command '{cmd_str}' finished with failure ({proc.returncode})\n{stderr}"
            )
        return stdout


@click.command(help="nuclio image deployer to remote system")
@click.option(
    "-c",
    "--config",
    help="Config file",
    default="hack/scripts/patch-remote/patch_env.yml",
    type=click.File(mode="r"),
    show_default=True,
)
@click.option(
    "-p",
    "--private-key-file",
    help="Private ssh key file",
    type=str,
    required=True,
)
@click.option(
    "-t",
    "--targets",
    type=str,
    help="A comma delimited list of targets to patch, to override the targets in the config",
    required=False,
)
@click.option("-v", "--verbose", is_flag=True, help="Print what we are doing")
def main(config, private_key_file, targets, verbose):
    NuclioPatcher(config, private_key_file, targets, verbose).patch_nuclio()


if __name__ == "__main__":
    main()
