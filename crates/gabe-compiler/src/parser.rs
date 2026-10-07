use crate::ast::*;
use crate::diagnostic::{Diagnostic, Result, Span};
use crate::lexer::{Kind, Token};

pub fn parse(tokens: Vec<Token>) -> Result<Program> {
    Parser {
        tokens,
        position: 0,
    }
    .program()
}

struct Parser {
    tokens: Vec<Token>,
    position: usize,
}

impl Parser {
    fn current(&self) -> &Token {
        &self.tokens[self.position]
    }
    fn at(&self, text: &str) -> bool {
        matches!(&self.current().kind, Kind::Word(value) | Kind::Symbol(value) if value == text)
    }
    fn take(&mut self, text: &str) -> bool {
        if self.at(text) {
            self.position += 1;
            true
        } else {
            false
        }
    }
    fn expect(&mut self, text: &str) -> Result<Span> {
        let span = self.current().span;
        if self.take(text) {
            Ok(span)
        } else {
            Err(Diagnostic::new(span, format!("expected '{text}'")))
        }
    }
    fn name(&mut self) -> Result<(String, Span)> {
        if let Kind::Word(name) = &self.current().kind {
            let result = (name.clone(), self.current().span);
            if matches!(
                name.as_str(),
                "class"
                    | "static"
                    | "public"
                    | "private"
                    | "internal"
                    | "unsafe"
                    | "extern"
                    | "using"
                    | "namespace"
                    | "return"
                    | "if"
                    | "else"
                    | "while"
                    | "for"
                    | "break"
                    | "continue"
                    | "true"
                    | "false"
                    | "var"
                    | "void"
                    | "int"
                    | "long"
                    | "double"
                    | "bool"
                    | "string"
            ) {
                return Err(Diagnostic::new(
                    result.1,
                    "expected an identifier, found a keyword",
                ));
            }
            self.position += 1;
            Ok(result)
        } else {
            Err(Diagnostic::new(
                self.current().span,
                "expected an identifier",
            ))
        }
    }
    fn modifiers(&mut self) -> (bool, bool, bool) {
        let (mut is_unsafe, mut is_extern, mut is_static) = (false, false, false);
        loop {
            if self.take("unsafe") {
                is_unsafe = true;
            } else if self.take("extern") {
                is_extern = true;
            } else if self.take("static") {
                is_static = true;
            } else if self.take("public") || self.take("private") || self.take("internal") {
            } else {
                break;
            }
        }
        (is_unsafe, is_extern, is_static)
    }
    fn ty(&mut self) -> Result<Type> {
        let mut ty = if self.take("int") {
            Type::Int
        } else if self.take("long") {
            Type::Long
        } else if self.take("double") {
            Type::Double
        } else if self.take("bool") {
            Type::Bool
        } else if self.take("string") {
            Type::String
        } else if self.take("void") {
            Type::Void
        } else {
            return Err(Diagnostic::new(
                self.current().span,
                "expected a type (int, long, double, bool, string, or void)",
            ));
        };
        while self.take("*") {
            ty = Type::Pointer(Box::new(ty));
        }
        Ok(ty)
    }
    fn program(&mut self) -> Result<Program> {
        let mut functions = Vec::new();
        while self.take("using") {
            let (name, span) = self.name()?;
            if name != "System" {
                return Err(Diagnostic::new(
                    span,
                    "only 'using System;' is supported in v0.1",
                ));
            }
            self.expect(";")?;
        }
        while self.current().kind != Kind::Eof {
            let start = self.current().span;
            let modifiers = self.modifiers();
            if self.take("class") {
                if modifiers.1 {
                    return Err(Diagnostic::new(start, "a class cannot be extern"));
                }
                let (container, _) = self.name()?;
                self.expect("{")?;
                while !self.take("}") {
                    let start = self.current().span;
                    let mut inner_modifiers = self.modifiers();
                    if !inner_modifiers.2 {
                        return Err(Diagnostic::new(
                            start,
                            "class methods must be static in v0.1",
                        ));
                    }
                    inner_modifiers.0 |= modifiers.0;
                    functions.push(self.function(
                        Some(container.clone()),
                        inner_modifiers,
                        start,
                    )?);
                }
            } else {
                functions.push(self.function(None, modifiers, start)?);
            }
        }
        Ok(Program { functions })
    }
    fn function(
        &mut self,
        container: Option<String>,
        modifiers: (bool, bool, bool),
        start: Span,
    ) -> Result<Function> {
        let return_type = self.ty()?;
        let (short_name, _) = self.name()?;
        let name = container.as_ref().map_or_else(
            || short_name.clone(),
            |class| format!("{class}.{short_name}"),
        );
        self.expect("(")?;
        let mut parameters = Vec::new();
        if !self.take(")") {
            loop {
                let span = self.current().span;
                let ty = self.ty()?;
                let (name, end) = self.name()?;
                parameters.push(Parameter {
                    name,
                    ty,
                    span: span.join(end),
                });
                if self.take(")") {
                    break;
                }
                self.expect(",")?;
            }
        }
        let body = if modifiers.1 {
            self.expect(";")?;
            None
        } else {
            Some(self.block()?)
        };
        let end = self.tokens[self.position - 1].span;
        Ok(Function {
            name,
            container,
            return_type,
            parameters,
            body,
            is_unsafe: modifiers.0,
            span: start.join(end),
        })
    }
    fn block(&mut self) -> Result<Stmt> {
        let start = self.expect("{")?;
        let mut statements = Vec::new();
        while !self.at("}") {
            if self.current().kind == Kind::Eof {
                return Err(Diagnostic::new(
                    self.current().span,
                    "expected '}' before end of file",
                ));
            }
            statements.push(self.statement()?);
        }
        let end = self.expect("}")?;
        Ok(Stmt {
            kind: StmtKind::Block(statements),
            span: start.join(end),
        })
    }
    fn statement(&mut self) -> Result<Stmt> {
        let start = self.current().span;
        let kind = if self.at("{") {
            return self.block();
        } else if self.take("unsafe") {
            StmtKind::Unsafe(Box::new(self.block()?))
        } else if self.take("return") {
            let value = if self.at(";") {
                None
            } else {
                Some(self.expression()?)
            };
            self.expect(";")?;
            StmtKind::Return(value)
        } else if self.take("if") {
            self.expect("(")?;
            let condition = self.expression()?;
            self.expect(")")?;
            let then_body = Box::new(self.statement()?);
            let else_body = if self.take("else") {
                Some(Box::new(self.statement()?))
            } else {
                None
            };
            StmtKind::If {
                condition,
                then_body,
                else_body,
            }
        } else if self.take("while") {
            self.expect("(")?;
            let condition = self.expression()?;
            self.expect(")")?;
            StmtKind::While {
                condition,
                body: Box::new(self.statement()?),
            }
        } else if self.take("for") {
            self.expect("(")?;
            let initializer = if self.take(";") {
                None
            } else {
                Some(Box::new(self.simple_statement()?))
            };
            let condition = if self.at(";") {
                None
            } else {
                Some(self.expression()?)
            };
            self.expect(";")?;
            let step = if self.at(")") {
                None
            } else {
                Some(self.expression()?)
            };
            self.expect(")")?;
            StmtKind::For {
                initializer,
                condition,
                step,
                body: Box::new(self.statement()?),
            }
        } else if self.take("break") {
            self.expect(";")?;
            StmtKind::Break
        } else if self.take("continue") {
            self.expect(";")?;
            StmtKind::Continue
        } else if self.take(";") {
            StmtKind::Empty
        } else {
            return self.simple_statement();
        };
        Ok(Stmt {
            kind,
            span: start.join(self.tokens[self.position - 1].span),
        })
    }
    fn simple_statement(&mut self) -> Result<Stmt> {
        let start = self.current().span;
        let kind = if ["var", "int", "long", "double", "bool", "string", "void"]
            .iter()
            .any(|ty| self.at(ty))
        {
            let ty = if self.take("var") {
                None
            } else {
                Some(self.ty()?)
            };
            let (name, _) = self.name()?;
            self.expect("=")?;
            StmtKind::Let {
                name,
                ty,
                value: self.expression()?,
            }
        } else {
            StmtKind::Expr(self.expression()?)
        };
        let end = self.expect(";")?;
        Ok(Stmt {
            kind,
            span: start.join(end),
        })
    }
    fn expression(&mut self) -> Result<Expr> {
        let target = self.binary(1)?;
        let op = if self.take("=") {
            Some(None)
        } else if self.take("+=") {
            Some(Some(BinaryOp::Add))
        } else if self.take("-=") {
            Some(Some(BinaryOp::Subtract))
        } else if self.take("*=") {
            Some(Some(BinaryOp::Multiply))
        } else if self.take("/=") {
            Some(Some(BinaryOp::Divide))
        } else if self.take("%=") {
            Some(Some(BinaryOp::Remainder))
        } else {
            None
        };
        if let Some(op) = op {
            let value = self.expression()?;
            let span = target.span.join(value.span);
            Ok(Expr {
                kind: ExprKind::Assign {
                    target: Box::new(target),
                    value: Box::new(value),
                    op,
                },
                span,
            })
        } else {
            Ok(target)
        }
    }
    fn binary_operator(&self) -> Option<(BinaryOp, u8)> {
        let Kind::Symbol(symbol) = &self.current().kind else {
            return None;
        };
        Some(match symbol.as_str() {
            "||" => (BinaryOp::Or, 1),
            "&&" => (BinaryOp::And, 2),
            "==" => (BinaryOp::Equal, 3),
            "!=" => (BinaryOp::NotEqual, 3),
            "<" => (BinaryOp::Less, 4),
            "<=" => (BinaryOp::LessEqual, 4),
            ">" => (BinaryOp::Greater, 4),
            ">=" => (BinaryOp::GreaterEqual, 4),
            "+" => (BinaryOp::Add, 5),
            "-" => (BinaryOp::Subtract, 5),
            "*" => (BinaryOp::Multiply, 6),
            "/" => (BinaryOp::Divide, 6),
            "%" => (BinaryOp::Remainder, 6),
            _ => return None,
        })
    }
    fn binary(&mut self, minimum: u8) -> Result<Expr> {
        let mut left = self.unary()?;
        while let Some((op, precedence)) = self.binary_operator() {
            if precedence < minimum {
                break;
            }
            self.position += 1;
            let right = self.binary(precedence + 1)?;
            let span = left.span.join(right.span);
            left = Expr {
                kind: ExprKind::Binary {
                    op,
                    left: Box::new(left),
                    right: Box::new(right),
                },
                span,
            };
        }
        Ok(left)
    }
    fn unary(&mut self) -> Result<Expr> {
        let start = self.current().span;
        if self.take("++") || self.take("--") {
            let delta = if self.tokens[self.position - 1].kind == Kind::Symbol("++".to_owned()) {
                1
            } else {
                -1
            };
            let target = self.unary()?;
            let span = start.join(target.span);
            return Ok(Expr {
                kind: ExprKind::Update {
                    target: Box::new(target),
                    delta,
                    prefix: true,
                },
                span,
            });
        }
        let op = if self.take("-") {
            Some(UnaryOp::Negate)
        } else if self.take("+") {
            Some(UnaryOp::Positive)
        } else if self.take("!") {
            Some(UnaryOp::Not)
        } else if self.take("&") {
            Some(UnaryOp::Address)
        } else if self.take("*") {
            Some(UnaryOp::Dereference)
        } else {
            None
        };
        if let Some(op) = op {
            if op == UnaryOp::Negate {
                if let Kind::Number(number) = &self.current().kind {
                    let number = format!("-{number}");
                    let span = start.join(self.current().span);
                    self.position += 1;
                    return Ok(Expr {
                        kind: ExprKind::Literal(parse_number(&number, span)?),
                        span,
                    });
                }
            }
            let value = self.unary()?;
            let span = start.join(value.span);
            return Ok(Expr {
                kind: ExprKind::Unary {
                    op,
                    value: Box::new(value),
                },
                span,
            });
        }
        let mut value = self.primary()?;
        if self.take("++") || self.take("--") {
            let token = &self.tokens[self.position - 1];
            let delta = if token.kind == Kind::Symbol("++".to_owned()) {
                1
            } else {
                -1
            };
            let span = value.span.join(token.span);
            value = Expr {
                kind: ExprKind::Update {
                    target: Box::new(value),
                    delta,
                    prefix: false,
                },
                span,
            };
        }
        Ok(value)
    }
    fn primary(&mut self) -> Result<Expr> {
        let token = self.current().clone();
        if self.take("(") {
            let mut expression = self.expression()?;
            expression.span = token.span.join(self.expect(")")?);
            return Ok(expression);
        }
        let kind = match token.kind {
            Kind::Number(number) => {
                self.position += 1;
                ExprKind::Literal(parse_number(&number, token.span)?)
            }
            Kind::String(value) => {
                self.position += 1;
                ExprKind::Literal(Literal::String(value))
            }
            Kind::Word(ref value) if matches!(value.as_str(), "true" | "false") => {
                self.position += 1;
                ExprKind::Literal(Literal::Bool(value == "true"))
            }
            Kind::Word(_) => {
                let (mut name, _) = self.name()?;
                while self.take(".") {
                    name.push('.');
                    name.push_str(&self.name()?.0);
                }
                if self.take("(") {
                    let mut arguments = Vec::new();
                    if !self.take(")") {
                        loop {
                            arguments.push(self.expression()?);
                            if self.take(")") {
                                break;
                            }
                            self.expect(",")?;
                        }
                    }
                    ExprKind::Call { name, arguments }
                } else {
                    ExprKind::Name(name)
                }
            }
            _ => return Err(Diagnostic::new(token.span, "expected an expression")),
        };
        Ok(Expr {
            kind,
            span: token.span.join(self.tokens[self.position - 1].span),
        })
    }
}

fn parse_number(number: &str, span: Span) -> Result<Literal> {
    let long = number.ends_with(['l', 'L']);
    let digits = if long {
        &number[..number.len() - 1]
    } else {
        number
    };
    if digits.contains(['.', 'e', 'E']) {
        if long {
            return Err(Diagnostic::new(
                span,
                "a floating-point literal cannot have the L suffix",
            ));
        }
        let value = digits
            .parse::<f64>()
            .map_err(|_| Diagnostic::new(span, "invalid double literal"))?;
        if !value.is_finite() {
            return Err(Diagnostic::new(span, "double literal is out of range"));
        }
        return Ok(Literal::Double(value));
    }
    let value = digits
        .parse::<i64>()
        .map_err(|_| Diagnostic::new(span, "integer literal is outside the signed 64-bit range"))?;
    if !long {
        if let Ok(value) = i32::try_from(value) {
            return Ok(Literal::Int(value));
        }
    }
    Ok(Literal::Long(value))
}
