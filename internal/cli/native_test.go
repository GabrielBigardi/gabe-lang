package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var compilerBinary string
var nativeOnce sync.Once
var nativeAvailable bool

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "gabe-cli-tests-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	name := "gabe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	compilerBinary = filepath.Join(directory, name)
	command := exec.Command("go", "build", "-o", compilerBinary, "./cmd/gabe")
	command.Dir = filepath.Join("..", "..")
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "cannot build test compiler: %v\n%s", err, output)
		_ = os.RemoveAll(directory)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(directory)
	os.Exit(code)
}

type fixture struct {
	t         *testing.T
	directory string
}
type processResult struct {
	stdout, stderr string
	code           int
}

func newFixture(t *testing.T, source string) *fixture {
	t.Helper()
	f := &fixture{t: t, directory: t.TempDir()}
	f.write("program.gb", source)
	return f
}
func (f *fixture) write(name, source string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.directory, name), []byte(source), 0o644); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) command(arguments ...string) processResult {
	f.t.Helper()
	return runProcess(f.t, f.directory, compilerBinary, arguments...)
}
func runProcess(t *testing.T, directory, executable string, arguments ...string) processResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("process timed out: %s %v", executable, arguments)
	}
	code := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatal(err)
		}
		code = exitError.ExitCode()
	}
	return processResult{stdout.String(), stderr.String(), code}
}
func requireNative(t *testing.T) {
	t.Helper()
	nativeOnce.Do(func() {
		backend := os.Getenv("GABE_LLC")
		if backend == "" {
			backend = os.Getenv("GABE_CLANG")
		}
		if backend == "" {
			backend = "clang"
		}
		nativeAvailable = exec.Command(backend, "--version").Run() == nil
	})
	if nativeAvailable {
		return
	}
	if _, required := os.LookupEnv("GABE_REQUIRE_NATIVE_TESTS"); required {
		t.Fatal("native tests require Clang or GABE_LLC")
	}
	t.Skip("install Clang or set GABE_LLC; set GABE_REQUIRE_NATIVE_TESTS=1 to require native checks")
}
func readExample(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "examples", name+".gb"))
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}
func assertProgram(t *testing.T, source, expected string) {
	t.Helper()
	requireNative(t)
	f := newFixture(t, source)
	for _, optimization := range []string{"-O0", "-O3"} {
		output := f.command("run", "program.gb", optimization)
		if output.code != 0 {
			t.Fatalf("%s: %s", optimization, output.stderr)
		}
		if actual := strings.ReplaceAll(output.stdout, "\r\n", "\n"); actual != expected {
			t.Fatalf("%s: got %q, want %q", optimization, actual, expected)
		}
	}
}
func TestHelloAndLiteralOutputAreNative(t *testing.T) {
	t.Parallel()
	assertProgram(t, readExample(t, "hello"), "Hello from Gabe!\n")
	assertProgram(t, `void Main() { Console.Write("100% %s %n"); Console.WriteLine(); Console.WriteLine(true); Console.WriteLine(false); Console.WriteLine(3.5); Console.WriteLine(9000000000L); }`, "100% %s %n\nTrue\nFalse\n3.5\n9000000000\n")
	assertProgram(t, `void Main() { Console.WriteLine("Olá 🌍 %s"); }`, "Olá 🌍 %s\n")
}
func TestFunctionsLoopsAndRecursionWork(t *testing.T) {
	t.Parallel()
	assertProgram(t, readExample(t, "fibonacci"), "102334155\n")
	assertProgram(t, "int Main() { Console.WriteLine(Factorial(6)); return 0; } long Factorial(int n) { if (n <= 1) return 1; else return n * Factorial(n - 1); }", "720\n")
}
func TestPointersAndCInteropWork(t *testing.T) {
	t.Parallel()
	assertProgram(t, readExample(t, "pointers"), "42\n")
	assertProgram(t, readExample(t, "interop"), "42\n")
	assertProgram(t, "unsafe int* Get(int* p, int* count) { (*count)++; return p; } int Main() { int x = 5; int count = 0; unsafe { *Get(&x, &count) += 3; } Console.WriteLine(x); Console.WriteLine(count); return 0; }", "8\n1\n")
}
func TestShortCircuitPreservesSideEffectsAndNestedControlFlow(t *testing.T) {
	t.Parallel()
	assertProgram(t, "int Main() { int x = 0; bool a = false && ++x > 0; bool b = true || ++x > 0; bool c = true && (++x == 1 || ++x == 99); bool d = true && 8 / 2 == 4 && ++x == 2; Console.WriteLine(x); Console.WriteLine(a); Console.WriteLine(b); Console.WriteLine(c); Console.WriteLine(d); return 0; }", "2\nFalse\nTrue\nTrue\nTrue\n")
}
func TestContinueBreakAndPostfixHaveCorrectOrder(t *testing.T) {
	t.Parallel()
	assertProgram(t, "int Main() { int sum = 0; for (int i = 0; i < 10; i++) { if (i == 2) continue; if (i == 5) break; sum += i; } int j = 0; while (j < 4) { j++; if (j == 2) continue; sum += j; } int old = j++; int next = ++j; Console.WriteLine(sum); Console.WriteLine(old); Console.WriteLine(next); return 0; }", "16\n4\n6\n")
}
func TestIntegerWrappingWideningAndFloatComparisonsWork(t *testing.T) {
	t.Parallel()
	assertProgram(t, "int Main() { int x = 2147483647; x++; Console.WriteLine(x); long y = x; Console.WriteLine(y); double d = 7; Console.WriteLine(d / 2); Console.WriteLine(d % 2.0); double nan = 0.0 / 0.0; Console.WriteLine(nan == nan); Console.WriteLine(nan != nan); Console.WriteLine(-17 / 5); Console.WriteLine(-17 % 5); return 0; }", "-2147483648\n-2147483648\n3.5\n1\nFalse\nTrue\n-3\n-2\n")
}
func TestArgumentAndAssignmentEvaluationOrder(t *testing.T) {
	t.Parallel()
	assertProgram(t, "unsafe int* Get(int* p, int* count) { (*count)++; return p; } unsafe int Set(int* p) { *p = 100; return 3; } int Pair(int a, int b) { return a * 10 + b; } int Main() { int x = 5; int count = 0; unsafe { *Get(&x, &count) += Set(&x); } Console.WriteLine(x); Console.WriteLine(count); int i = 1; Console.WriteLine(Pair(++i, ++i)); return 0; }", "8\n1\n23\n")
}
func TestRustBaselineIsNative(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile(filepath.Join("..", "..", "compiler", "testdata", "baseline.gb"))
	if err != nil {
		t.Fatal(err)
	}
	assertProgram(t, string(source), "3\n-9223372036854775808\nTrue\nFalse\nTrue\nFalse\nOlá 🌍 %s %n\n\t\"\\\n720\n")
}

