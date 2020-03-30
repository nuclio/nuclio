import subprocess
import os
import sys
import requests


def build_functions():
	handlers = [
		"nothing",
		"with_modules",
		"with_vendor",
	]
	offline_modes = [
		# "true",
		"false",
	]
	cwd = os.path.dirname(os.path.abspath(os.path.join(__file__, os.pardir)))
	functions = []
	for handler in handlers:
		for offline_mode in offline_modes:
			if handler == "with_modules" and offline_mode == "true":

				# it is not possible to accept go modules build when offline mode is enable
				continue

			network = "host" if offline_mode == "false" else "none"

			function_name = f"nuclio-go-{handler}-{offline_mode}"
			build_command = [
				"docker", "build",
				"--file", "./go1.13tmp/Dockerfile",
				"--tag", function_name,
				"--network", network,
				"--build-arg", f"HANDLER={handler}",
				"--build-arg", f"NUCLIO_BUILD_OFFLINE={offline_mode}",
				"."
				]
			print(f"Building function {function_name}")
			process = subprocess.run(build_command, cwd=cwd)
			process.check_returncode()
			functions.append(function_name)
	return functions

def run_functions(functions):
	base_port = 8080
	running_functions = []
	for idx, function_name in enumerate(functions):
		port = base_port + idx
		run_command = [
			"docker", "run",
			"-d",
			"-p", f"{port}:8080",
			function_name
		]
		print(f"Running function {function_name}")
		running_functions.append((function_name, port))
		subprocess.run(run_command).check_returncode()
	return running_functions


def run():
	functions = build_functions()
	running_functions = run_functions(functions)
	
	for idx, (function_name, port) in enumerate(running_functions):
		http_command = [
			"http", f"localhost:{port}",
		]
		print(f"Sending HTTP request to function {function_name}")
		process = subprocess.run(http_command, stdout=sys.stdout, stderr=sys.stderr)
		process.check_returncode()

if __name__ == "__main__":
	run()
