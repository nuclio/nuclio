/*
Package java implements a Java runtime
u c
To implement a handler you need to write a class the implements
io.nuclio.EventHandler and then build a jar to pass nuclio builder.

You can also pass a Java file to `nuctl build` and it'll generate a jar from it.
You can specify dependencies using (inline) build configuration

// @nuclio.configure
//
// function.yaml:
//   spec:
//     build:
//       dependencies:
//         - group: com.google.code.gson"
//           name: gson"
//           version: 2.8.2"
//         - group: com.google.guava"
//           name: guava"
//           version: 23.6-jre"


The default image is using OpenJDK 9

If you have dependecies in other packages, create a fat/uber Jar with all the
dependencies. We currently do not support maven/gradle/sbt/ant/... builds
*/
package java
