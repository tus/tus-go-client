// Package e2e contains the end-to-end tests of tusgo against tusd (https://github.com/tus/tusd), the reference TUS
// server implementation.
//
// tusd is embedded into the test process as an [net/http.Handler] backed by a file store in a temporary directory,
// so the tests need no external server and can both inject faults between the client and the server and inspect the
// data the server has actually stored.
package e2e
