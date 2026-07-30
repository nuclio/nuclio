#!/usr/bin/env python3
# Copyright 2026 The Nuclio Authors.
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
"""
Merges docker-config-json secrets and cloud registry credential tokens into one authfile.

Runs as a build-job init container, stdlib only (no third-party dependencies).
"""

import argparse
import base64
import json
import os
import sys


def warn(message):
    print("WARNING: " + message, file=sys.stderr)


def files_with_suffix(directory, suffix):
    try:
        names = sorted(os.listdir(directory))
    except OSError:
        return []
    return [os.path.join(directory, name) for name in names if name.endswith(suffix)]


def read_credential_file(path):
    try:
        with open(path, "r") as f:
            contents = f.read()
    except OSError as exc:
        warn("failed to read registry credential token {}: {}".format(path, exc))
        return None, None

    lines = contents.split("\n", 2)
    if len(lines) < 3:
        warn("malformed registry credential token {}: expected 3 lines, got {}".format(path, len(lines)))
        return None, None

    host = lines[0].strip()
    username = lines[1].strip()
    token = lines[2].strip()
    if not host or not username or not token:
        warn("malformed registry credential token {}: empty host/username/token".format(path))
        return None, None

    auth = base64.b64encode((username + ":" + token).encode("utf-8")).decode("ascii")
    return host, {"auth": auth}


def merge_auth_files(secrets_dir, tokens_dir, target_path):
    auths = {}
    rest = {}

    secret_paths = files_with_suffix(secrets_dir, ".json")
    print("found {} registry auth source(s) in {}".format(len(secret_paths), secrets_dir))

    for path in secret_paths:
        try:
            with open(path, "r") as f:
                doc = json.load(f)
        except (OSError, ValueError) as exc:
            warn("failed to read/parse registry auth source {}: {}".format(path, exc))
            continue

        host_auths = doc.get("auths")
        if isinstance(host_auths, dict):
            hosts = []
            for host, entry in host_auths.items():
                auths[host] = entry
                hosts.append(host)
            print("merged registry auth source {}: hosts={}".format(path, hosts))
        else:
            print("merged registry auth source {}: no hosts".format(path))

        for key, value in doc.items():
            if key != "auths":
                rest[key] = value

    token_paths = files_with_suffix(tokens_dir, ".token") if tokens_dir else []
    print("found {} cloud registry credential token(s) in {}".format(len(token_paths), tokens_dir))

    for path in token_paths:
        host, entry = read_credential_file(path)
        if host is not None:
            auths[host] = entry
            print("applied cloud registry credential token {} for host {}".format(path, host))

    rest["auths"] = auths
    merged = json.dumps(rest)

    target_dir = os.path.dirname(target_path)
    if target_dir:
        os.makedirs(target_dir, exist_ok=True)
    with open(target_path, "w") as f:
        f.write(merged)
    print("wrote merged authfile to {} with {} host(s)".format(target_path, len(auths)))

    for path in token_paths:
        try:
            os.remove(path)
        except OSError as exc:
            warn("failed to remove consumed registry credential token {}: {}".format(path, exc))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--secrets", default="", help="Directory of numbered docker-config-json sources")
    parser.add_argument("--cloud-tokens", default="", help="Directory of numbered cloud registry credential token files")
    parser.add_argument("--target", required=True, help="Authfile to write")
    args = parser.parse_args()

    merge_auth_files(args.secrets, args.cloud_tokens, args.target)


if __name__ == "__main__":
    main()
