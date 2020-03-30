#!/usr/bin/env sh


# exit on failure
set -o errexit

# show command before execute
set -o xtrace

if [ -d "vendor" ]; then

	# merge processor vendor modules
	cp -R /processor_vendor/* vendor/
	cp /processor_go.mod go.mod
	cp /processor_go.sum go.sum

elif [ -f "go.mod" ]; then

	if [ "${NUCLIO_BUILD_OFFLINE}" == "true" ]; then

		# error
		echo "Impossible to accept go.mod when building offline"
		exit 1
	else

		# add any missing modules & remove unused modules
		go mod tidy

		# make a vendor
		go mod vendor
	fi
else

	# use processor vendor to build function
	mv /processor_vendor vendor
	cp /processor_go.mod go.mod
	cp /processor_go.sum go.sum

	if [ "${NUCLIO_BUILD_OFFLINE}" == "false" ]; then

		# remove unused modules
		go mod tidy

		# recreate vendor based on processor
		go mod vendor
	fi
fi

# Removing breadcrums
rm -rf /processor_vendor /processor_go.mod /processor_go.sum
