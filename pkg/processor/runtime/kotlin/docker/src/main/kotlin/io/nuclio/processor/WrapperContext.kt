/*
Copyright 2018 The Nuclio Authors.

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

package io.nuclio.processor

import io.nuclio.Context
import io.nuclio.Logger
import java.io.OutputStream

internal class WrapperContext(out: OutputStream) : Context {
    private val logger: WrapperLogger = WrapperLogger(out)

    override fun getLogger(): Logger {
        return logger
    }
}
