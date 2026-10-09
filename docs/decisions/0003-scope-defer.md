# ADR 0003: Deferred calls at lexical scope exit

Status: accepted at the owner's request.

## Context

Gabe needs a simple way to arrange resource cleanup while retaining native performance and explicit low-level control. The requested syntax is `defer DoSomething();`, with cleanup when the current block exits. Go provides a useful precedent for argument capture and reverse call order, but its defers run when the whole function returns.

## Decision

Add `defer` as a reserved statement keyword followed by a call expression and a semicolon. Evaluate arguments and implicit conversions once, from left to right, when the statement is reached. Save those values and discard any result of the eventual call. Use ordinary name, arity, type, and unsafe checks.

Associate the call with its current lexical scope. Run reached calls in reverse order on fallthrough, return, break, or continue, cleaning up scopes from innermost to outermost. Braced blocks, unsafe blocks, and implicit scopes around unbraced branch/loop bodies participate. Each loop iteration cleans up before its step or next condition; loop exits preserve surrounding scopes. Evaluate return expressions before cleanup. Fatal traps and process termination do not unwind.

Semantic analysis assigns typed hidden local slots to captured arguments. LLVM lowering allocates these slots in the entry block and stores arguments at registration. It tracks lexical cleanup lists and emits ordinary calls at each exit. Return values stay in SSA operands computed before those calls. Loop target metadata records the cleanup depth to retain on break or continue.

Every static defer statement can be reached at most once per activation of its scope under the current structured control flow. A loop body has a fresh scope each iteration, so its capture slots can be reused after cleanup. No dynamic registration stack, heap allocation, or additional runtime is required. Keep ordinary LLVM output unchanged for programs without defers.

## Consequences

The syntax stays small, and block exit gives resources a predictable cleanup point. Capturing arguments makes later changes to local values irrelevant to the saved arguments; a saved pointer still observes later changes to its pointee. Defer does not extend memory lifetimes or provide ownership checking.

Calls may be emitted at several exits, increasing unoptimized code size. LLVM may optimize capture slots and duplicated cleanup paths. Performance claims still require measurements. Future closures, exceptions, goto, or other control-flow changes must revisit the static registration invariant and unwinding rules.

Acceptance, rejection, fuzz, and native tests cover capture order, nested blocks, conditional registration, repeated loop iterations, returns, break/continue, and unsafe C interop at `-O0` and `-O3`. The existing LLVM compatibility fixture remains unchanged.

Reference: [Go defer specification](https://go.dev/ref/spec#Defer_statements).
