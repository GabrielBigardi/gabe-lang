use gabe_compiler::{check, compile};

fn program(body: &str) -> String {
    format!("class Program {{ static int Main() {{ {body} }} }}")
}

#[test]
fn examples_pass_analysis() {
    for source in [
        include_str!("../../../examples/hello.gb"),
        include_str!("../../../examples/fibonacci.gb"),
        include_str!("../../../examples/pointers.gb"),
        include_str!("../../../examples/interop.gb"),
    ] {
        check(source).unwrap();
    }
}

#[test]
fn diagnoses_invalid_programs_before_codegen() {
    let cases = [
        ("int x = true; return 0;", "cannot convert bool to int"),
        (
            "int x = 1; x = 1.5; return 0;",
            "cannot convert double to int",
        ),
        ("return missing;", "unknown variable"),
        ("int x = 1; int x = 2; return 0;", "duplicate local"),
        ("if (1) return 0; return 0;", "cannot convert int to bool"),
        ("int x = 1; int* p = &x; return 0;", "require an unsafe"),
        ("break; return 0;", "only valid inside a loop"),
        ("continue; return 0;", "only valid inside a loop"),
        ("int x = 1;", "not all paths"),
        (
            "var x = Console.WriteLine(1); return 0;",
            "cannot have type void",
        ),
        ("1 + 2; return 0;", "expression statement"),
        ("Console.WriteLine(1, 2); return 0;", "zero or one"),
        ("Console.Write(); return 0;", "one value"),
        (
            "for (int i = 0; i < 1; i++) {} return i;",
            "unknown variable",
        ),
        (
            "unsafe { int x = 1; *x = 3; } return 0;",
            "requires a pointer",
        ),
        ("long x = 9223372036854775808; return 0;", "signed 64-bit"),
        ("double x = 1e999; return 0;", "out of range"),
    ];
    for (body, expected) in cases {
        let error = check(&program(body)).unwrap_err();
        assert!(error.message.contains(expected), "{body}: {error}");
    }
}

#[test]
fn validates_entry_signatures_and_function_calls() {
    for (source, expected) in [
        ("int Add(int x) { return x; }", "missing entry"),
        ("int Main(int x) { return x; }", "entry point"),
        ("unsafe int Main() { return 0; }", "entry point"),
        (
            "int Main() { return 0; } void Main() {}",
            "duplicate function",
        ),
        (
            "class A { static int Main() { return 0; } } class B { static int Main() { return 0; } }",
            "exactly one Main",
        ),
        (
            "int F(int x) { return x; } int Main() { return F(); }",
            "expects 1 arguments",
        ),
        (
            "extern int abs(int x); int Main() { return abs(-1); }",
            "require an unsafe",
        ),
        (
            "extern bool flag(); int Main() { return 0; }",
            "extern bool",
        ),
        ("extern int gb_1(); int Main() { return 0; }", "reserved"),
        (
            "class Program { int Main() { return 0; } }",
            "must be static",
        ),
    ] {
        let error = check(source).unwrap_err();
        assert!(error.message.contains(expected), "{source}: {error}");
    }
}

#[test]
fn supports_forward_calls_recursion_and_widening() {
    check("int Main() { double d = F(4); return 0; } long F(int n) { if (n <= 1) return 1; return n * F(n - 1); }").unwrap();
    check(&program(
        "int a = -2147483648; long b = -9223372036854775808L; long c = a; double d = b; return 0;",
    ))
    .unwrap();
}

#[test]
fn handles_scopes_without_leaking_branch_variables() {
    check(&program(
        "int x = 1; { int x = 2; Console.WriteLine(x); } return x;",
    ))
    .unwrap();
    let error = check(&program("if (true) { int x = 1; } return x;")).unwrap_err();
    assert!(error.message.contains("unknown variable"));
}

#[test]
fn llvm_emission_is_deterministic_and_target_specific() {
    let source = program("Console.WriteLine(\"Olá 🌍 %s\"); return 0;");
    let first = compile(&source, "linux-arm64").unwrap();
    assert_eq!(first, compile(&source, "linux-arm64").unwrap());
    assert!(first.contains("target triple = \"aarch64-unknown-linux-gnu\""));
    assert!(first.contains("\\C3\\A1"));
    assert!(compile(&source, "bad\"target").is_err());
}

#[test]
fn reports_source_locations_including_unicode() {
    let source = "int Main() {\n    Console.WriteLine(\"Olá\");\n    return unknown;\n}";
    let error = check(source).unwrap_err().render("test.gb", source);
    assert!(error.starts_with("test.gb:3:12: error:"), "{error}");
    let source = "int Main() { return é; }";
    let error = check(source).unwrap_err().render("test.gb", source);
    assert!(error.contains("unexpected character 'é'"));
}

#[test]
fn malformed_input_is_diagnostic_instead_of_a_panic() {
    for source in [
        "",
        "/*",
        "int Main() {",
        "class Program {",
        "int Main() { Console.WriteLine(\"oops); }",
        "int Main() { double x = 1e+; }",
        "int Main() { Console.WriteLine(\"\\q\"); }",
        "int Main() { Console.WriteLine(\"\0\"); }",
    ] {
        assert!(check(source).is_err(), "{source}");
    }
}
