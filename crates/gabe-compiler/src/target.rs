use crate::diagnostic::{Diagnostic, Result, Span};

pub const TARGETS: &[(&str, &str)] = &[
    ("windows-x64", "x86_64-pc-windows-msvc"),
    ("windows-arm64", "aarch64-pc-windows-msvc"),
    ("linux-x64", "x86_64-unknown-linux-gnu"),
    ("linux-arm64", "aarch64-unknown-linux-gnu"),
    ("macos-x64", "x86_64-apple-darwin"),
    ("macos-arm64", "arm64-apple-darwin"),
    ("linux-riscv64", "riscv64-unknown-linux-gnu"),
    ("wasm32", "wasm32-unknown-unknown"),
];

pub fn host() -> &'static str {
    match (std::env::consts::OS, std::env::consts::ARCH) {
        ("windows", "x86_64") => "x86_64-pc-windows-msvc",
        ("windows", "aarch64") => "aarch64-pc-windows-msvc",
        ("linux", "x86_64") => "x86_64-unknown-linux-gnu",
        ("linux", "aarch64") => "aarch64-unknown-linux-gnu",
        ("macos", "x86_64") => "x86_64-apple-darwin",
        ("macos", "aarch64") => "arm64-apple-darwin",
        _ => "unknown",
    }
}

pub fn resolve(name: &str) -> Result<String> {
    if let Some((_, triple)) = TARGETS.iter().find(|(alias, _)| *alias == name) {
        return Ok((*triple).to_owned());
    }
    if name.split('-').count() >= 3
        && name
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || b"-_.".contains(&byte))
    {
        return Ok(name.to_owned());
    }
    Err(Diagnostic::new(
        Span::default(),
        "expected a target alias or LLVM target triple; see 'gabe targets'",
    ))
}
