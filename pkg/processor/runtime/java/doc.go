/*
Copyright 2023 The Nuclio Authors.

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

/*
Package java implements a Java runtime

To implement a handler you need to write a class the implements
io.nuclio.EventHandler.

`nuctl build` does the following logic, depends on the value of `-p`:
- If it's a jar, use it
- If it's a directory and there's `handler.jar` there, use it
- If it's a directory with only single jar (including subdirectories), use it
- If there's a `build.gradle` file there - run `nuclioJar` task and use jar from `build` directory
- If there's no `build.gradle`, generate one, build annd use jar from `build` directory

Build will work also if the path passed is a single Java file.
You can specify dependencies using (inline in Java file or Jar) build configuration.

// @nuclio.configure
//
// function.yaml:
//   spec:
//     build:
//       dependencies:
//         - group: com.google.code.gson
//           name: gson
//           version: 2.8.9
//         - group: com.google.guava
//           name: guava
//           version: 23.6-jre

The default image is using OpenJDK 11
If you have dependencies in other packages, create a fat/uber Jar with all the dependencies.
We currently do not support maven/sbt/ant/... builds.

You can specify JVM options in the configuration as well, i.e.:

// @nuclio.configure
//
// function.yaml:
//   spec:
//     runtimeAttributes:
//       jvmOptions:
//         - -Xms128m
//         - -Xmx512m
*/
package java
