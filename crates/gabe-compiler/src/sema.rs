use crate::ast::{self, BinaryOp, Type, UnaryOp};
use crate::diagnostic::{Diagnostic, Result, Span};
use crate::hir::{self, Expr, ExprKind, LValue, LValueKind, Stmt};
use std::collections::HashMap;

#[derive(Clone)]
struct Signature {
    id: usize,
    return_type: Type,
    parameters: Vec<Type>,
    is_unsafe: bool,
}

pub fn check(program: ast::Program) -> Result<hir::Program> {
    let mut signatures = HashMap::new();
    let mut entry = None;
    for (id, function) in program.functions.iter().enumerate() {
        if function.name.starts_with("Console.") || function.name.starts_with("System.Console.") {
            return Err(Diagnostic::new(
                function.span,
                "Console is reserved for built-in output",
            ));
        }
        if function.body.is_none() && function.container.is_some() {
            return Err(Diagnostic::new(
                function.span,
                "declare extern C functions at file scope",
            ));
        }
        for parameter in &function.parameters {
            if parameter.ty == Type::Void {
                return Err(Diagnostic::new(
                    parameter.span,
                    "parameters cannot have type void",
                ));
            }
        }
        if function.body.is_none() {
            // Aggregate and narrow-integer C ABI lowering is deliberately deferred.
            if function.return_type == Type::Bool
                || function.parameters.iter().any(|p| p.ty == Type::Bool)
            {
                return Err(Diagnostic::new(
                    function.span,
                    "extern bool parameters and results are not supported; use int for C ABI flags",
                ));
            }
            if function.name.starts_with("gb_")
                || matches!(function.name.as_str(), "main" | "printf" | "puts")
            {
                return Err(Diagnostic::new(
                    function.span,
                    "this external symbol is reserved by the runtime",
                ));
            }
        }
        let signature = Signature {
            id,
            return_type: function.return_type.clone(),
            parameters: function
                .parameters
                .iter()
                .map(|parameter| parameter.ty.clone())
                .collect(),
            is_unsafe: function.is_unsafe || function.body.is_none(),
        };
        if signatures
            .insert(function.name.clone(), signature)
            .is_some()
        {
            return Err(Diagnostic::new(
                function.span,
                format!(
                    "duplicate function '{}'; overloading is not supported",
                    function.name
                ),
            ));
        }
        if function.name.rsplit('.').next() == Some("Main") && function.body.is_some() {
            if entry.replace(id).is_some() {
                return Err(Diagnostic::new(
                    function.span,
                    "a program must have exactly one Main function",
                ));
            }
            if !function.parameters.is_empty()
                || !matches!(function.return_type, Type::Int | Type::Void)
                || function.is_unsafe
            {
                return Err(Diagnostic::new(
                    function.span,
                    "entry point must be a safe int Main() or void Main()",
                ));
            }
        }
    }
    let entry = entry.ok_or_else(|| {
        Diagnostic::new(
            Span::default(),
            "missing entry point: declare int Main() or static int Main() inside a class",
        )
    })?;
    let mut functions = Vec::new();
    for (id, function) in program.functions.iter().enumerate() {
        let mut checker = Checker {
            signatures: &signatures,
            function,
            scopes: vec![HashMap::new()],
            variables: Vec::new(),
            unsafe_depth: usize::from(function.is_unsafe || function.body.is_none()),
            loop_depth: 0,
        };
        checker.validate_type(&function.return_type, function.span)?;
        let mut parameters = Vec::new();
        for parameter in &function.parameters {
            checker.validate_type(&parameter.ty, parameter.span)?;
            parameters.push(checker.declare(
                &parameter.name,
                parameter.ty.clone(),
                parameter.span,
            )?);
        }
        let body = if let Some(body) = &function.body {
            let (body, terminates) = checker.statement(body)?;
            if function.return_type != Type::Void && !terminates {
                return Err(Diagnostic::new(
                    function.span,
                    format!("not all paths in '{}' return a value", function.name),
                ));
            }
            Some(body)
        } else {
            None
        };
        functions.push(hir::Function {
            name: function.name.clone(),
            symbol: if function.body.is_none() {
                function.name.clone()
            } else {
                format!("gb_{id}")
            },
            return_type: function.return_type.clone(),
            parameters,
            variables: checker.variables,
            body,
        });
    }
    Ok(hir::Program { functions, entry })
}

struct Checker<'a> {
    signatures: &'a HashMap<String, Signature>,
    function: &'a ast::Function,
    scopes: Vec<HashMap<String, usize>>,
    variables: Vec<Type>,
    unsafe_depth: usize,
    loop_depth: usize,
}

