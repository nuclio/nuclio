**Note:** The cgo directory contains a collection of cgo wrappers over the
canonical C implementation for each compression format. These cgo wrappers are
only used by the fuzzer and bench tools to test for correctness and performance
of the Go implementations relative to the C implementations.
There are no unit tests for each wrapper since they are thoroughly tested by
the aforementioned tools.
