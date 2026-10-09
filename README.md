# gabe-lang

**Gabe** is an experimental native programming language with **`.gb`** source files, **C#-inspired syntax**, and an **LLVM backend**.

The goal is Rust/C++-class performance and low-level control with the simplicity and readability of C#. Version 0.1 establishes a working compiler; performance parity has not been benchmarked.

```csharp
using System;

class Program
{
    public static int Main()
    {
        Console.WriteLine("Hello from Gabe!");
        return 0;
    }
}
```

## Get started

Install [Go 1.25 or newer](https://go.dev/dl/), [LLVM/Clang 17 or newer](https://releases.llvm.org/), and a native linker/SDK:

- Windows: Visual Studio or Build Tools with **Desktop development with C++** and a Windows SDK.
- macOS: Xcode Command Line Tools (`xcode-select --install`); Apple Clang is supported for the emitted IR subset.
- Linux: Clang and the C development toolchain for your distribution.

From the repository root:

```sh
go build -trimpath -o bin/ ./cmd/gabe
go run ./cmd/gabe check examples/hello.gb
go run ./cmd/gabe run examples/hello.gb
go run ./cmd/gabe run examples/fibonacci.gb --release
go run ./cmd/gabe run examples/pointers.gb
go run ./cmd/gabe run examples/defer.gb
```

The compiled CLI is `bin/gabe` (`bin/gabe.exe` on Windows). To install it in your Go binary directory, run `go install ./cmd/gabe`. The compiler uses only the Go standard library and needs no cgo or LLVM bindings. Generated Gabe programs are native LLVM output and do not include the Go runtime or garbage collector.

On Windows, this environment helper discovers Clang or an existing `llc` bundled with the .NET Android SDK:

```powershell
. .\scripts\env.ps1
go run ./cmd/gabe run examples/hello.gb
```

Use `GABE_CLANG` / `--clang` to select a Clang executable, or `GABE_LLC` / `--llc` to select LLVM's object generator. The `llc` backend can also link Windows/MSVC executables; executable linking on Linux and macOS uses Clang. `llc -O3` applies code-generation optimization only. Clang `-O3` also runs LLVM's IR optimization pipeline and is the primary backend for performance work.

## What works in v0.1

- A lexer, parser, type checker, typed HIR, and LLVM IR generator written in Go with no third-party dependencies.
- `int`, `long`, `double`, `bool`, `string`, `void`, typed pointers, numeric widening, and `var` inference.
- Functions, recursion, static methods grouped in classes, and `int Main()` / `void Main()` entry points.
- Scoped mutable variables, assignment, arithmetic, comparisons, short-circuit booleans, and prefix/postfix increment.
- `if` / `else`, `while`, `for`, `break`, `continue`, and `return`.
- `defer Call();` with captured arguments and reverse-order cleanup when the current scope exits.
- `Console.Write` / `Console.WriteLine` for primitive values and UTF-8 string literals.
- Unsafe stack addresses, pointer dereferencing, and primitive C ABI calls.
- Source diagnostics, native executables, object files, LLVM IR, optimization options, and target triples.

This is a C#-style subset. Classes currently group static functions. Objects, inheritance, arrays, structs, generics, modules, heap ownership, exceptions, and a standard library are planned. Read [the language reference](docs/language.md) for precise semantics.

## Targets and cross-compilation

```sh
go run ./cmd/gabe targets
go run ./cmd/gabe build examples/hello.gb --emit llvm -o build/hello.ll
go run ./cmd/gabe build examples/hello.gb --emit obj --target linux-arm64 -o build/hello-arm64.o
go run ./cmd/gabe build examples/hello.gb --target linux-arm64 --sysroot /path/to/sdk --link-arg "-fuse-ld=lld"
```

| Alias | LLVM target triple |
| --- | --- |
| `windows-x64` | `x86_64-pc-windows-msvc` |
| `windows-arm64` | `aarch64-pc-windows-msvc` |
| `linux-x64` | `x86_64-unknown-linux-gnu` |
| `linux-arm64` | `aarch64-unknown-linux-gnu` |
| `macos-x64` | `x86_64-apple-darwin` |
| `macos-arm64` | `arm64-apple-darwin` |
| `linux-riscv64` | `riscv64-unknown-linux-gnu` |
| `wasm32` | `wasm32-unknown-unknown` |

x64 is AMD64/x86-64; ARM64 is AArch64. Other valid LLVM triples are accepted when the installed LLVM build supports that architecture. A target alias does not bundle its SDK, linker, C runtime, or execution environment. Object generation needs the LLVM backend; cross-linking also needs target libraries and tools. `wasm32` and RISC-V are experimental aliases; the console runtime needs a suitable libc or host imports for WebAssembly linking. See [target details](docs/targets.md).

## Project layout

```text
cmd/gabe/         compiler executable entry point
compiler/         frontend, typed HIR, LLVM generation, frontend tests
internal/cli/     command-line driver, toolchain integration, native tests
tools/checkfmt/   portable Go formatting check
examples/         runnable .gb programs
docs/             language, architecture, targets, decisions, roadmap
scripts/          environment discovery, validation, private GitHub publishing
.github/          Windows/Linux/macOS CI and issue/PR templates
AGENTS.md         instructions for future AI agents and contributors
```

Validate changes with `scripts/check.ps1` (PowerShell) or `scripts/check.sh` (POSIX). They check Go formatting, run `go vet`, require native tests, and build the compiler. Native tests exercise observable behavior at `-O0` and `-O3`, and inspect emitted COFF, ELF, and Mach-O objects. GitHub CI runs on Windows, Linux, and macOS, each on x64 and ARM64. A golden fixture preserves the original Rust compiler's LLVM output; see [ADR 0002](docs/decisions/0002-go-compiler.md) for the migration.

## Repository and contribution

The GitHub repository is [GabrielBigardi/gabe-lang](https://github.com/GabrielBigardi/gabe-lang) and is **private**. [Compiler CI](https://github.com/GabrielBigardi/gabe-lang/actions/workflows/ci.yml) validates pushes to `main` and pull requests. The initialization and private publishing scripts remain available for setting up a new copy; they are not needed for normal development in this repository.

See [CONTRIBUTING.md](CONTRIBUTING.md), [AGENTS.md](AGENTS.md), and [the roadmap](docs/roadmap.md). A distribution license has not been selected.