impl Checker<'_> {
    fn require_unsafe(&self, span: Span) -> Result<()> {
        if self.unsafe_depth == 0 {
            Err(Diagnostic::new(
                span,
                "pointer operations and extern/unsafe calls require an unsafe block or unsafe function",
            ))
        } else {
            Ok(())
        }
    }
    fn validate_type(&self, ty: &Type, span: Span) -> Result<()> {
        if let Type::Pointer(_) = ty {
            self.require_unsafe(span)?;
        }
        Ok(())
    }
    fn declare(&mut self, name: &str, ty: Type, span: Span) -> Result<usize> {
        let scope = self.scopes.last_mut().expect("a scope exists");
        if scope.contains_key(name) {
            return Err(Diagnostic::new(span, format!("duplicate local '{name}'")));
        }
        let id = self.variables.len();
        self.variables.push(ty);
        scope.insert(name.to_owned(), id);
        Ok(id)
    }
    fn lookup(&self, name: &str, span: Span) -> Result<usize> {
        self.scopes
            .iter()
            .rev()
            .find_map(|scope| scope.get(name).copied())
            .ok_or_else(|| Diagnostic::new(span, format!("unknown variable '{name}'")))
    }
    fn scoped_statement(&mut self, statement: &ast::Stmt) -> Result<(Stmt, bool)> {
        self.scopes.push(HashMap::new());
        let result = self.statement(statement);
        self.scopes.pop();
        result
    }
    fn statement(&mut self, statement: &ast::Stmt) -> Result<(Stmt, bool)> {
        let span = statement.span;
        let mut terminates = false;
        let result = match &statement.kind {
            ast::StmtKind::Block(statements) => {
                self.scopes.push(HashMap::new());
                let mut checked = Vec::new();
                for statement in statements {
                    let (statement, ends) = self.statement(statement)?;
                    terminates |= ends;
                    checked.push(statement);
                }
                self.scopes.pop();
                Stmt::Block(checked)
            }
            ast::StmtKind::Unsafe(body) => {
                self.unsafe_depth += 1;
                let result = self.scoped_statement(body);
                self.unsafe_depth -= 1;
                return result;
            }
            ast::StmtKind::Let { name, ty, value } => {
                let value_span = value.span;
                let value = self.expression(value)?;
                let ty = ty.as_ref().unwrap_or(&value.ty).clone();
                self.validate_type(&ty, span)?;
                if ty == Type::Void {
                    return Err(Diagnostic::new(span, "a variable cannot have type void"));
                }
                let value = coerce(value, &ty, value_span)?;
                let variable = self.declare(name, ty, span)?;
                Stmt::Let { variable, value }
            }
            ast::StmtKind::Expr(expression) => {
                if !matches!(
                    expression.kind,
                    ast::ExprKind::Call { .. }
                        | ast::ExprKind::Assign { .. }
                        | ast::ExprKind::Update { .. }
                ) {
                    return Err(Diagnostic::new(
                        expression.span,
                        "an expression statement must be a call, assignment, or increment",
                    ));
                }
                Stmt::Expr(self.expression(expression)?)
            }
            ast::StmtKind::Return(value) => {
                terminates = true;
                let value = match (value, &self.function.return_type) {
                    (None, Type::Void) => None,
                    (Some(_), Type::Void) => {
                        return Err(Diagnostic::new(
                            span,
                            "a void function cannot return a value",
                        ));
                    }
                    (None, ty) => {
                        return Err(Diagnostic::new(
                            span,
                            format!("expected a return value of type {ty}"),
                        ));
                    }
                    (Some(value), ty) => Some(coerce(self.expression(value)?, ty, value.span)?),
                };
                Stmt::Return(value)
            }
            ast::StmtKind::If {
                condition,
                then_body,
                else_body,
            } => {
                let condition = coerce(self.expression(condition)?, &Type::Bool, condition.span)?;
                let (then_body, then_ends) = self.scoped_statement(then_body)?;
                let else_body = if let Some(body) = else_body {
                    let (body, else_ends) = self.scoped_statement(body)?;
                    terminates = then_ends && else_ends;
                    Some(Box::new(body))
                } else {
                    None
                };
                Stmt::If {
                    condition,
                    then_body: Box::new(then_body),
                    else_body,
                }
            }
            ast::StmtKind::While { condition, body } => {
                let condition = coerce(self.expression(condition)?, &Type::Bool, condition.span)?;
                self.loop_depth += 1;
                let result = self.scoped_statement(body);
                self.loop_depth -= 1;
                let (body, _) = result?;
                Stmt::While {
                    condition,
                    body: Box::new(body),
                }
            }
            ast::StmtKind::For {
                initializer,
                condition,
                step,
                body,
            } => {
                self.scopes.push(HashMap::new());
                let initializer = initializer
                    .as_ref()
                    .map(|s| self.statement(s).map(|(s, _)| Box::new(s)))
                    .transpose()?;
                let condition = condition
                    .as_ref()
                    .map(|e| coerce(self.expression(e)?, &Type::Bool, e.span))
                    .transpose()?;
                let step = step.as_ref().map(|e| self.expression(e)).transpose()?;
                self.loop_depth += 1;
                let result = self.scoped_statement(body);
                self.loop_depth -= 1;
                let (body, _) = result?;
                self.scopes.pop();
                Stmt::For {
                    initializer,
                    condition,
                    step,
                    body: Box::new(body),
                }
            }
            ast::StmtKind::Break | ast::StmtKind::Continue => {
                if self.loop_depth == 0 {
                    return Err(Diagnostic::new(
                        span,
                        "break and continue are only valid inside a loop",
                    ));
                }
                terminates = true;
                if matches!(statement.kind, ast::StmtKind::Break) {
                    Stmt::Break
                } else {
                    Stmt::Continue
                }
            }
            ast::StmtKind::Empty => Stmt::Empty,
        };
        Ok((result, terminates))
    }
    fn lvalue(&mut self, expression: &ast::Expr) -> Result<LValue> {
        match &expression.kind {
            ast::ExprKind::Name(name) => {
                let id = self.lookup(name, expression.span)?;
                let ty = self.variables[id].clone();
                self.validate_type(&ty, expression.span)?;
                Ok(LValue {
                    kind: LValueKind::Local(id),
                    ty,
                })
            }
            ast::ExprKind::Unary {
                op: UnaryOp::Dereference,
                value,
            } => {
                self.require_unsafe(expression.span)?;
                let pointer = self.expression(value)?;
                let Type::Pointer(inner) = &pointer.ty else {
                    return Err(Diagnostic::new(
                        expression.span,
                        "dereferencing requires a pointer",
                    ));
                };
                if **inner == Type::Void {
                    return Err(Diagnostic::new(
                        expression.span,
                        "cannot dereference a void pointer",
                    ));
                }
                Ok(LValue {
                    ty: (**inner).clone(),
                    kind: LValueKind::Dereference(Box::new(pointer)),
                })
            }
            _ => Err(Diagnostic::new(
                expression.span,
                "expected a variable or dereferenced pointer",
            )),
        }
    }
    fn expression(&mut self, expression: &ast::Expr) -> Result<Expr> {
        let span = expression.span;
        let (kind, ty) = match &expression.kind {
            ast::ExprKind::Literal(literal) => {
                let ty = match literal {
                    ast::Literal::Int(_) => Type::Int,
                    ast::Literal::Long(_) => Type::Long,
                    ast::Literal::Double(_) => Type::Double,
                    ast::Literal::Bool(_) => Type::Bool,
                    ast::Literal::String(_) => Type::String,
                };
                (ExprKind::Literal(literal.clone()), ty)
            }
            ast::ExprKind::Name(_) => {
                let value = self.lvalue(expression)?;
                let ty = value.ty.clone();
                (ExprKind::Load(value), ty)
            }
            ast::ExprKind::Call { name, arguments } => {
                if matches!(
                    name.as_str(),
                    "Console.WriteLine"
                        | "System.Console.WriteLine"
                        | "Console.Write"
                        | "System.Console.Write"
                ) {
                    let newline = name.ends_with("WriteLine");
                    if arguments.len() > 1 || (arguments.is_empty() && !newline) {
                        return Err(Diagnostic::new(
                            span,
                            "Console.Write takes one value; Console.WriteLine takes zero or one",
                        ));
                    }
                    let value = if let Some(value) = arguments.first() {
                        self.expression(value)?
                    } else {
                        Expr {
                            kind: ExprKind::Literal(ast::Literal::String(String::new())),
                            ty: Type::String,
                        }
                    };
                    if !matches!(
                        value.ty,
                        Type::Int | Type::Long | Type::Double | Type::Bool | Type::String
                    ) {
                        return Err(Diagnostic::new(
                            span,
                            "Console output supports int, long, double, bool, and string",
                        ));
                    }
                    (
                        ExprKind::Print {
                            value: Box::new(value),
                            newline,
                        },
                        Type::Void,
                    )
                } else {
                    let qualified = self
                        .function
                        .container
                        .as_ref()
                        .map(|class| format!("{class}.{name}"));
                    let signature = qualified
                        .as_ref()
                        .and_then(|name| self.signatures.get(name))
                        .or_else(|| self.signatures.get(name))
                        .ok_or_else(|| Diagnostic::new(span, format!("unknown function '{name}'")))?
                        .clone();
                    if signature.is_unsafe {
                        self.require_unsafe(span)?;
                    }
                    if signature.parameters.len() != arguments.len() {
                        return Err(Diagnostic::new(
                            span,
                            format!(
                                "'{name}' expects {} arguments, got {}",
                                signature.parameters.len(),
                                arguments.len()
                            ),
                        ));
                    }
                    let mut checked = Vec::new();
                    for (argument, ty) in arguments.iter().zip(&signature.parameters) {
                        checked.push(coerce(self.expression(argument)?, ty, argument.span)?);
                    }
                    (
                        ExprKind::Call {
                            function: signature.id,
                            arguments: checked,
                        },
                        signature.return_type,
                    )
                }
            }
            ast::ExprKind::Unary {
                op: UnaryOp::Address,
                value,
            } => {
                self.require_unsafe(span)?;
                let value = self.lvalue(value)?;
                let ty = Type::Pointer(Box::new(value.ty.clone()));
                (ExprKind::Address(value), ty)
            }
            ast::ExprKind::Unary {
                op: UnaryOp::Dereference,
                ..
            } => {
                let value = self.lvalue(expression)?;
                let ty = value.ty.clone();
                (ExprKind::Load(value), ty)
            }
            ast::ExprKind::Unary { op, value } => {
                let value = self.expression(value)?;
                if (*op == UnaryOp::Not && value.ty != Type::Bool)
                    || (*op != UnaryOp::Not && !value.ty.is_numeric())
                {
                    return Err(Diagnostic::new(
                        span,
                        "unary ! requires bool; unary + and - require a number",
                    ));
                }
                let ty = value.ty.clone();
                (
                    ExprKind::Unary {
                        op: *op,
                        value: Box::new(value),
                    },
                    ty,
                )
            }
            ast::ExprKind::Binary { op, left, right } => {
                let left = self.expression(left)?;
                let right = self.expression(right)?;
                return binary(*op, left, right, span);
            }
            ast::ExprKind::Assign { target, value, op } => {
                let target = self.lvalue(target)?;
                let value_span = value.span;
                let value = self.expression(value)?;
                if op.is_some() && (!target.ty.is_numeric() || !value.ty.is_numeric()) {
                    return Err(Diagnostic::new(
                        span,
                        "compound assignment requires numbers",
                    ));
                }
                let value = coerce(value, &target.ty, value_span)?;
                let ty = target.ty.clone();
                (
                    ExprKind::Assign {
                        target,
                        value: Box::new(value),
                        op: *op,
                    },
                    ty,
                )
            }
            ast::ExprKind::Update {
                target,
                delta,
                prefix,
            } => {
                let target = self.lvalue(target)?;
                if !target.ty.is_numeric() {
                    return Err(Diagnostic::new(
                        span,
                        "increment and decrement require a number",
                    ));
                }
                let ty = target.ty.clone();
                (
                    ExprKind::Update {
                        target,
                        delta: *delta,
                        prefix: *prefix,
                    },
                    ty,
                )
            }
        };
        Ok(Expr { kind, ty })
    }
}

