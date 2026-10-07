use crate::diagnostic::Span;
use std::fmt;

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Type {
    Int,
    Long,
    Double,
    Bool,
    String,
    Void,
    Pointer(Box<Type>),
}

impl Type {
    pub fn is_numeric(&self) -> bool {
        matches!(self, Self::Int | Self::Long | Self::Double)
    }

    pub fn is_integer(&self) -> bool {
        matches!(self, Self::Int | Self::Long)
    }

    pub fn llvm(&self) -> &'static str {
        match self {
            Self::Int => "i32",
            Self::Long => "i64",
            Self::Double => "double",
            Self::Bool => "i1",
            Self::String | Self::Pointer(_) => "ptr",
            Self::Void => "void",
        }
    }
}

impl fmt::Display for Type {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Pointer(inner) => write!(formatter, "{inner}*"),
            _ => formatter.write_str(match self {
                Self::Int => "int",
                Self::Long => "long",
                Self::Double => "double",
                Self::Bool => "bool",
                Self::String => "string",
                Self::Void => "void",
                Self::Pointer(_) => unreachable!(),
            }),
        }
    }
}

#[derive(Debug, Clone)]
pub struct Program {
    pub functions: Vec<Function>,
}

#[derive(Debug, Clone)]
pub struct Function {
    pub name: String,
    pub container: Option<String>,
    pub return_type: Type,
    pub parameters: Vec<Parameter>,
    pub body: Option<Stmt>,
    pub is_unsafe: bool,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct Parameter {
    pub name: String,
    pub ty: Type,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct Stmt {
    pub kind: StmtKind,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub enum StmtKind {
    Block(Vec<Stmt>),
    Unsafe(Box<Stmt>),
    Let {
        name: String,
        ty: Option<Type>,
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
    pub span: Span,
}

#[derive(Debug, Clone)]
pub enum Literal {
    Int(i32),
    Long(i64),
    Double(f64),
    Bool(bool),
    String(String),
}

#[derive(Debug, Clone)]
pub enum ExprKind {
    Literal(Literal),
    Name(String),
    Call {
        name: String,
        arguments: Vec<Expr>,
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
    Assign {
        target: Box<Expr>,
        value: Box<Expr>,
        op: Option<BinaryOp>,
    },
    Update {
        target: Box<Expr>,
        delta: i32,
        prefix: bool,
    },
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum UnaryOp {
    Negate,
    Positive,
    Not,
    Address,
    Dereference,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BinaryOp {
    Add,
    Subtract,
    Multiply,
    Divide,
    Remainder,
    Equal,
    NotEqual,
    Less,
    LessEqual,
    Greater,
    GreaterEqual,
    And,
    Or,
}
