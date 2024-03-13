# Patch Nuclio images from your current code on a live Iguazio system (for debugging)


In order to deploy your current code (for debugging), you need the following:

* Install automation/requirements.txt 
~~~bash
pip install -r hack/scripts/patch-igz/requirements.txt
~~~
* Create a patch_env.yml based on patch_env_template.yml
* Have a docker registry you can push to (e.g. docker.io via account on docker.com)
* Make sure you are logged in into your registry (docker login --username user --password passwd), or optionally add username/password to config
* From nuclio root dir run, e.g.:
~~~bash
./hack/scripts/patch-igz/patch_remote.py --tag 1.12.14 dashboard controller
~~~

WARNING: This may not persist after system restart
