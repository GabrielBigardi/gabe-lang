//! The Gabe frontend and LLVM IR backend. No external dependencies or host runtime.
pub mod ast;
pub mod diagnostic;
pub mod hir;
mod lexer;
mod llvm;
mod parser;
mod sema;
pub mod target;

pub use diagnostic::{Diagnostic, Result};

pub fn check(source: &str) -> Result<hir::Program> {
    sema::check(parser::parse(lexer::lex(source)?)?)
}

pub fn compile(source: &str, target: &str) -> Result<String> {
    let triple = target::resolve(target)?;
    Ok(llvm::emit(&check(source)?, &triple))
}
