# Compiler architecture

```text
.gb source → lexer → tokens → parser → AST → semantic analysis → typed HIR
                                                            ↓
                                                       LLVM IR (.ll)
                                                            ↓
                                               Clang or LLVM llc
                                                            ↓
                                                   object (.o/.obj)
                                                            ↓
                                                     native linker
                                                            ↓
                                                   native executable
```

## Frontend

`compiler/` is a platform-independent Go package. It owns source spans, syntax, symbol resolution, lexical scopes, primitive types, numeric coercion, return checking, and unsafe restrictions. Its public `Check` API returns typed HIR; `Compile` returns LLVM IR. No LLVM installation is needed for either operation. AST and HIR nodes are private to the package so callers cannot construct invalid lowering input.

The parser uses recursive descent for statements and precedence climbing for binary expressions. AST preserves source spans. Semantic analysis first collects all function signatures, enabling forward calls and recursion, then checks each body. Every local gets a unique ID and fixed type. Assignment targets use a separate typed lvalue representation so indirect addresses are evaluated once.

The current checker reports the first error. Multi-error recovery, incremental source databases, rich error codes, and modules belong in later iterations.

## LLVM backend

The backend emits textual LLVM IR with opaque `ptr` pointers. This is real LLVM input, consumed by Clang or `llc`, rather than transpiled C. This keeps the compiler dependency-free and decouples frontend builds from LLVM's C++ ABI and a particular library installation. [ADR 0001](decisions/0001-bootstrap-backend.md) records the bootstrap backend; [ADR 0002](decisions/0002-go-compiler.md) changes its implementation language from Rust to Go while retaining this architecture.

All local slots are allocated in each function's entry block; Clang's optimization passes can promote them into SSA registers. Branches and loops have explicit basic blocks. Boolean short-circuiting produces branches and phi nodes, preserving evaluation order and side effects. Integer division guards branch to `llvm.trap` before an invalid `sdiv` / `srem` can introduce LLVM undefined behavior.

Generated functions use reserved `gb_<id>` names. A C `main` wrapper calls Gabe's entry point. Primitive C externs keep their native symbol names. String constants are UTF-8 byte arrays with a trailing zero, interned deterministically; output always uses compiler-owned printf format strings.

## Driver and linking

`cmd/gabe/` provides the executable entry point. `internal/cli/` handles filesystem I/O, options, process execution, and tool discovery. It passes compiler output through temporary files, cleans those files on return, and propagates tool failures and program exit status. It rejects outputs that would overwrite the source, including aliases of the same file.

Clang is the primary backend and runs optimization, object generation, and platform linking. The alternative `llc` path supports object generation and Windows/MSVC linking. On Windows this path discovers Visual Studio using `vswhere` and the Windows SDK library directories. It links the static C runtime. Other executable formats use Clang's native linker integration.

Linux executable links include `libm` for helpers such as floating remainder (`fmod`). Target sysroots must provide both the C runtime and the math library.

Windows/MSVC executable links include `legacy_stdio_definitions.lib` on both backend paths. It provides the out-of-line `printf` symbol used by generated LLVM IR, which does not pass through the CRT headers' inline definitions.

An LLVM target triple selects code generation. SDK/sysroot, C runtime, system libraries, and compatible linker are separate inputs. The compiler does not redistribute SDKs or promise universal cross-linking from a bare installation.

## Runtime and performance direction

There is no managed runtime, JIT, garbage collector, or interpreter. Console output currently calls platform `printf`, and externs call primitive C ABI functions. The initial memory surface consists of stack locals and unsafe pointers. Heap allocation, ownership, aggregates, bounds checks, and standard library design still require explicit decisions.

The compiler executable uses Go's runtime and garbage collector. These are development-tool dependencies only: emitting LLVM IR and linking the platform C runtime does not embed the Go runtime in compiled Gabe programs. The compiler uses no cgo, Go `unsafe`, or third-party packages.

Future performance work should examine optimized IR/assembly and measure representative workloads. Debug metadata, library compilation, monomorphization, ABI-safe aggregates, LTO, and reproducible benchmark tooling are planned. LLVM integration alone does not establish performance parity.
