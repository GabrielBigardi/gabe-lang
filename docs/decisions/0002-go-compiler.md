# ADR 0002: Implement the compiler in Go

Status: accepted at the owner's request.

## Context

The owner finds Rust unfamiliar and is concerned about compiler build times. The v0.1 bootstrap already has working language semantics and an LLVM backend. Changing the implementation language should make the compiler easier to maintain while preserving Gabe's syntax, generated code, and low-level direction. Faster builds are a motivation, not a measured guarantee about either compiler performance or generated program performance.

## Decision

Replace the Rust workspace with a Go 1.25+ module using only the standard library. Put the executable in `cmd/gabe/`, the reusable frontend and LLVM emitter in `compiler/`, external tool invocation in `internal/cli/`, and the formatting check in `tools/checkfmt/`. Do not use cgo, LLVM Go bindings, the Go `unsafe` package, or a Go-based runtime for generated programs.

Preserve the lexer → parser → AST → semantic analysis → typed HIR → textual LLVM IR pipeline. Keep the existing Clang and LLVM `llc` integrations, platform linker discovery, target aliases, and CLI options. Port acceptance, rejection, and native tests, and continue requiring native execution at `-O0` and `-O3` on Windows, Linux, and macOS on x64 and ARM64.

Capture `compiler/testdata/baseline.golden` using the Rust compiler at commit `60f13cb` for `baseline.gb` targeting `linux-arm64`. It covers functions, widening, pointers, interop, assignments, floating operations, console output, loops, short-circuiting, and integer trap guards. Require byte-for-byte agreement in the Go frontend test suite. Keep a native test for the fixture and fuzz the source-to-IR API for diagnostics instead of crashes. The original source remains available in Git history.

## Consequences

Building and maintaining the compiler now requires Go instead of Rust. The compiler executable has Go's runtime and garbage collector; generated Gabe executables still contain native LLVM output and link the platform C runtime. Their ownership and allocation behavior remains a separate Gabe language decision.

Go's `GOOS`/`GOARCH` cross-build the compiler executable without cgo. Gabe's `--target` continues to select generated machine code through LLVM and still requires suitable target linkers, SDKs, and runtime libraries. The migration does not eliminate LLVM optimization and linking time. Measure compiler build latency, source compilation latency, and generated program performance separately before making speed claims.
