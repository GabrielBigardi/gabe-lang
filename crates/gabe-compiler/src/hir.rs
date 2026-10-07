//! Typed representation. LLVM lowering only receives programs that passed analysis.
use crate::ast::{BinaryOp, Literal, Type, UnaryOp};

#[derive(Debug, Clone)]
pub struct Program {
    pub functions: Vec<Function>,
    pub entry: usize,
}

#[derive(Debug, Clone)]
pub struct Function {
    pub name: String,
    pub symbol: String,
    pub return_type: Type,
    pub parameters: Vec<usize>,
    pub variables: Vec<Type>,
    pub body: Option<Stmt>,
}

#[derive(Debug, Clone)]
pub enum Stmt {
    Block(Vec<Stmt>),
    Let {
        variable: usize,
        value: Expr,
    },
    Expr(Expr),
    Return(Option<Expr>),
    If {
        condition: Expr,
        then_body: Box<Stmt>,
        else_body: Option<Box<Stmt>>,
    },
    While {
        condition: Expr,
        body: Box<Stmt>,
    },
    For {
        initializer: Option<Box<Stmt>>,
        condition: Option<Expr>,
        step: Option<Expr>,
        body: Box<Stmt>,
    },
    Break,
    Continue,
    Empty,
}

#[derive(Debug, Clone)]
pub struct Expr {
    pub kind: ExprKind,
    pub ty: Type,
}

#[derive(Debug, Clone)]
pub enum ExprKind {
    Literal(Literal),
    Load(LValue),
    Address(LValue),
    Call {
        function: usize,
        arguments: Vec<Expr>,
    },
    Print {
        value: Box<Expr>,
        newline: bool,
    },
    Unary {
        op: UnaryOp,
        value: Box<Expr>,
    },
    Binary {
        op: BinaryOp,
        left: Box<Expr>,
        right: Box<Expr>,
    },
    Cast(Box<Expr>),
    Assign {
        target: LValue,
        value: Box<Expr>,
        op: Option<BinaryOp>,
    },
    Update {
        target: LValue,
        delta: i32,
        prefix: bool,
    },
}

#[derive(Debug, Clone)]
pub struct LValue {
    pub kind: LValueKind,
    pub ty: Type,
}

#[derive(Debug, Clone)]
pub enum LValueKind {
    Local(usize),
    Dereference(Box<Expr>),
}
