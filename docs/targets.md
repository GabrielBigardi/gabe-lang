# Targets and toolchains

Gabe emits LLVM IR. LLVM performs machine-code generation; the platform toolchain links the resulting objects. Go builds the Gabe compiler itself. Go's `GOOS` and `GOARCH` select a host for that compiler executable; Gabe's `--target` selects the output program's LLVM target. Neither mechanism installs the output target's SDK or runtime libraries.

## Three distinct outputs

| `--emit` | Requires | Result |
| --- | --- | --- |
| `llvm` | Gabe only | LLVM IR containing the selected target triple |
| `obj` | Clang or `llc` with the requested architecture backend | COFF, ELF, Mach-O, or another target object format |
| `exe` | LLVM tools, a compatible linker, target C runtime/system libraries | Linked native executable |

`gabe run` builds and executes only the host target. Cross-target executables require an appropriate machine or emulator to execute.

## Primary target families

Windows/MSVC, Linux/GNU, and macOS/Darwin aliases exist for x64 and ARM64. This bootstrap was validated on Windows/x64 with LLVM 18.1.6 and MSVC: native examples and end-to-end tests executed, and COFF/ELF/Mach-O objects were generated for all six primary target combinations. Executables on Linux, macOS, or Windows ARM64 were not run in this initial local environment.

CI runs the Go compiler and generated native programs on Windows, Linux, and macOS, each on x64 and ARM64, plus cross-object generation. Inspect the [actual CI results](https://github.com/GabrielBigardi/gabe-lang/actions/workflows/ci.yml) when assessing a particular revision.

## Clang

```sh
gabe build examples/hello.gb --release
gabe build examples/hello.gb --emit obj --target linux-arm64 -o build/hello.o
gabe build examples/hello.gb --target linux-arm64 --sysroot /sdk/aarch64 --link-arg "-fuse-ld=lld"
```

Select a driver with `--clang /path/to/clang` or `GABE_CLANG`. `--target` accepts aliases or LLVM target triples; `--sysroot` supplies a target root; repeated `--link-arg` values pass through to Clang during executable linking. Options take separate values. Darwin SDK selection can also be passed through as `--link-arg "-isysroot" --link-arg "/path/to/MacOSX.sdk"` for executable links.

Clang can generate objects for several targets from one installation. A cross-link also needs correctly targeted CRT objects, libraries, and a linker. macOS linking needs an appropriate Apple SDK/toolchain. Windows/MSVC linking needs matching MSVC and Windows SDK libraries. Linux GNU linking needs the target's libc and startup objects. A target alias alone supplies none of those files.

The Linux executable driver also links `libm` for floating-point runtime helpers. Supply it in a Linux target sysroot; additional OS targets can use `--link-arg "-lm"` when their platform requires a separate math library.

For Windows/MSVC executables, the driver links `legacy_stdio_definitions.lib` to resolve direct `printf` calls from generated LLVM IR. The target's MSVC libraries must include this library, including when targeting ARM64.

## LLVM llc and Windows fallback

```powershell
. .\scripts\env.ps1
gabe build examples/hello.gb --llc 'C:\path\to\llc.exe'
gabe build examples/hello.gb --llc 'C:\path\to\llc.exe' --emit obj --target macos-arm64
```

Some Windows .NET Android SDK installations include `llc`. The environment helper can discover it. The driver can use it for objects across installed LLVM backends and link Windows/MSVC executables using Visual Studio discovery (`vswhere`) and Windows SDK libraries. `--linker` selects an MSVC-compatible linker; `--link-arg` passes raw MSVC linker arguments on this path. `--sysroot` is supported only on the Clang path.

The fallback currently links a static C runtime on Windows and supports x64/ARM64 target library discovery. For Linux/macOS executables, use Clang or link emitted objects with your platform toolchain. `llc -O3` tunes code generation; it does not run Clang's complete LLVM IR optimization pipeline.

## Additional architectures

`linux-riscv64` and `wasm32` are experimental aliases. The available `llc` on the initial machine contains x86 and ARM backends only, so those additional object targets were not locally validated. A full LLVM build may support more; `llc --version` lists its registered targets. Raw triples for other LLVM architectures are accepted, with backend and runtime availability left to the installed toolchain.

Console output is a C runtime dependency. A freestanding or WebAssembly environment needs libc/host imports or a future alternative runtime. This version does not package a WASI runtime, bare-metal startup code, target SDK downloads, or a universal cross-linker.

References: [Clang cross-compilation](https://clang.llvm.org/docs/CrossCompilation.html), [LLVM llc](https://llvm.org/docs/CommandGuide/llc.html), [LLVM IR reference](https://llvm.org/docs/LangRef.html).
