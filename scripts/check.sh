#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
cargo fmt --all -- --check
cargo clippy --workspace --all-targets --locked -- -D warnings
GABE_REQUIRE_NATIVE_TESTS=1 cargo test --workspace --locked
