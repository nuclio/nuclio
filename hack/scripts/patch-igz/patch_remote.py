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
import typing

import click
import coloredlogs
import paramiko
import yaml

log_level = logging.INFO
fmt = "%(asctime)s %(levelname)s %(message)s"
logging.basicConfig(level=log_level)
logger = logging.getLogger("nuclio-patch")
coloredlogs.install(level=log_level, logger=logger, fmt=fmt)
supported_targets = ["dashboard", "controller"]


def target_wrapper(func):
    """
    Decorator to run the function for each target in supported_targets
    """
    def wrapper(*args, **kwargs):
        for target in supported_targets:
            func(*args, **kwargs, target=target)

    return wrapper


class NuclioPatcher:
    class Consts:
        mandatory_fields = {"DATA_NODES", "SSH_USER", "SSH_PASSWORD", "DOCKER_REGISTRY"}
        patch_dict = {
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

    def __init__(self, conf_file, tag, arch, targets):
        self._config = yaml.safe_load(conf_file)
        self._tag = tag
        self._arch = arch
        self._targets = targets
        self._validate_config()
        self._validate_targets()

    def patch_nuclio(self):
        nodes = self._config["DATA_NODES"]
        if not isinstance(nodes, list):
            nodes = [nodes]

        version = self._get_current_version()
        image_tag = self._get_image_tag(version)
        self._docker_login_if_configured()

        self._build_and_push_target_images(image_tag)

        node = nodes[0]
        self._connect_to_node(node)
        try:
            self._replace_deploy_policy()
            self._replace_deployment_images()
            self._restart_deployment()
            self._wait_deployment_ready()
        finally:
            self._log_pod_names()
            self._disconnect_from_node()

        logger.info(
            "Successfully patched branch successfully to remote (Note this may not survive system restarts)"
        )

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

    def _validate_targets(self):
        # target is a tuple, that can contain dashboard, controller or both
        if not isinstance(self._targets, tuple):
            self._targets = (self._targets,)

        for target in self._targets:
            if target not in supported_targets:
                raise RuntimeError(f"Invalid target: {target}")

    def _get_current_version(self) -> str:
        if "unstable" in self._tag:
            return "unstable"
        return self._tag

    def _build_and_push_target_images(self, image_tag):
        logger.info(f"Building nuclio docker images for: {self._targets}")
        image_rules = " ".join(self._targets)
        env = {
            "DOCKER_BUILDKIT": "1",
            "NUCLIO_DOCKER_REPO": self._config["DOCKER_REGISTRY"],
            "NUCLIO_LABEL": image_tag,
            "NUCLIO_ARCH": self._arch,
            "DOCKER_IMAGES_RULES": image_rules,
        }
        cmd = ["make", "docker-images", "push-docker-images"]
        self._exec_local(cmd, live=True, env=env)

    def _get_target_image_name(self, target, tag):
        return f"{self._config['DOCKER_REGISTRY']}/{target}:{tag}-{self._arch}"

    def _connect_to_node(self, node):
        logger.debug(f"Connecting to {node}")

        self._ssh_client = paramiko.SSHClient()
        self._ssh_client.set_missing_host_key_policy(paramiko.WarningPolicy)
        self._ssh_client.connect(
            node,
            username=self._config["SSH_USER"],
            password=self._config["SSH_PASSWORD"],
        )

    def _disconnect_from_node(self):
        self._ssh_client.close()

    # TODO: Check if this is needed at all
    def _push_docker_images(self, built_images):
        logger.info(f"Pushing docker images: {built_images}")

        for image in built_images:
            self._exec_local(
                [
                    "docker",
                    "push",
                    image,
                ],
                live=True,
            )

    @target_wrapper
    def _replace_deploy_policy(self, target="dashboard"):
        if target not in self._targets:
            return

        deployment_name = f"nuclio-{target}"
        patch_string = self._generate_patch_string(deployment_name)

        logger.info(f"Patching {deployment_name} deployment")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                "default-tenant",
                "patch",
                "deployment",
                deployment_name,
                "-p",
                f"{patch_string}",
            ]
        )

    @target_wrapper
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
        logger.info(f"Replacing {container} in {target} deployment")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                "default-tenant",
                "set",
                "image",
                f"deployment/nuclio-{target}",
                f"{container}={image}",
            ]
        )

    @target_wrapper
    def _restart_deployment(self, target="dashboard"):
        if target not in self._targets:
            return

        logger.info(f"Restarting {target} deployment")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                "default-tenant",
                "rollout",
                "restart",
                "deployment",
                f"nuclio-{target}",
            ]
        )

    @target_wrapper
    def _wait_deployment_ready(self, target="dashboard"):
        if target not in self._targets:
            return

        logger.info(f"Waiting for {target} deployment to become ready")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                "default-tenant",
                "rollout",
                "status",
                "deployment",
                f"nuclio-{target}",
                "--timeout=120s",
            ],
            live=True,
        )

        logger.info(f"Waiting for {target} pod to become ready")
        self._exec_remote(
            [
                "kubectl",
                "-n",
                "default-tenant",
                "wait",
                "pods",
                "-l",
                f"nuclio.io/app={target}",
                "--for",
                "condition=Ready",
                "--timeout=240s",
            ],
            live=True,
        )

    @staticmethod
    def _get_image_tag(tag) -> str:
        return f"{tag}"

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
        logger.debug("Exec local: %s", " ".join(cmd))
        buf = io.StringIO()
        for line in self._execute_local_proc_interactive(cmd, env):
            buf.write(line)
            if live:
                print(line, end="")
        output = buf.getvalue()
        return output

    def _exec_remote(self, cmd: list[str], live=False) -> str:
        cmd_str = shlex.join(cmd)
        logger.debug("Executing remote command: %s", cmd_str)
        stdin_stream, stdout_stream, stderr_stream = self._ssh_client.exec_command(
            cmd_str
        )

        stdout = ""
        if live:
            while True:
                line = stdout_stream.readline()
                stdout += line
                if not line:
                    break
                print(line, end="")
        else:
            stdout = stdout_stream.read().decode("utf8")

        stderr = stderr_stream.read().decode("utf8")

        exit_status = stdout_stream.channel.recv_exit_status()

        if exit_status:
            raise RuntimeError(
                f"Command '{cmd_str}' finished with failure ({exit_status})\n{stderr}"
            )

        return stdout

    def _generate_patch_string(self, container_name):
        patch_dict = self.Consts.patch_dict.copy()
        patch_dict["spec"]["template"]["spec"]["containers"][0]["name"] = container_name
        return json.dumps(patch_dict)

    def _log_pod_names(self):
        out = self._exec_remote(
            [
                "kubectl",
                "-n",
                "default-tenant",
                "get",
                "pods",
            ],
        )
        for line in out.splitlines():
            for target in self._targets:
                if f"nuclio-{target}" in line:
                    logger.info(line)
                    break


@click.command(help="nuclio image deployer to remote system")
@click.option("-v", "--verbose", is_flag=True, help="Print what we are doing")
@click.option(
    "-c",
    "--config",
    help="Config file",
    default="hack/scripts/patch-igz/patch_env.yml",
    type=click.File(mode="r"),
    show_default=True,
)
@click.option(
    "-t",
    "--tag",
    default="0.0.0+unstable",
    help="Tag to use for the API. Defaults to unstable (latest and greatest)",
)
@click.option(
    "-a",
    "--arch",
    default="amd64",
    help="Architecture to build for",
)
@click.argument(
    "targets",
    nargs=-1,
    type=str,
    required=True,
)
def main(verbose, config, tag, arch, targets):
    if verbose:
        coloredlogs.set_level(logging.DEBUG)

    NuclioPatcher(config, tag, arch, targets).patch_nuclio()


if __name__ == "__main__":
    main()
