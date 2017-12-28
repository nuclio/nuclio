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

import java.io.FileInputStream;

class PipeReader {
    private String path;
    private FileInputStream in;

    public PipeReader(String path) {
        this.path = path;
    }

    public int read() throws Throwable {
        // We create file here since it'll block initially on named pipe
        if (this.in == null) {
            this.in = new FileInputStream(this.path);
        }

        while (true) {
            int ch = this.in.read();
            if (ch != -1) {
                return ch;
            }
            Thread.sleep(0);
        }
    }
}
