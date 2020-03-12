/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package io.nuclio.processor;

import io.nuclio.Context;
import io.nuclio.Logger;

import java.io.OutputStream;
import java.io.PrintWriter;

class WrapperContext implements Context {
    private WrapperLogger logger;
    private String workerID;

    public WrapperContext(OutputStream out, String workerID) {
        this.workerID = workerID;
        this.logger = new WrapperLogger(out, workerID);
    }

    @Override
    public Logger getLogger() {
        return logger;
    }
}
