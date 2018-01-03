name := "nuclio-java-runtime"
organization := "io.nuclio"
version := "1.0-SNAPSHOT"
description := "Nuclio Java Runtime"
crossPaths := false
autoScalaLibrary := false

libraryDependencies += "commons-cli" % "commons-cli" % "1.4"
libraryDependencies += "com.fasterxml.jackson.core" % "jackson-databind" % "2.9.0"
libraryDependencies += "com.fasterxml.jackson.core" % "jackson-core" % "2.9.0"
libraryDependencies += "com.fasterxml.jackson.core" % "jackson-annotations" % "2.9.0"
libraryDependencies += "io.nuclio" % "nuclio-sdk" % "1.0-SNAPSHOT" from "file://" +
    baseDirectory.value + "/nuclio-sdk-1.0-SNAPSHOT.jar"

assemblyJarName in assembly := "nuclio-java-wrapper.jar"
