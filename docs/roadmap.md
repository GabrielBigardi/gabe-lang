# Gabe roadmap

The starting priorities are a working C#-style language, native performance, explicit low-level capabilities, and a maintainable compiler. The owner plans to direct syntax changes after the baseline works.

## v0.1: working bootstrap

Implemented:

- Go module with a reusable compiler package and `gabe` CLI, using only the standard library.
- Lexer, parser, source diagnostics, typed HIR, semantic checking.
- Primitive types, functions/static class containers, recursion, mutable locals.
- Expressions, widening, assignment, short-circuit booleans, integer trap behavior.
- Conditionals, loops, loop control, and console output.
- Scope-exit `defer` calls, captured arguments, and cleanup on returns and loop exits.
- Unsafe stack pointers and scalar C ABI extern calls.
- LLVM IR, cross-target object generation, native linking and execution.
- Frontend, native, fuzz, and LLVM compatibility tests; contributor guidance; Windows/Linux/macOS CI on x64 and ARM64.

The private GitHub repository is initialized and published. The original Rust bootstrap passed all six native CI jobs; the Go migration retains the same matrix and its LLVM backend. See [ADR 0002](decisions/0002-go-compiler.md).

## Next: useful native programs

1. Add multiple source files, imports/modules, library compilation, and export rules.
2. Add value structs, fixed/dynamic arrays and slices, indexing, explicit layout, and bounds behavior.
3. Decide ownership/lifetimes and resource disposal before adding a general heap/object model. Keep unsafe escape hatches explicit.
4. Extend low-level integers, casts, bitwise operations, pointer arithmetic, and C ABI coverage with target-aware layout tests.
5. Add multi-error diagnostics, debug metadata, stack traces, and editor tooling.

## Performance milestones

- Establish a reproducible benchmark harness against equivalent Rust/C++ programs, with identical work and optimization settings.
- Inspect optimized LLVM IR and assembly for hot loops, calls, stack promotion, aliasing, and bounds checks.
- Add optimization pipeline visibility, CPU/features options, LTO, and allocation/resource benchmarks.
- Track compiler speed and memory use independently of generated program performance.

No performance parity is claimed until measurements exist. Avoid choosing unsafe defaults simply to make a benchmark faster.

## Later, after language decisions

Generics/monomorphization, interfaces, methods on value types, an explicit native object model, standard library modules, package/build tooling, concurrency, incremental compilation, a language server, and broader targets.

Syntax proposals should include runnable before/after examples, grammar/ambiguity notes, diagnostic effects, and migration impact. Each accepted feature should land as a complete lexer/parser/checker/backend/tests/documentation slice.
