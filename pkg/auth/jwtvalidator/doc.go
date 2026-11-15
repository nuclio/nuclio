/*
Copyright 2026 The Nuclio Authors.

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

// Package jwtvalidator verifies JWTs issued by an external identity provider against its public keys.
// It knows nothing about files, Kubernetes or functions: it gets a token and an issuer entry.
//
// This part of the package is the key layer: a KeyProvider returns the identity provider's public key for a
// key ID, and JWKSDownloader is the implementation that downloads the keys. The Validator that checks a
// token with these keys is added on top of it.
package jwtvalidator
