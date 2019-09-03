#!/bin/sh

# Copyright 2017 The Nuclio Authors.
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

#!/bin/sh

export EVENT_BODY=$(cat)

if [ "${EVENT_BODY}" == "return_body" ]; then
	echo ${EVENT_BODY}
elif [ "${EVENT_BODY}" == "return_env" ]; then
	echo ${ENV1}-${ENV2}
elif [ "${EVENT_BODY}" == "return_error" ]; then
	exit 1
elif [ "${EVENT_BODY}" == "return_arguments" ]; then
	echo $1-$2
fi
