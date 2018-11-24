# Windows buildlet images

Windows images are built by creating and configuring VMs in GCP then capturing the image to the GCP Project.

## Prerequisite: Access internal network

The scripts assume the internal GCP network is accessible. To do this, create a linux VM in the project and SSH into the machine or use a VPN like [sshuttle](https://github.com/apenwarr/sshuttle).

## Examples/Tools

### Build and test a single base image
Builds a buildlet from the BASE_IMAGE and sets it up with  and  An image is captured and then a new VM is created from that image and validated with [test_buildlet.bash](./test_buildlet.bash).

```bash
export PROJECT_ID=YOUR_GCP_PROJECT
export BASE_IMAGE=windows-server-2016-dc-core-v20171010
export IMAGE_PROJECT=windows-cloud

./build.bash
```

### Build all targets
```bash
PROJECT_ID=YOUR_GCP_PROJECT ./make.bash
```

### Build/test golang
```bash
instance_name=golang-buildlet-test
external_ip=$(gcloud compute instances describe golang-buildlet-test --project=${PROJECT_ID} --zone=${ZONE} --format="value(networkInterfaces[0].accessConfigs[0].natIP)")
./test_buildlet.bash $external_ip
```

### Troubleshoot via remote access
```bash
./rdp.bash <instance_name>
./ssh.bash <instance_name>
```
