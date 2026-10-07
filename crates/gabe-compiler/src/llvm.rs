use crate::ast::{BinaryOp, Literal, Type, UnaryOp};
use crate::hir::{Expr, ExprKind, Function, LValue, LValueKind, Program, Stmt};
use std::collections::HashMap;
use std::fmt::Write;

pub fn emit(program: &Program, triple: &str) -> String {
    let mut module = Module {
        program,
        strings: Vec::new(),
        interned: HashMap::new(),
    };
    let mut functions = String::new();
    for function in &program.functions {
        if function.body.is_some() {
            functions.push_str(&FunctionEmitter::new(&mut module, function).emit());
        } else {
            let parameters = function
                .parameters
                .iter()
                .map(|id| function.variables[*id].llvm())
                .collect::<Vec<_>>()
                .join(", ");
            writeln!(
                functions,
                "declare {} @{}({parameters})\n",
                function.return_type.llvm(),
                function.symbol
            )
            .unwrap();
        }
    }
    let entry = &program.functions[program.entry];
    if entry.return_type == Type::Void {
        writeln!(
            functions,
            "define i32 @main() {{\nentry:\n  call void @{}()\n  ret i32 0\n}}",
            entry.symbol
        )
        .unwrap();
    } else {
        writeln!(
            functions,
            "define i32 @main() {{\nentry:\n  %result = call i32 @{}()\n  ret i32 %result\n}}",
            entry.symbol
        )
        .unwrap();
    }
    let mut output =
        format!("; Gabe LLVM IR (opaque pointers; LLVM 17+)\ntarget triple = \"{triple}\"\n\n");
    for (id, value) in module.strings.iter().enumerate() {
        let mut encoded = String::new();
        for byte in value.as_bytes() {
            write!(encoded, "\\{byte:02X}").unwrap();
        }
        writeln!(
            output,
            "@gb_string_{id} = private unnamed_addr constant [{} x i8] c\"{encoded}\\00\"",
            value.len() + 1
        )
        .unwrap();
    }
    output.push_str(
        "\ndeclare i32 @printf(ptr, ...)\ndeclare void @llvm.trap() cold noreturn nounwind\n\n",
    );
    output.push_str(&functions);
    output
}

struct Module<'a> {
    program: &'a Program,
    strings: Vec<String>,
    interned: HashMap<String, usize>,
}

impl Module<'_> {
    fn string(&mut self, value: &str) -> String {
        let id = if let Some(id) = self.interned.get(value) {
            *id
        } else {
            let id = self.strings.len();
            self.strings.push(value.to_owned());
            self.interned.insert(value.to_owned(), id);
            id
        };
        format!(
            "getelementptr inbounds ([{} x i8], ptr @gb_string_{id}, i64 0, i64 0)",
            value.len() + 1
        )
    }
}

struct Value {
    ty: Type,
    operand: String,
}

struct FunctionEmitter<'a, 'b> {
    module: &'b mut Module<'a>,
    function: &'a Function,
    output: String,
    register: usize,
    block: usize,
    current_block: String,
    terminated: bool,
    loops: Vec<(String, String)>,
}

