# Patch Nuclio images from your current code on a remote system (for debugging)


When developing Nuclio, it is sometimes useful to deploy your current code changes in the dashboard or the controller 
to a remote system for debugging. This can be done by automatically by following the instructions below.

To deploy your current code changes to a remote system, you need the following:

* Install hack/scripts/patch-remote/requirements.txt 
~~~bash
pip install -r hack/scripts/patch-remote/requirements.txt
~~~
* Create a `patch_env.yml` based on `patch_env_template.yml`, and fill in the required and optional fields
* Have a docker registry you can push to (e.g. docker.io via account on docker.com)
* Make sure you are logged in into your registry (docker login --username user --password user_password), or optionally add username/password to config
* The script can reach the remote system in two ways: over SSH, or directly via `kubectl` using a local kubeconfig - a more convenient and secure alternative, since it avoids ssh key/password management entirely. To use it, set `KUBECONFIG` in `patch_env.yml` to a local kubeconfig file pointed at the remote cluster; in that case `HOST_IP`/`SSH_USER` and the private key file are not required. If `KUBECONFIG` is not set (or the path doesn't exist), the script falls back to SSH.
* From nuclio root dir run:
~~~bash
make patch-remote-nuclio
~~~
* On first run (SSH mode only), you will need to manually input the remote system's password. This will create a new ssh key that will be used for future connections, so you will not need to input the password again.
* The script will automatically patch the remote system with your current code changes

For ease of use, you can add the specific make commands to patch only the dashboard or the controller:
~~~bash
make patch-remote-dashboard
make patch-remote-controller
~~~


:bangbang: **Disclaimers**:
* This script is intended for debugging purposes only. It is not recommended to use it in a production environment.
* This script will only work with a kubernetes Nuclio platform, and not with a Docker Nuclio platform.
* The changes will not persist after a service restart (e.g `helm upgrade`).