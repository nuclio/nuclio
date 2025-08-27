#!/usr/bin/env python3
# Copyright 2025 The Nuclio Authors.
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
import re
import sys
from abc import abstractmethod

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry
from packaging.version import Version
from pathlib import Path
import argparse
import time


class Dependency:
    """Base class for a dependency/version checker."""

    def __init__(self, name, regex_pattern):
        self.name = name
        # Compile the regex pattern to extract version information from the Makefile
        self.regex_pattern = re.compile(regex_pattern)

    def get_makefile_versions(self, makefile_content):
        """Extract all matching versions from Makefile content using regex."""
        # Use regex to find all matches in the Makefile content
        matches = self.regex_pattern.findall(makefile_content)
        if not matches:
            raise ValueError(f"Version for {self.name} not found in Makefile")
        # Ensure only the version part (group 2) is returned
        return [match if isinstance(match, str) else match[1] for match in matches]

    def get_latest_version(self):
        """Override in subclass."""
        raise NotImplementedError

    def check_versions(self, makefile_content):
        """Compare all Makefile versions with the latest version."""
        # Extract all versions from the Makefile
        makefile_versions = self.get_makefile_versions(makefile_content)
        # Check if there are inconsistent versions in the Makefile
        if len(set(makefile_versions)) > 1:
            print(f"❌ Inconsistent versions for {self.name} in Makefile: {', '.join(makefile_versions)}")
            return False

        # Fetch the latest version from the source (e.g., GitHub)
        latest_version = self.get_latest_version()
        print(f"🔍 Latest {self.name} version: {latest_version}")
        print(f"📦 Makefile version:          {makefile_versions[0]}")
        # Compare the Makefile version with the latest version
        if makefile_versions[0] == latest_version:
            print(f"✅ {self.name} version is up-to-date\n")
            return True
        else:
            print(f"❌ {self.name} version is outdated or inconsistent\n")
            return False

    def bump_to_latest(self, makefile_content):
        """
        Replace all occurrences of the version in the Makefile with the latest version.

        Match groups:
        - group(1): The part of the line before the version (e.g., "NUCLIO_DOCKER_CLIENT_VERSION ?= ").
        - group(2): The current version (e.g., "28.3.3").
        - group(3): The part of the line after the version (if applicable, e.g., "-alpine").
        """
        latest_version = self.get_latest_version()
        updated_content = makefile_content

        # Iterate over all matches in the Makefile
        for match in self.regex_pattern.finditer(makefile_content):
            old_version = match.group(2)  # Extract the current version
            if old_version == latest_version:
                # Skip if the version is already up-to-date
                print(f"✅ {self.name} is already up-to-date: {old_version}")
                continue

            new_line = f"{match.group(1)}{latest_version}{match.group(3) if len(match.groups()) > 2 else ''}"
            old_line = f"{match.group(1)}{old_version}{match.group(3) if len(match.groups()) > 2 else ''}"

            # Log the change
            print(f"🔄 Updating {self.name}:")
            print(f"   Old: {old_line}")
            print(f"   New: {new_line}")

            # Replace the old version with the latest version while preserving the surrounding text
            updated_content = updated_content.replace(old_line, new_line)

        return updated_content


def create_session_with_retries(retries=3, backoff_factor=0.3, status_forcelist=(500, 502, 503, 504)):
    """Create a requests session with retry logic."""
    session = requests.Session()
    retry = Retry(
        total=retries,
        read=retries,
        connect=retries,
        backoff_factor=backoff_factor,
        status_forcelist=status_forcelist,
    )
    adapter = HTTPAdapter(max_retries=retry)
    session.mount("http://", adapter)
    session.mount("https://", adapter)
    return session


