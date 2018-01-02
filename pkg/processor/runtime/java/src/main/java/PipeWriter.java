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

import java.io.FileOutputStream;

public class PipeWriter {
    private String path;
    private FileOutputStream out;

    public PipeWriter(String path) {
        this.path = path;
    }

    public void write(char c) throws Throwable {
        // We create file here since it'll block initially on named pipe
        if (this.out == null) {
            this.out = new FileOutputStream(this.path);
        }

        this.out.write(c);
        this.out.flush();
    }
}
