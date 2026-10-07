# Gabe roadmap

The starting priorities are a working C#-style language, native performance, explicit low-level capabilities, and a maintainable compiler. The owner plans to direct syntax changes after the baseline works.

## v0.1: working bootstrap

Implemented:

- Rust workspace with compiler library and `gabe` CLI.
- Lexer, parser, source diagnostics, typed HIR, semantic checking.
- Primitive types, functions/static class containers, recursion, mutable locals.
- Expressions, widening, assignment, short-circuit booleans, integer trap behavior.
- Conditionals, loops, loop control, and console output.
- Unsafe stack pointers and scalar C ABI extern calls.
- LLVM IR, cross-target object generation, native linking and execution on Windows.
- End-to-end tests, contributor guidance, and configured Windows/Linux/macOS CI.

Pending external setup: finish the initial Git commit in a terminal with Git metadata write access, create/push the private GitHub repository once authenticated, then inspect real CI results. `scripts/initialize-repo.ps1 -Publish` handles initial commit and private publishing after `gh auth login`.

## Next: useful native programs

1. Run and fix the configured native CI on Windows, Linux, and macOS on both x64 and ARM64.
2. Add multiple source files, imports/modules, library compilation, and export rules.
3. Add value structs, fixed/dynamic arrays and slices, indexing, explicit layout, and bounds behavior.
4. Decide ownership/lifetimes and resource disposal before adding a general heap/object model. Keep unsafe escape hatches explicit.
5. Extend low-level integers, casts, bitwise operations, pointer arithmetic, and C ABI coverage with target-aware layout tests.
6. Add multi-error diagnostics, debug metadata, stack traces, and editor tooling.

## Performance milestones

- Establish a reproducible benchmark harness against equivalent Rust/C++ programs, with identical work and optimization settings.
- Inspect optimized LLVM IR and assembly for hot loops, calls, stack promotion, aliasing, and bounds checks.
- Add optimization pipeline visibility, CPU/features options, LTO, and allocation/resource benchmarks.
- Track compiler speed and memory use independently of generated program performance.

No performance parity is claimed until measurements exist. Avoid choosing unsafe defaults simply to make a benchmark faster.

## Later, after language decisions

Generics/monomorphization, interfaces, methods on value types, an explicit native object model, standard library modules, package/build tooling, concurrency, incremental compilation, a language server, and broader targets.

Syntax proposals should include runnable before/after examples, grammar/ambiguity notes, diagnostic effects, and migration impact. Each accepted feature should land as a complete lexer/parser/checker/backend/tests/documentation slice.
