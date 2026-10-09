# Gabe contributor and agent guide

## Product direction

- Language name: **gabe**. Project/repository: **gabe-lang**. Source extension: **.gb**. Compiler command: **gabe**.
- Gabe should combine native performance and explicit low-level control with the readability and simplicity of C#.
- The first syntax is intentionally close to C#. The owner plans to request syntax changes after the baseline works. Avoid unsolicited language redesigns.
- Performance comparable to Rust/C++ is a design goal, not a proven property. Support claims with reproducible optimized benchmarks.
- Generated programs use native machine code and currently link the platform C runtime. They have no VM or garbage collector.

## Read before changing code

1. `README.md` for setup and the currently working features.
2. `docs/language.md` for actual language semantics and limitations.
3. `docs/architecture.md` for stage boundaries and design choices.
4. `docs/roadmap.md` for priorities; future features are not implemented promises.

## Repository map

```text
cmd/gabe/        compiler executable entry point
compiler/
  diagnostic.go  byte spans and source diagnostics
  lexer.go       tokens, comments, strings, numbers
  ast.go         parsed syntax and types
  parser.go      recursive-descent statements and precedence parsing
  hir.go         typed intermediate representation
  sema.go        names, scopes, types, returns, unsafe restrictions
  llvm.go        deterministic LLVM IR generation
  target.go      aliases and LLVM target triples
  testdata/      LLVM compatibility fixture from the Rust baseline
internal/cli/    driver, LLVM processes, platform linking, end-to-end tests
tools/checkfmt/  portable formatting check
examples/        compilable .gb examples
docs/            specifications, decisions, and development roadmap
scripts/         local environment, validation, GitHub publishing
.github/         CI and contributor templates
```

## Development commands

The Go module requires Go 1.25 or newer and has no third-party dependencies. `go.mod` is committed; `go.sum` is unnecessary while the module uses only the standard library.

```text
go run ./tools/checkfmt
go vet ./...
go test -count=1 ./...
go build -trimpath -o bin/ ./cmd/gabe
go run ./cmd/gabe check examples/hello.gb
go run ./cmd/gabe build examples/hello.gb --emit llvm
go run ./cmd/gabe run examples/hello.gb
```

Native builds require LLVM/Clang 17+ and a platform linker/SDK. `GABE_CLANG` selects Clang. `GABE_LLC` selects the alternative LLVM object backend; it can link Windows executables using installed MSVC and Windows SDK libraries. On Windows, dot-source `scripts/env.ps1` to discover an existing backend. `scripts/check.ps1` and `scripts/check.sh` require native tests.

`scripts/initialize-repo.ps1` finishes the initial Git commit if needed. `-Publish` invokes the private GitHub publishing script after GitHub CLI authentication. If the execution environment marks `.git` read-only, report that restriction and leave the prepared command for a terminal with Git write access; do not claim a commit or push happened.

**Set `GABE_REQUIRE_NATIVE_TESTS=1` for completion checks.** Without it, native tests report a skip when LLVM is unavailable. A skipped native test is not evidence of working machine code. CI requires these tests.

Format edits with `gofmt -w cmd compiler internal tools`. Run `go test ./compiler -run '^$' -fuzz '^FuzzCheck$' -fuzztime=15s` for parser/diagnostic changes. In a filesystem sandbox that cannot write Go's default cache, point `GOCACHE` and `GOMODCACHE` at absolute paths under the ignored `.local/` directory for that process.

## Implementation rules

- Keep lexing, parsing, analysis, LLVM lowering, and tool invocation separate. LLVM receives typed HIR only.
- Report invalid source with a diagnostic and span. User errors must not panic. Assertions in lowering may only enforce analyzed invariants.
- New semantics need both passing behavior tests and rejection tests where applicable. Check native results at `-O0` and `-O3` for changes that can affect generated code.
- Preserve left-to-right evaluation, single evaluation of assignment addresses, and short-circuit `&&` / `||`.
- Require exactly one `public` / `private` modifier on classes, functions, methods, and externs. Reject missing, repeated, conflicting, or `internal` access modifiers. Private methods are callable only inside their class; private file-scope declarations are visible throughout the current source file.
- Use unqualified `Print` / `PrintLine` built-ins for output; legacy `Console` output built-ins are removed. Reserve these function names at file scope and keep ordinary qualified class methods distinct.
- `defer` captures arguments once at registration and runs calls in reverse order at lexical scope exit, including implicit branch/loop body scopes. Clean up only scopes left by `break` / `continue`; evaluate return values before cleanup. Traps do not unwind.
- Emit allocas in the entry block. Terminate every LLVM basic block. Update phi predecessors after expressions that introduce control flow.
- Use opaque LLVM pointers and explicit Gabe pointee types. Native pointer operations and extern calls require `unsafe` in Gabe source. Unsafe source is a programmer contract; no lifetime/borrow analysis exists yet.
- Integer add/subtract/multiply wrap. Do not introduce `nsw` / `nuw` without changing the specification. Integer divide/remainder trap on zero and signed MIN/-1. Floating arithmetic uses ordinary IEEE operations without fast-math.
- Keep `int` at 32 bits, `long` at 64 bits, and `double` at 64 bits across targets. Handle ABI and pointer-width assumptions deliberately.
- Reserve generated `gb_` symbols and runtime symbols against extern declarations. Never treat user strings as printf format strings.
- Avoid embedding machine-specific paths in committed files. Do not commit tool downloads, binaries, build directories, credentials, or `.local` files.
- Compiler implementation uses Go with only the standard library, without cgo or the Go `unsafe` package. Do not add a dependency or replace the LLVM backend without a clear reason recorded in an architecture decision.
- Keep the compiler's Go runtime separate from generated Gabe programs. The implementation language does not determine the output language's allocation or ownership model.
- Preserve the LLVM compatibility fixture captured from Rust commit `60f13cb`. Update its golden IR only for an intentional backend change, with native behavior tests and a documented reason.
- Update the language reference, examples, and CLI help when behavior changes. Keep architecture decisions in `docs/decisions/`.
- Distinguish LLVM IR emission, object generation, linking, and execution when reporting target support. Emitting an ARM64/macOS object on Windows does not prove a linked macOS application runs.
- The owner requested a public GitHub repository after a privacy audit. Do not change repository visibility or licensing, or publish packages, without an explicit owner instruction. Use the owner's GitHub noreply email for commits and keep personal contact details out of committed metadata.

## Current baseline and next work

v0.1 has a single-file compiler, primitive types, functions/static class containers, console output, mutable locals, numeric widening, conditionals, loops, scope-exit deferred calls, unsafe stack pointers, primitive C interop, LLVM IR/object/executable output, and target triples. See the roadmap for structs, arrays, memory ownership, modules, diagnostics/debug information, and performance work. Add features as complete vertical slices instead of placeholders.
