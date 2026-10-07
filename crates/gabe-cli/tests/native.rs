//! End-to-end behavior tests. CI requires a backend instead of silently skipping.
use std::env;
use std::fs;
use std::path::PathBuf;
use std::process::{Command, Output};
use std::sync::atomic::{AtomicUsize, Ordering};

static NEXT_ID: AtomicUsize = AtomicUsize::new(0);

struct Fixture(PathBuf);

impl Fixture {
    fn new(source: &str) -> Self {
        let root = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../target/native-tests");
        let path = root.join(format!(
            "{}-{}",
            std::process::id(),
            NEXT_ID.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir_all(&path).unwrap();
        fs::write(path.join("program.gb"), source).unwrap();
        Self(path)
    }
    fn command(&self) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_gabe"));
        command.current_dir(&self.0);
        command
    }
    fn run(&self, optimization: &str) -> Output {
        let output = self
            .command()
            .args(["run", "program.gb", optimization])
            .output()
            .unwrap();
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        output
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn native_available() -> bool {
    let backend = env::var_os("GABE_LLC")
        .or_else(|| env::var_os("GABE_CLANG"))
        .unwrap_or_else(|| "clang".into());
    let available = Command::new(backend)
        .arg("--version")
        .output()
        .is_ok_and(|output| output.status.success());
    if !available {
        assert!(
            env::var_os("GABE_REQUIRE_NATIVE_TESTS").is_none(),
            "native tests require Clang or GABE_LLC"
        );
        eprintln!(
            "Skipping native test: install Clang or set GABE_LLC. Set GABE_REQUIRE_NATIVE_TESTS=1 to require native checks."
        );
    }
    available
}

fn assert_program(source: &str, expected: &str) {
    if !native_available() {
        return;
    }
    let fixture = Fixture::new(source);
    // Check observable behavior at both optimization extremes.
    for optimization in ["-O0", "-O3"] {
        let output = fixture.run(optimization);
        let actual = String::from_utf8(output.stdout)
            .unwrap()
            .replace("\r\n", "\n");
        assert_eq!(actual, expected, "{optimization}");
    }
}

#[test]
fn hello_and_literal_output_are_native() {
    assert_program(
        include_str!("../../../examples/hello.gb"),
        "Hello from Gabe!\n",
    );
    assert_program(
        "void Main() { Console.Write(\"100% %s %n\"); Console.WriteLine(); Console.WriteLine(true); Console.WriteLine(false); Console.WriteLine(3.5); Console.WriteLine(9000000000L); }",
        "100% %s %n\nTrue\nFalse\n3.5\n9000000000\n",
    );
    assert_program(
        "void Main() { Console.WriteLine(\"Olá 🌍 %s\"); }",
        "Olá 🌍 %s\n",
    );
}

#[test]
fn functions_loops_and_recursion_work() {
    assert_program(
        include_str!("../../../examples/fibonacci.gb"),
        "102334155\n",
    );
    assert_program(
        "int Main() { Console.WriteLine(Factorial(6)); return 0; } long Factorial(int n) { if (n <= 1) return 1; else return n * Factorial(n - 1); }",
        "720\n",
    );
}

#[test]
fn pointers_and_c_interop_work() {
    assert_program(include_str!("../../../examples/pointers.gb"), "42\n");
    assert_program(include_str!("../../../examples/interop.gb"), "42\n");
    assert_program(
        "unsafe int* Get(int* p, int* count) { (*count)++; return p; } int Main() { int x = 5; int count = 0; unsafe { *Get(&x, &count) += 3; } Console.WriteLine(x); Console.WriteLine(count); return 0; }",
        "8\n1\n",
    );
}

#[test]
fn short_circuit_preserves_side_effects_and_nested_control_flow() {
    assert_program(
        "int Main() { int x = 0; bool a = false && ++x > 0; bool b = true || ++x > 0; bool c = true && (++x == 1 || ++x == 99); bool d = true && 8 / 2 == 4 && ++x == 2; Console.WriteLine(x); Console.WriteLine(a); Console.WriteLine(b); Console.WriteLine(c); Console.WriteLine(d); return 0; }",
        "2\nFalse\nTrue\nTrue\nTrue\n",
    );
}

#[test]
fn continue_break_and_postfix_have_correct_order() {
    assert_program(
        "int Main() { int sum = 0; for (int i = 0; i < 10; i++) { if (i == 2) continue; if (i == 5) break; sum += i; } int j = 0; while (j < 4) { j++; if (j == 2) continue; sum += j; } int old = j++; int next = ++j; Console.WriteLine(sum); Console.WriteLine(old); Console.WriteLine(next); return 0; }",
        "16\n4\n6\n",
    );
}

#[test]
fn integer_wrapping_widening_and_float_comparisons_work() {
    assert_program(
        "int Main() { int x = 2147483647; x++; Console.WriteLine(x); long y = x; Console.WriteLine(y); double d = 7; Console.WriteLine(d / 2); Console.WriteLine(d % 2.0); double nan = 0.0 / 0.0; Console.WriteLine(nan == nan); Console.WriteLine(nan != nan); Console.WriteLine(-17 / 5); Console.WriteLine(-17 % 5); return 0; }",
        "-2147483648\n-2147483648\n3.5\n1\nFalse\nTrue\n-3\n-2\n",
    );
}

#[test]
fn exit_codes_and_cli_errors_propagate() {
    let fixture = Fixture::new("int Main() { return 0; }");
    fs::write(
        fixture.0.join("program.part.gb"),
        "int Main() { return 0; }",
    )
    .unwrap();
    let output = fixture
        .command()
        .args(["build", "program.part.gb", "--emit", "llvm"])
        .output()
        .unwrap();
    assert!(output.status.success());
    assert!(fixture.0.join("build/program.part.ll").is_file());
    let output = fixture
        .command()
        .args(["build", "program.gb", "--emit", "llvm", "-o", "program.gb"])
        .output()
        .unwrap();
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("overwrite the source"));
    assert_eq!(
        fs::read_to_string(fixture.0.join("program.gb")).unwrap(),
        "int Main() { return 0; }"
    );
    fs::write(fixture.0.join("program.gb"), "int Main() { return true; }").unwrap();
    let output = fixture
        .command()
        .args(["check", "program.gb"])
        .output()
        .unwrap();
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("program.gb:1:"));
    if native_available() {
        fs::write(fixture.0.join("program.gb"), "int Main() { return 7; }").unwrap();
        let output = fixture
            .command()
            .args(["run", "program.gb"])
            .output()
            .unwrap();
        assert_eq!(output.status.code(), Some(7));
    }
}

#[test]
fn invalid_integer_division_traps() {
    if !native_available() {
        return;
    }
    for source in [
        "int Main() { int zero = 0; return 42 / zero; }",
        "int Main() { int min = -2147483648; return min / -1; }",
    ] {
        let fixture = Fixture::new(source);
        for optimization in ["-O0", "-O3"] {
            let build = fixture
                .command()
                .args(["build", "program.gb", optimization])
                .output()
                .unwrap();
            assert!(
                build.status.success(),
                "{}",
                String::from_utf8_lossy(&build.stderr)
            );
            let executable = fixture.0.join(if cfg!(windows) {
                "build/program.exe"
            } else {
                "build/program"
            });
            let status = Command::new(executable).output().unwrap().status;
            assert!(!status.success());
        }
    }
}

#[test]
fn emits_objects_for_primary_target_families() {
    if !native_available() {
        return;
    }
    let fixture = Fixture::new("int Main() { return 42; }");
    for (target, magic) in [
        ("windows-x64", vec![0x64, 0x86]),
        ("windows-arm64", vec![0x64, 0xAA]),
        ("linux-x64", vec![0x7F, b'E', b'L', b'F']),
        ("linux-arm64", vec![0x7F, b'E', b'L', b'F']),
        ("macos-x64", vec![0xCF, 0xFA, 0xED, 0xFE]),
        ("macos-arm64", vec![0xCF, 0xFA, 0xED, 0xFE]),
    ] {
        let output = fixture
            .command()
            .args([
                "build",
                "program.gb",
                "--emit",
                "obj",
                "--target",
                target,
                "-o",
                "cross.obj",
            ])
            .output()
            .unwrap();
        assert!(
            output.status.success(),
            "{target}: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        let object = fs::read(fixture.0.join("cross.obj")).unwrap();
        assert!(
            object.starts_with(&magic),
            "{target}: incorrect object format"
        );
        let arm64 = target.ends_with("arm64");
        if target.starts_with("linux") {
            assert_eq!(object.get(4), Some(&2), "{target}: expected ELF64");
            let machine = if arm64 { [0xB7, 0] } else { [0x3E, 0] };
            assert_eq!(
                object.get(18..20),
                Some(machine.as_slice()),
                "{target}: incorrect ELF CPU"
            );
        } else if target.starts_with("macos") {
            let cpu = if arm64 { [12, 0, 0, 1] } else { [7, 0, 0, 1] };
            assert_eq!(
                object.get(4..8),
                Some(cpu.as_slice()),
                "{target}: incorrect Mach-O CPU"
            );
        }
    }
}