impl<'a, 'b> FunctionEmitter<'a, 'b> {
    fn new(module: &'b mut Module<'a>, function: &'a Function) -> Self {
        Self {
            module,
            function,
            output: String::new(),
            register: 0,
            block: 0,
            current_block: "entry".to_owned(),
            terminated: false,
            loops: Vec::new(),
        }
    }
    fn emit(mut self) -> String {
        let parameters = self
            .function
            .parameters
            .iter()
            .enumerate()
            .map(|(index, id)| format!("{} %arg{index}", self.function.variables[*id].llvm()))
            .collect::<Vec<_>>()
            .join(", ");
        writeln!(
            self.output,
            "; {}\ndefine {} @{}({parameters}) {{\nentry:",
            self.function.name,
            self.function.return_type.llvm(),
            self.function.symbol
        )
        .unwrap();
        // All slots live in entry, including loop locals, so optimizers can promote them.
        for (id, ty) in self.function.variables.iter().enumerate() {
            self.instruction(format!("%v{id} = alloca {}", ty.llvm()));
        }
        for (index, id) in self.function.parameters.iter().enumerate() {
            self.instruction(format!(
                "store {} %arg{index}, ptr %v{id}",
                self.function.variables[*id].llvm()
            ));
        }
        self.statement(self.function.body.as_ref().expect("defined function"));
        if !self.terminated {
            assert_eq!(
                self.function.return_type,
                Type::Void,
                "analysis guarantees returns"
            );
            self.instruction("ret void");
        }
        self.output.push_str("}\n\n");
        self.output
    }
    fn instruction(&mut self, instruction: impl AsRef<str>) {
        writeln!(self.output, "  {}", instruction.as_ref()).unwrap();
    }
    fn result(&mut self, instruction: impl AsRef<str>) -> String {
        let result = format!("%r{}", self.register);
        self.register += 1;
        self.instruction(format!("{result} = {}", instruction.as_ref()));
        result
    }
    fn new_block(&mut self) -> String {
        let label = format!("block_{}", self.block);
        self.block += 1;
        label
    }
    fn label(&mut self, label: &str) {
        writeln!(self.output, "{label}:").unwrap();
        self.current_block = label.to_owned();
        self.terminated = false;
    }
    fn branch(&mut self, target: &str) {
        if !self.terminated {
            self.instruction(format!("br label %{target}"));
            self.terminated = true;
        }
    }
    fn conditional(&mut self, value: &str, yes: &str, no: &str) {
        self.instruction(format!("br i1 {value}, label %{yes}, label %{no}"));
        self.terminated = true;
    }
    fn statement(&mut self, statement: &Stmt) {
        if self.terminated {
            return;
        }
        match statement {
            Stmt::Block(statements) => {
                for statement in statements {
                    self.statement(statement);
                }
            }
            Stmt::Let { variable, value } => {
                let value = self.expression(value);
                self.instruction(format!(
                    "store {} {}, ptr %v{variable}",
                    value.ty.llvm(),
                    value.operand
                ));
            }
            Stmt::Expr(expression) => {
                self.expression(expression);
            }
            Stmt::Return(value) => {
                if let Some(value) = value {
                    let value = self.expression(value);
                    self.instruction(format!("ret {} {}", value.ty.llvm(), value.operand));
                } else {
                    self.instruction("ret void");
                }
                self.terminated = true;
            }
            Stmt::If {
                condition,
                then_body,
                else_body,
            } => {
                let condition = self.expression(condition);
                let yes = self.new_block();
                let no = self.new_block();
                let end = self.new_block();
                self.conditional(
                    &condition.operand,
                    &yes,
                    if else_body.is_some() { &no } else { &end },
                );
                self.label(&yes);
                self.statement(then_body);
                let yes_ends = self.terminated;
                self.branch(&end);
                let mut no_ends = false;
                if let Some(body) = else_body {
                    self.label(&no);
                    self.statement(body);
                    no_ends = self.terminated;
                    self.branch(&end);
                }
                if yes_ends && no_ends {
                    self.terminated = true;
                } else {
                    self.label(&end);
                }
            }
            Stmt::While { condition, body } => {
                let test = self.new_block();
                let run = self.new_block();
                let end = self.new_block();
                self.branch(&test);
                self.label(&test);
                let condition = self.expression(condition);
                self.conditional(&condition.operand, &run, &end);
                self.label(&run);
                self.loops.push((end.clone(), test.clone()));
                self.statement(body);
                self.loops.pop();
                self.branch(&test);
                self.label(&end);
            }
            Stmt::For {
                initializer,
                condition,
                step,
                body,
            } => {
                if let Some(initializer) = initializer {
                    self.statement(initializer);
                }
                let test = self.new_block();
                let run = self.new_block();
                let increment = self.new_block();
                let end = self.new_block();
                self.branch(&test);
                self.label(&test);
                if let Some(condition) = condition {
                    let condition = self.expression(condition);
                    self.conditional(&condition.operand, &run, &end);
                } else {
                    self.branch(&run);
                }
                self.label(&run);
                self.loops.push((end.clone(), increment.clone()));
                self.statement(body);
                self.loops.pop();
                self.branch(&increment);
                self.label(&increment);
                if let Some(step) = step {
                    self.expression(step);
                }
                self.branch(&test);
                self.label(&end);
            }
            Stmt::Break | Stmt::Continue => {
                let (end, step) = self
                    .loops
                    .last()
                    .expect("analysis verifies loop control")
                    .clone();
                self.branch(if matches!(statement, Stmt::Break) {
                    &end
                } else {
                    &step
                });
            }
            Stmt::Empty => {}
        }
    }
    fn address(&mut self, target: &LValue) -> String {
        match &target.kind {
            LValueKind::Local(id) => format!("%v{id}"),
            LValueKind::Dereference(pointer) => self.expression(pointer).operand,
        }
    }
    fn load(&mut self, target: &LValue, address: &str) -> Value {
        Value {
            ty: target.ty.clone(),
            operand: self.result(format!("load {}, ptr {address}", target.ty.llvm())),
        }
    }
    fn expression(&mut self, expression: &Expr) -> Value {
        let operand = match &expression.kind {
            ExprKind::Literal(literal) => match literal {
                Literal::Int(value) => value.to_string(),
                Literal::Long(value) => value.to_string(),
                Literal::Double(value) => double_constant(*value),
                Literal::Bool(value) => value.to_string(),
                Literal::String(value) => self.module.string(value),
            },
            ExprKind::Load(target) => {
                let address = self.address(target);
                self.load(target, &address).operand
            }
            ExprKind::Address(target) => self.address(target),
            ExprKind::Call {
                function,
                arguments,
            } => {
                let symbol = self.module.program.functions[*function].symbol.clone();
                let arguments = arguments
                    .iter()
                    .map(|argument| {
                        let value = self.expression(argument);
                        format!("{} {}", value.ty.llvm(), value.operand)
                    })
                    .collect::<Vec<_>>()
                    .join(", ");
                let call = format!("call {} @{symbol}({arguments})", expression.ty.llvm());
                if expression.ty == Type::Void {
                    self.instruction(call);
                    String::new()
                } else {
                    self.result(call)
                }
            }
            ExprKind::Print { value, newline } => {
                let mut value = self.expression(value);
                let format = match value.ty {
                    Type::Int => "%d",
                    Type::Long => "%lld",
                    Type::Double => "%.17g",
                    Type::String => "%s",
                    Type::Bool => {
                        let yes = self.module.string("True");
                        let no = self.module.string("False");
                        value.operand = self
                            .result(format!("select i1 {}, ptr {yes}, ptr {no}", value.operand));
                        value.ty = Type::String;
                        "%s"
                    }
                    _ => unreachable!("analysis checks output types"),
                };
                let format = self
                    .module
                    .string(&format!("{format}{}", if *newline { "\n" } else { "" }));
                self.instruction(format!(
                    "call i32 (ptr, ...) @printf(ptr {format}, {} {})",
                    value.ty.llvm(),
                    value.operand
                ));
                String::new()
            }
            ExprKind::Unary { op, value } => {
                let value = self.expression(value);
                match op {
                    UnaryOp::Positive => value.operand,
                    UnaryOp::Negate if value.ty == Type::Double => {
                        self.result(format!("fneg double {}", value.operand))
                    }
                    UnaryOp::Negate => {
                        self.result(format!("sub {} 0, {}", value.ty.llvm(), value.operand))
                    }
                    UnaryOp::Not => self.result(format!("xor i1 {}, true", value.operand)),
                    _ => unreachable!("address/dereference are lowered separately"),
                }
            }
            ExprKind::Binary {
                op: BinaryOp::And | BinaryOp::Or,
                left,
                right,
            } => {
                let and = matches!(
                    expression.kind,
                    ExprKind::Binary {
                        op: BinaryOp::And,
                        ..
                    }
                );
                let left = self.expression(left);
                let origin = self.current_block.clone();
                let rhs = self.new_block();
                let end = self.new_block();
                if and {
                    self.conditional(&left.operand, &rhs, &end);
                } else {
                    self.conditional(&left.operand, &end, &rhs);
                }
                self.label(&rhs);
                let right = self.expression(right);
                let rhs_end = self.current_block.clone();
                self.branch(&end);
                self.label(&end);
                self.result(format!(
                    "phi i1 [ {}, %{origin} ], [ {}, %{rhs_end} ]",
                    if and { "false" } else { "true" },
                    right.operand
                ))
            }
            ExprKind::Binary { op, left, right } => {
                let left = self.expression(left);
                let right = self.expression(right);
                self.binary(*op, left, right)
            }
            ExprKind::Cast(value) => {
                let value = self.expression(value);
                let instruction = if expression.ty == Type::Long {
                    "sext"
                } else {
                    "sitofp"
                };
                self.result(format!(
                    "{instruction} {} {} to {}",
                    value.ty.llvm(),
                    value.operand,
                    expression.ty.llvm()
                ))
            }
            ExprKind::Assign { target, value, op } => {
                let address = self.address(target);
                let previous = op.map(|_| self.load(target, &address));
                let value = self.expression(value);
                let result = if let Some(op) = op {
                    self.binary(*op, previous.expect("compound assignment"), value)
                } else {
                    value.operand
                };
                self.instruction(format!(
                    "store {} {result}, ptr {address}",
                    target.ty.llvm()
                ));
                result
            }
            ExprKind::Update {
                target,
                delta,
                prefix,
            } => {
                let address = self.address(target);
                let old = self.load(target, &address);
                let old_operand = old.operand.clone();
                let step = Value {
                    ty: target.ty.clone(),
                    operand: if target.ty == Type::Double {
                        double_constant(f64::from(*delta))
                    } else {
                        delta.to_string()
                    },
                };
                let new = self.binary(BinaryOp::Add, old, step);
                self.instruction(format!("store {} {new}, ptr {address}", target.ty.llvm()));
                if *prefix { new } else { old_operand }
            }
        };
        Value {
            ty: expression.ty.clone(),
            operand,
        }
    }
    fn binary(&mut self, op: BinaryOp, left: Value, right: Value) -> String {
        use BinaryOp::*;
        let double = left.ty == Type::Double;
        if matches!(op, Divide | Remainder) && left.ty.is_integer() {
            let zero = self.result(format!("icmp eq {} {}, 0", right.ty.llvm(), right.operand));
            let minimum = if left.ty == Type::Int {
                i64::from(i32::MIN)
            } else {
                i64::MIN
            };
            let min = self.result(format!(
                "icmp eq {} {}, {minimum}",
                left.ty.llvm(),
                left.operand
            ));
            let negative_one =
                self.result(format!("icmp eq {} {}, -1", right.ty.llvm(), right.operand));
            let overflow = self.result(format!("and i1 {min}, {negative_one}"));
            let invalid = self.result(format!("or i1 {zero}, {overflow}"));
            let trap = self.new_block();
            let valid = self.new_block();
            self.conditional(&invalid, &trap, &valid);
            self.label(&trap);
            self.instruction("call void @llvm.trap()");
            self.instruction("unreachable");
            self.terminated = true;
            self.label(&valid);
        }
        let instruction = match (op, double) {
            (Add, false) => "add",
            (Subtract, false) => "sub",
            (Multiply, false) => "mul",
            (Divide, false) => "sdiv",
            (Remainder, false) => "srem",
            (Add, true) => "fadd",
            (Subtract, true) => "fsub",
            (Multiply, true) => "fmul",
            (Divide, true) => "fdiv",
            (Remainder, true) => "frem",
            (Equal, false) => "icmp eq",
            (NotEqual, false) => "icmp ne",
            (Less, false) => "icmp slt",
            (LessEqual, false) => "icmp sle",
            (Greater, false) => "icmp sgt",
            (GreaterEqual, false) => "icmp sge",
            (Equal, true) => "fcmp oeq",
            (NotEqual, true) => "fcmp une",
            (Less, true) => "fcmp olt",
            (LessEqual, true) => "fcmp ole",
            (Greater, true) => "fcmp ogt",
            (GreaterEqual, true) => "fcmp oge",
            _ => unreachable!("logical operators are control flow"),
        };
        self.result(format!(
            "{instruction} {} {}, {}",
            left.ty.llvm(),
            left.operand,
            right.operand
        ))
    }
}

fn double_constant(value: f64) -> String {
    format!("0x{:016X}", value.to_bits())
}
