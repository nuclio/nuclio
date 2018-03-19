# Building Wrapper

We embed user code into the wrapper Jar. The build process should create
`user-handler.jar` in the current directory before calling `gradle shadowJar`

See code in `pkg/processor/build/runtime/java/runtime.go`
