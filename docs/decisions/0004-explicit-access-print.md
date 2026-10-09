# ADR 0004: Explicit access and simple printing

Status: accepted at the owner's request.

## Context

The initial C#-inspired grammar accepted omitted access modifiers and discarded explicit access modifiers. Output used `Console.Write` and `Console.WriteLine`. The owner wants declarations to require `public` or `private`, and printing to use the simpler names `Print` and `PrintLine`.

## Decision

Require exactly one `public` or `private` on every class, file-scope function, static method, and extern declaration. Keep modifier order flexible. Reject omitted, repeated, conflicting, or `internal` access modifiers with a source diagnostic. Locals and parameters keep their existing syntax without access modifiers.

Carry function access and its declaring class through AST and semantic signatures. Private methods are callable only from the same class, whether the call is qualified, unqualified, or deferred. Public methods can be called across classes. Private file-scope functions, externs, and classes are visible throughout the current single source file. A safe parameterless `Main` can be public or private. Access does not change native symbol names or define library exports; modules and library export rules remain future work.

Replace console output built-ins with unqualified `Print(value)` and `PrintLine(value)`. `Print` requires one printable primitive and appends no newline; `PrintLine` accepts zero or one printable primitive and appends a newline. Literal newline characters are printed normally. Both calls return void and work with `defer`. Preserve the existing typed print HIR, LLVM lowering, C runtime formatting, and literal percent handling.

Reserve `Print` and `PrintLine` function names at file scope. Unqualified calls always select these built-ins; qualified class methods with these names remain ordinary calls subject to access checks. Remove the former `Console` and `System.Console` output built-ins. Keep optional `using System;` accepted while examples use printing without imports.

## Consequences

Existing sources must add explicit access modifiers and migrate their output calls. Update every example, source-based regression test, CLI help, contributor guidance, and language reference. Cover missing and conflicting modifiers, private access violations, legacy output rejection, same-line output, embedded newlines, printable types, and deferred output with frontend and native tests at `-O0` and `-O3`.

Migrate `compiler/testdata/baseline.gb` to the new grammar while keeping the Rust-era LLVM golden file byte-for-byte unchanged. Its existing helper remains named `Program.Print` and is called explicitly as `Program.Print(...)`; its body uses the new built-ins. Keeping the helper name preserves the LLVM source-name comment as well as all native behavior. This is a frontend syntax change with no intentional backend change.
