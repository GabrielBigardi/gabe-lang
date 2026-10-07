# ADR 0001: Rust frontend and textual LLVM IR

Status: accepted for the v0.1 bootstrap.

## Context

Gabe needs a working compiler with C#-style syntax, native output, target architecture flexibility, and a low-friction development setup. Linking a particular LLVM C/C++ library into the first compiler would couple Rust builds to LLVM installation details before the language core is established.

## Decision

Implement the frontend and driver in safe Rust. Emit deterministic textual LLVM IR using opaque pointers and an LLVM 17+ compatible instruction subset. Use Clang for optimization, machine-code generation, and native linking. Also support LLVM `llc` object output, with MSVC linking on Windows when available.

Keep parser AST, typed HIR, backend, and driver distinct. Keep the first compiler free of third-party Rust dependencies. Generated programs use native machine code and the platform C runtime, with no VM or garbage collector.

## Consequences

Frontend checks and LLVM emission work without a separately installed LLVM development library. IR can be inspected and independently compiled. Target triples remain explicit. Native compilation requires external LLVM tools; cross-linking requires platform SDKs and libraries. `llc`'s optimization setting does not replace Clang's IR optimization pipeline.

An in-process LLVM API can replace the textual emitter later if measurements, diagnostics, debug information, or compiler latency justify it. Typed HIR and the public compiler entry points should survive that change.