class GitHubDependency(Dependency):
    """Dependency checker for GitHub-hosted projects."""

    def __init__(self, name, repo, tag_regex, makefile_regex):
        super().__init__(name, makefile_regex)
        self.repo = repo
        # Compile the regex pattern to extract version tags from GitHub
        self.tag_regex = re.compile(tag_regex)
        # Create a session with retry logic for API requests
        self.session = create_session_with_retries()

    @abstractmethod
    def get_latest_version(self):
        """
        Fetch the latest version from GitHub tags with retry and wait.

        The GitHub API returns a list of tags, and we use the tag_regex to extract valid version tags.
        """
        url = f"https://api.github.com/repos/{self.repo}/tags"
        retries = 3
        for attempt in range(retries):
            try:
                # Make a GET request to fetch tags
                response = self.session.get(url, timeout=10)
                response.raise_for_status()
                tags = response.json()
                # Extract valid versions using the tag_regex
                versions = [
                    match.group(1)
                    for tag in tags
                    if (match := self.tag_regex.match(tag["name"]))
                ]
                if not versions:
                    raise RuntimeError(f"No valid tags found for {self.name} in {self.repo}")
                # Return the highest version
                return str(max(Version(v) for v in versions))
            except requests.RequestException as e:
                if attempt < retries - 1:
                    print(f"Warning: Failed to fetch tags for {self.name} (attempt {attempt + 1}/{retries}). Retrying in 5 seconds...")
                    time.sleep(5)
                else:
                    raise RuntimeError(f"Failed to fetch tags for {self.name} after {retries} attempts: {e}")

            except Exception as e:
                raise RuntimeError(f"Error processing tags for {self.name}: {e}")



def load_makefile_content(makefile_path):
    """Load the content of the Makefile."""
    try:
        with open(makefile_path, "r") as f:
            return f.read()
    except FileNotFoundError:
        raise FileNotFoundError(f"Makefile not found at {makefile_path}")


def save_makefile_content(makefile_path, content):
    """Save the updated content back to the Makefile."""
    with open(makefile_path, "w") as f:
        f.write(content)


def main(makefile_path=None, bump_to_latest=False):
    """Main function to check or bump dependency versions."""
    makefile_content = load_makefile_content(
        Path(makefile_path) if makefile_path else Path(__file__).resolve().parent.parent.parent.parent / "Makefile"
    )

    dependencies = [
        GitHubDependency(
            name="docker-cli",
            repo="docker/cli",
            tag_regex=r"^v?(\d+\.\d+\.\d+)$",  # Match version tags like "v28.3.3" or "28.3.3"
            makefile_regex=r"(NUCLIO_DOCKER_CLIENT_VERSION\s*\?=\s*)([\d.]+)",  # Match Makefile lines for Docker CLI
        ),
        GitHubDependency(
            name="nginx",
            repo="nginx/nginx",
            tag_regex=r"^release-(\d+\.\d+)\.\d+$",  # Match version tags like "release-1.29.0"
            makefile_regex=r"(NUCLIO_DOCKER_DASHBOARD_NGINX_BASE_IMAGE.*nginx:)([\d.]+)(-.*)",  # Match Makefile lines for NGINX
        ),
    ]

    if bump_to_latest:
        for dep in dependencies:
            makefile_content = dep.bump_to_latest(makefile_content)
        save_makefile_content(makefile_path, makefile_content)
        print(f"✅ Makefile updated to the latest versions.")
    else:
        all_checks_passed = True
        for dep in dependencies:
            if not dep.check_versions(makefile_content):
                all_checks_passed = False

        if not all_checks_passed:
            print("❌ One or more dependencies are outdated or inconsistent.")
            sys.exit(1)
        else:
            print("✅ All dependencies are up-to-date.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Check or bump dependency versions.")
    parser.add_argument(
        "--makefile-path",
        type=str,
        default=None,
        help="Path to the Makefile (default: project root Makefile)",
    )
    parser.add_argument(
        "--bump-to-the-latest",
        action="store_true",
        help="Bump all dependencies in the Makefile to their latest versions",
    )
    args = parser.parse_args()

    try:
        main(args.makefile_path, args.bump_to_the_latest)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)
