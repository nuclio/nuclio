name := "nuclio-java-runtime"
organization := "io.nuclio"
version := "1.0-SNAPSHOT"
description := "Nuclio Java Runtime"
crossPaths := false
autoScalaLibrary := false

libraryDependencies += "org.capnproto" % "runtime" % "0.1.2"
libraryDependencies += "io.nuclio" % "nuclio-sdk" % "1.0-SNAPSHOT" from "file://" + baseDirectory.value + "/nuclio-sdk-1.0-SNAPSHOT.jar"
