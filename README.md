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

Install [Rust stable](https://www.rust-lang.org/tools/install), [LLVM/Clang 17 or newer](https://releases.llvm.org/), and a native linker/SDK:

- Windows: Visual Studio or Build Tools with **Desktop development with C++** and a Windows SDK.
- macOS: Xcode Command Line Tools (`xcode-select --install`); Apple Clang is supported for the emitted IR subset.
- Linux: Clang and the C development toolchain for your distribution.

From the repository root:

```sh
cargo build --release --locked
cargo run -- check examples/hello.gb
cargo run -- run examples/hello.gb
cargo run -- run examples/fibonacci.gb --release
cargo run -- run examples/pointers.gb
```

The compiled CLI is `target/release/gabe` (`gabe.exe` on Windows). To install it in your Cargo binary directory, run `cargo install --path crates/gabe-cli --locked`.

On Windows, this environment helper discovers Clang or an existing `llc` bundled with the .NET Android SDK:

```powershell
. .\scripts\env.ps1
cargo run -- run examples/hello.gb
```

Use `GABE_CLANG` / `--clang` to select a Clang executable, or `GABE_LLC` / `--llc` to select LLVM's object generator. The `llc` backend can also link Windows/MSVC executables; executable linking on Linux and macOS uses Clang. `llc -O3` applies code-generation optimization only. Clang `-O3` also runs LLVM's IR optimization pipeline and is the primary backend for performance work.

## What works in v0.1

- A lexer, parser, type checker, typed HIR, and LLVM IR generator written in Rust with no third-party dependencies.
- `int`, `long`, `double`, `bool`, `string`, `void`, typed pointers, numeric widening, and `var` inference.
- Functions, recursion, static methods grouped in classes, and `int Main()` / `void Main()` entry points.
- Scoped mutable variables, assignment, arithmetic, comparisons, short-circuit booleans, and prefix/postfix increment.
- `if` / `else`, `while`, `for`, `break`, `continue`, and `return`.
- `Console.Write` / `Console.WriteLine` for primitive values and UTF-8 string literals.
- Unsafe stack addresses, pointer dereferencing, and primitive C ABI calls.
- Source diagnostics, native executables, object files, LLVM IR, optimization options, and target triples.

This is a C#-style subset. Classes currently group static functions. Objects, inheritance, arrays, structs, generics, modules, heap ownership, exceptions, and a standard library are planned. Read [the language reference](docs/language.md) for precise semantics.

## Targets and cross-compilation

```sh
cargo run -- targets
cargo run -- build examples/hello.gb --emit llvm -o build/hello.ll
cargo run -- build examples/hello.gb --emit obj --target linux-arm64 -o build/hello-arm64.o
cargo run -- build examples/hello.gb --target linux-arm64 --sysroot /path/to/sdk --link-arg "-fuse-ld=lld"
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
crates/
  gabe-compiler/   frontend, type analysis, LLVM generation, frontend tests
  gabe-cli/        command-line driver, toolchain integration, native tests
examples/         runnable .gb programs
docs/             language, architecture, targets, decisions, roadmap
scripts/          environment discovery, validation, private GitHub publishing
.github/          Windows/Linux/macOS CI and issue/PR templates
AGENTS.md         instructions for future AI agents and contributors
```

Validate changes with `scripts/check.ps1` (PowerShell) or `scripts/check.sh` (POSIX). Native tests exercise observable behavior at `-O0` and `-O3`, and inspect emitted COFF, ELF, and Mach-O objects. GitHub CI runs native tests on Windows, Linux, and macOS.

## Repository and contribution

The intended GitHub repository is **private**, named **gabe-lang**, on the owner's account. To finish initial setup in a terminal with Git write access:

```powershell
# Install GitHub CLI from https://cli.github.com if gh is unavailable.
gh auth login
.\scripts\initialize-repo.ps1 -Publish
```

The setup script creates the initial commit if one does not exist, then invokes `scripts/publish-github.ps1` to create the private remote and push the Git history. Use the setup script without `-Publish` to finish only the initial local commit. Publishing refuses to overwrite an existing origin.

See [CONTRIBUTING.md](CONTRIBUTING.md), [AGENTS.md](AGENTS.md), and [the roadmap](docs/roadmap.md). A distribution license has not been selected.
