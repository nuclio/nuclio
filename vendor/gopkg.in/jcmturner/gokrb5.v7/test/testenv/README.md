# gokrb5 Integration Test Environment

Below are some instructions for how to run the network services required for the gokrb5 integration tests.

There are two options to run these services.
1. As docker containers directly on your machine.
2. As a vagrant VM instances.

## Docker Containers
The Docker containers can be run with the commands you see in the ```before_install``` section of the 
[.travis.yml](https://github.com/jcmturner/gokrb5/blob/master/.travis.yml) file.
You will need to set the environment variable ```TEST_KDC_ADDR``` to "127.0.0.1" when running the integration tests.

## Vagant VM Instance
The Vagrant VM instance has been configured to work with VirtualBox.
VirtualBox will need to be configured with a host network. In the Vagrantfile configuration the CIDR range for that 
network is assumed to be 10.80.0.0/16 and the instance will run on 10.80.88.88. If this does not work for your environment you will need to update the 
Vagrantfile accordingly and when running the integration tests set the environment variable ```TEST_KDC_ADDR``` 
to the IP you have allocated for this Vagrant instance.

## Running the Integration Tests
Ensure you have set the environment variable ```TEST_KDC_ADDR``` accordingly.

To run the integration tests pass ```-tags=adintegration``` as an argument to the go test command. This will run all 
unit and integration tests (other than the integration tests for Active Directory).

