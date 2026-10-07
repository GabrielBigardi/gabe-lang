# Contributing to Gabe

Read `AGENTS.md`, `docs/language.md`, and `docs/architecture.md` before changing language behavior. C#-inspired syntax is the starting point; syntax changes should follow the owner's direction.

Use a focused branch for each feature or fix. Explain the source program or behavior being changed, the resulting behavior, and validation in the pull request. Keep unrelated cleanup separate.

Changes to compiler semantics need frontend acceptance/rejection coverage and end-to-end native coverage. Update examples and the language reference with the implementation. Significant changes to runtime, type layout, ownership, or backend strategy should include an architecture decision in `docs/decisions/`.

Before opening a pull request:

```sh
cargo fmt --all -- --check
cargo clippy --workspace --all-targets -- -D warnings
cargo test --workspace --locked
```

Set `GABE_REQUIRE_NATIVE_TESTS=1` for the test command. The scripts `scripts/check.ps1` and `scripts/check.sh` do this automatically. `GABE_CLANG` or `GABE_LLC` can select your backend. Without a backend and without the required flag, native tests explicitly skip; do not describe those skips as successful native validation.

Use `--emit llvm` to inspect generated IR, and `--emit obj --target <triple>` to isolate code generation from linking. Cross-platform claims must distinguish object generation, linking, and execution.

No language performance guarantee is established yet. Benchmark optimized generated programs against equivalent Rust/C++ implementations before making performance claims.