fn coerce(expression: Expr, target: &Type, span: Span) -> Result<Expr> {
    if &expression.ty == target {
        return Ok(expression);
    }
    if matches!(
        (&expression.ty, target),
        (Type::Int, Type::Long | Type::Double) | (Type::Long, Type::Double)
    ) {
        return Ok(Expr {
            kind: ExprKind::Cast(Box::new(expression)),
            ty: target.clone(),
        });
    }
    Err(Diagnostic::new(
        span,
        format!("cannot convert {} to {target}", expression.ty),
    ))
}

fn binary(op: BinaryOp, left: Expr, right: Expr, span: Span) -> Result<Expr> {
    use BinaryOp::*;
    let logical = matches!(op, And | Or);
    let equality = matches!(op, Equal | NotEqual);
    let comparison = matches!(
        op,
        Equal | NotEqual | Less | LessEqual | Greater | GreaterEqual
    );
    let (left, right) = if left.ty.is_numeric() && right.ty.is_numeric() && !logical {
        let ty = if left.ty == Type::Double || right.ty == Type::Double {
            Type::Double
        } else if left.ty == Type::Long || right.ty == Type::Long {
            Type::Long
        } else {
            Type::Int
        };
        (coerce(left, &ty, span)?, coerce(right, &ty, span)?)
    } else if left.ty == right.ty
        && ((left.ty == Type::Bool && (logical || equality))
            || (matches!(left.ty, Type::Pointer(_)) && equality))
    {
        (left, right)
    } else {
        return Err(Diagnostic::new(
            span,
            format!(
                "operator {op:?} does not support {} and {}",
                left.ty, right.ty
            ),
        ));
    };
    let ty = if logical || comparison {
        Type::Bool
    } else {
        left.ty.clone()
    };
    Ok(Expr {
        kind: ExprKind::Binary {
            op,
            left: Box::new(left),
            right: Box::new(right),
        },
        ty,
    })
}
