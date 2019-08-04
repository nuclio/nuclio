# Changelog

#### Version 1.2.0 (2019-06-14)

*Note: This release requires Golang at least 1.7, which is higher than the
previous release. All the versions being dropped are multiple years old and no
longer supported upstream, so I'm not counting this as a breaking change.*

 - Add `RunCtx` method on `Retrier` to support running with a context.
 - Ensure the `Retrier`'s use of random numbers is concurrency-safe.
 - Bump CI to ensure we support newer Golang versions.

#### Version 1.1.0 (2018-03-26)

 - Improve documentation and fix some typos.
 - Bump CI to ensure we support newer Golang versions.
 - Add `IsEmpty()` method on `Semaphore`.

#### Version 1.0.0 (2015-02-13)

Initial release.
