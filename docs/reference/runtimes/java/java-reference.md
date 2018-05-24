# Java reference

This document describes Java-specific build and deploy configurations.

## Function and handler

```java
import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

public class EmptyHandler implements EventHandler {

    @Override
    public Response handleEvent(Context context, Event event) {
       return new Response().setBody("");
    }
}
```

The `handler` field must simply contain the class name. For the above, the `handler` would be `EmptyHandler`.

## Build
When instructed to build the user's handler (to create a user handler jar), the Java runtime will generate a Gradle build script from the following template:
```
plugins {
  id 'com.github.johnrengelman.shadow' version '2.0.2'
  id 'java'
}

repositories {
	{{ range .Repositories }}
	{{ . }}
	{{ end }}
}

dependencies {
	{{ range .Dependencies }}
	compile group: '{{.Group}}', name: '{{.Name}}', version: '{{.Version}}'
	{{ end }}

    compile files('./nuclio-sdk-1.0-SNAPSHOT.jar')
}

shadowJar {
   baseName = 'user-handler'
   classifier = null  // Don't append "all" to jar name
}

task userHandler(dependsOn: shadowJar)
```

The shim layer jar is contained within the `onbuild` image and an uber jar is created from user's jar and the shim layer jar. All dependencies (e.g. `com.github.johnrengelman.shadow`) are contained within the build cache, so no internet access is required by the basic build process.

### Dependencies
The Java runtime will format `spec.build.dependencies` into the Gradle build script. For example, the following `function.yaml` section:

```yaml
spec:
  build:
    dependencies:
    - "group: com.fasterxml.jackson.core, name: jackson-databind, version: 2.9.0"
    - "group: com.fasterxml.jackson.core, name: jackson-core, version: 2.9.0"
    - "group: com.fasterxml.jackson.core, name: jackson-annotations, version: 2.9.0"
```

Will populate the Gradle build script as follows:
```
dependencies {
	compile group: 'com.fasterxml.jackson.core', name: 'jackson-databind', version: '2.9.0'
	compile group: 'com.fasterxml.jackson.core', name: 'jackson-core', version: '2.9.0'
	compile group: 'com.fasterxml.jackson.core', name: 'jackson-annotations', version: '2.9.0'
}
```

### Runtime attributes
By providing the `repositories` runtime attribute, one can override the `repositories` section in the `build.gradle`. When this field is empty, `mavenCentral()` is used. For example this `function.yaml` section:

```yaml
spec:
  build:
    runtimeAttributes:
      repositories:
      - mavenCentral()
      - jcenter()
```

Will populate the Gradle build script as follows:
```
repositories {
    mavenCentral()
    jcenter()
}
```

### Custom Gradle script
Providing a `build.gradle` file inside the function directory or archive overrides the script generation.  

## Dockerfile
See [deploying Functions from Dockerfile](/docs/tasks/deploy-functions-from-dockerfile.md).

```
ARG NUCLIO_LABEL=0.5.0
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=openjdk:9-jre-slim
ARG NUCLIO_ONBUILD_IMAGE=nuclio/handler-builder-java-onbuild:${NUCLIO_LABEL}-${NUCLIO_ARCH}

# Supplies processor, handler.jar
FROM ${NUCLIO_ONBUILD_IMAGE} as builder

# Supplies uhttpc, used for healthcheck
FROM nuclio/uhttpc:0.0.1-amd64 as uhttpc

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=builder /home/gradle/bin/processor /usr/local/bin/processor
COPY --from=builder /home/gradle/src/wrapper/build/libs/nuclio-java-wrapper.jar /opt/nuclio/nuclio-java-wrapper.jar
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://localhost:8082/ready || exit 1

# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]
```