func TestExitCodesAndCLIErrorsPropagate(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "int Main() { return 0; }")
	f.write("program.part.gb", "int Main() { return 0; }")
	output := f.command("build", "program.part.gb", "--emit", "llvm")
	if output.code != 0 {
		t.Fatal(output.stderr)
	}
	if _, err := os.Stat(filepath.Join(f.directory, "build", "program.part.ll")); err != nil {
		t.Fatal(err)
	}
	output = f.command("build", "program.gb", "--emit", "llvm", "-o", "program.gb")
	if output.code == 0 || !strings.Contains(output.stderr, "overwrite the source") {
		t.Fatalf("incorrect overwrite rejection: %+v", output)
	}
	source, err := os.ReadFile(filepath.Join(f.directory, "program.gb"))
	if err != nil || string(source) != "int Main() { return 0; }" {
		t.Fatal("source was overwritten", err)
	}
	for _, test := range []struct {
		arguments []string
		expected  string
	}{
		{[]string{"build", "program.gb", "--emit", "invalid"}, "unknown output kind"},
		{[]string{"run", "program.gb", "--emit", "llvm"}, "requires --emit exe"},
		{[]string{"build", "program.gb", "--target"}, "requires a value"},
		{[]string{"build", "program.gb", "--emit", "obj", "--link-arg", "-lm"}, "requires --emit exe"},
		{[]string{"check", "program.gb", "-o", "out"}, "only analyzes"},
		{[]string{"build", "program.gb", "--clang", "missing-compiler", "--emit", "obj"}, "could not start Clang"},
		{[]string{"build", "program.gb", "--emit", "llvm", "--target", "bad\"target"}, "expected a target alias"},
		{[]string{"build", "program.txt"}, ".gb extension"},
		{[]string{"build", "program.gb", "other.gb"}, "exactly one"},
	} {
		output := f.command(test.arguments...)
		if output.code == 0 || !strings.Contains(output.stderr, test.expected) {
			t.Fatalf("%v: %+v", test.arguments, output)
		}
	}
	f.write("program.gb", "int Main() { return true; }")
	output = f.command("check", "program.gb")
	if output.code == 0 || !strings.Contains(output.stderr, "program.gb:1:") {
		t.Fatalf("incorrect source diagnostic: %+v", output)
	}
	requireNative(t)
	f.write("program.gb", "int Main() { return 7; }")
	output = f.command("run", "program.gb")
	if output.code != 7 {
		t.Fatalf("expected program exit 7, got %+v", output)
	}
}
func TestInvalidIntegerDivisionAndRemainderTrap(t *testing.T) {
	t.Parallel()
	requireNative(t)
	for _, source := range []string{
		"int Main() { int zero = 0; return 42 / zero; }",
		"int Main() { int zero = 0; return 42 % zero; }",
		"int Main() { int min = -2147483648; return min / -1; }",
		"int Main() { int min = -2147483648; return min % -1; }",
		"void Main() { long zero = 0L; Console.WriteLine(42L / zero); }",
		"void Main() { long zero = 0L; Console.WriteLine(42L % zero); }",
		"void Main() { long min = -9223372036854775808L; Console.WriteLine(min / -1L); }",
		"void Main() { long min = -9223372036854775808L; Console.WriteLine(min % -1L); }",
	} {
		f := newFixture(t, source)
		for _, optimization := range []string{"-O0", "-O3"} {
			output := f.command("build", "program.gb", optimization)
			if output.code != 0 {
				t.Fatalf("%s: %s", optimization, output.stderr)
			}
			name := "program"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			output = runProcess(t, f.directory, filepath.Join(f.directory, "build", name))
			if output.code == 0 {
				t.Fatalf("%s: invalid division/remainder did not trap: %s", optimization, source)
			}
		}
	}
}
func TestEmitsObjectsForPrimaryTargetFamilies(t *testing.T) {
	t.Parallel()
	requireNative(t)
	f := newFixture(t, "int Main() { return 42; }")
	for _, test := range []struct {
		target string
		magic  []byte
	}{
		{"windows-x64", []byte{0x64, 0x86}}, {"windows-arm64", []byte{0x64, 0xAA}},
		{"linux-x64", []byte{0x7F, 'E', 'L', 'F'}}, {"linux-arm64", []byte{0x7F, 'E', 'L', 'F'}},
		{"macos-x64", []byte{0xCF, 0xFA, 0xED, 0xFE}}, {"macos-arm64", []byte{0xCF, 0xFA, 0xED, 0xFE}},
	} {
		output := f.command("build", "program.gb", "--emit", "obj", "--target", test.target, "-o", "cross.obj")
		if output.code != 0 {
			t.Fatalf("%s: %s", test.target, output.stderr)
		}
		object, err := os.ReadFile(filepath.Join(f.directory, "cross.obj"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(object, test.magic) {
			t.Fatalf("%s: incorrect object format", test.target)
		}
		arm64 := strings.HasSuffix(test.target, "arm64")
		if strings.HasPrefix(test.target, "linux") {
			machine := []byte{0x3E, 0}
			if arm64 {
				machine = []byte{0xB7, 0}
			}
			if len(object) < 20 || object[4] != 2 || !bytes.Equal(object[18:20], machine) {
				t.Fatalf("%s: incorrect ELF class/CPU", test.target)
			}
		} else if strings.HasPrefix(test.target, "macos") {
			cpu := []byte{7, 0, 0, 1}
			if arm64 {
				cpu = []byte{12, 0, 0, 1}
			}
			if len(object) < 8 || !bytes.Equal(object[4:8], cpu) {
				t.Fatalf("%s: incorrect Mach-O CPU", test.target)
			}
		}
	}
}
