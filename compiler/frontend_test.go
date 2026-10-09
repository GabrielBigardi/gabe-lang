package compiler

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func program(body string) string {
	return "public class Program { public static int Main() { " + body + " } }"
}
func TestExamplesPassAnalysis(t *testing.T) {
	for _, name := range []string{"hello", "fibonacci", "pointers", "interop", "defer", "printing"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("..", "examples", name+".gb"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Check(string(source)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestDiagnosesInvalidProgramsBeforeCodegen(t *testing.T) {
	cases := []struct{ source, expected string }{
		{"int x = true; return 0;", "cannot convert bool to int"},
		{"int x = 1; x = 1.5; return 0;", "cannot convert double to int"},
		{"return missing;", "unknown variable"},
		{"int x = 1; int x = 2; return 0;", "duplicate local"},
		{"if (1) return 0; return 0;", "cannot convert int to bool"},
		{"int x = 1; int* p = &x; return 0;", "require an unsafe"},
		{"break; return 0;", "only valid inside a loop"},
		{"continue; return 0;", "only valid inside a loop"},
		{"int x = 1;", "not all paths"},
		{"var x = PrintLine(1); return 0;", "cannot have type void"},
		{"1 + 2; return 0;", "expression statement"},
		{"PrintLine(1, 2); return 0;", "zero or one"},
		{"Print(); return 0;", "one value"},
		{"for (int i = 0; i < 1; i++) {} return i;", "unknown variable"},
		{"unsafe { int x = 1; *x = 3; } return 0;", "requires a pointer"},
		{"long x = 9223372036854775808; return 0;", "signed 64-bit"},
		{"double x = 1e999; return 0;", "out of range"},
		{"unsafe { void* p = Get(); *p = 1; } return 0;", "unknown function"},
		{"int x = 1; unsafe { void* p = &x; } return 0;", "cannot convert int* to void*"},
		{"string s = \"x\"; bool b = s == s; return 0;", "does not support string and string"},
		{"bool b = true; b++; return 0;", "require a number"},
		{"double d = 1.2L; return 0;", "cannot have the L suffix"},
	}
	for _, test := range cases {
		t.Run(test.source, func(t *testing.T) { assertDiagnostic(t, program(test.source), test.expected) })
	}
}
func assertDiagnostic(t *testing.T, source, expected string) {
	t.Helper()
	_, err := Check(source)
	var diagnostic *Diagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected diagnostic, got %v for %q", err, source)
	}
	if !strings.Contains(diagnostic.Message, expected) {
		t.Fatalf("expected %q in %q for %q", expected, diagnostic.Message, source)
	}
	if diagnostic.Span.Start < 0 || diagnostic.Span.End < diagnostic.Span.Start || diagnostic.Span.End > len(source) {
		t.Fatalf("invalid diagnostic span: %+v", diagnostic.Span)
	}
}
func TestValidatesEntrySignaturesAndFunctionCalls(t *testing.T) {
	cases := []struct{ source, expected string }{
		{"public int Add(int x) { return x; }", "missing entry"},
		{"public int Main(int x) { return x; }", "entry point"},
		{"public unsafe int Main() { return 0; }", "entry point"},
		{"public int Main() { return 0; } public void Main() {}", "duplicate function"},
		{"public class A { public static int Main() { return 0; } } public class B { public static int Main() { return 0; } }", "exactly one Main"},
		{"public int F(int x) { return x; } public int Main() { return F(); }", "expects 1 arguments"},
		{"public extern int abs(int x); public int Main() { return abs(-1); }", "require an unsafe"},
		{"public extern bool flag(); public int Main() { return 0; }", "extern bool"},
		{"public extern int flag(bool b); public int Main() { return 0; }", "extern bool"},
		{"public extern int gb_1(); public int Main() { return 0; }", "reserved"},
		{"public extern int printf(); public int Main() { return 0; }", "reserved"},
		{"public class Program { public int Main() { return 0; } }", "must be static"},
		{"public class Program { public static extern int abs(int x); public static int Main() { return 0; } }", "file scope"},
		{"public void F(void x) {} public int Main() { return 0; }", "parameters cannot have type void"},
		{"public extern void* F(); public int Main() { unsafe { *F() = 1; } return 0; }", "cannot dereference a void pointer"},
		{"public unsafe int F() { return 0; } public int Main() { return F(); }", "require an unsafe"},
	}
	for _, test := range cases {
		t.Run(test.source, func(t *testing.T) { assertDiagnostic(t, test.source, test.expected) })
	}
}
func TestSupportsForwardCallsRecursionAndWidening(t *testing.T) {
	for _, source := range []string{
		"public int Main() { double d = F(4); return 0; } public long F(int n) { if (n <= 1) return 1; return n * F(n - 1); }",
		program("int a = -2147483648; long b = -9223372036854775808L; long c = a; double d = b; return 0;"),
	} {
		if _, err := Check(source); err != nil {
			t.Fatal(err)
		}
	}
}
func TestHandlesScopesWithoutLeakingBranchVariables(t *testing.T) {
	if _, err := Check(program("int x = 1; { int x = 2; PrintLine(x); } return x;")); err != nil {
		t.Fatal(err)
	}
	assertDiagnostic(t, program("if (true) { int x = 1; } return x;"), "unknown variable")
	assertDiagnostic(t, program("if (true) int x = 1; return x;"), "unknown variable")
	assertDiagnostic(t, program("while (false) int x = 1; return x;"), "unknown variable")
}
func TestLLVMEmissionIsDeterministicAndTargetSpecific(t *testing.T) {
	source := program("PrintLine(\"Olá 🌍 %s\"); return 0;")
	first, err := Compile(source, "linux-arm64")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		next, err := Compile(source, "linux-arm64")
		if err != nil || first != next {
			t.Fatalf("LLVM emission changed: %v", err)
		}
	}
	if !strings.Contains(first, "target triple = \"aarch64-unknown-linux-gnu\"") || !strings.Contains(first, "\\C3\\A1") {
		t.Fatal("incorrect target or UTF-8 encoding")
	}
	if _, err := Compile(source, "bad\"target"); err == nil {
		t.Fatal("accepted invalid target")
	}
}
func TestReportsSourceLocationsIncludingUnicode(t *testing.T) {
	source := "public int Main() {\n    PrintLine(\"Olá\");\n    return unknown;\n}"
	_, err := Check(source)
	var diagnostic *Diagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatal(err)
	}
	rendered := diagnostic.Render("test.gb", source)
	if !strings.HasPrefix(rendered, "test.gb:3:12: error:") {
		t.Fatal(rendered)
	}
	assertDiagnostic(t, "public int Main() { return é; }", "unexpected character 'é'")
	source = "public void Main() { PrintLine(\"🌍\"); @ }"
	_, err = Check(source)
	if !errors.As(err, &diagnostic) {
		t.Fatal(err)
	}
	column := strings.Index(source, "@") - 3 + 1 // one rune occupies four bytes
	if !strings.Contains(diagnostic.Render("test.gb", source), ":1:"+strconv.Itoa(column)+":") {
		t.Fatal(diagnostic.Render("test.gb", source))
	}
}
func TestMalformedInputIsDiagnosticInsteadOfPanic(t *testing.T) {
	for _, source := range []string{"", "/*", "public int Main() {", "public class Program {", "public int Main() { PrintLine(\"oops); }", "public int Main() { double x = 1e+; }", "public int Main() { PrintLine(\"\\q\"); }", "public int Main() { PrintLine(\"\x00\"); }", "\xff", "public int Main(,)", "public int Main() { return --; }"} {
		if _, err := Check(source); err == nil {
			t.Fatalf("accepted malformed input %q", source)
		}
	}
}
func TestLLVMMatchesRustBaseline(t *testing.T) {
	source, err := os.ReadFile("testdata/baseline.gb")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/baseline.golden")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := Compile(string(source), "linux-arm64")
	if err != nil {
		t.Fatal(err)
	}
	if actual != string(expected) {
		t.Fatal("LLVM IR differs from the Rust baseline captured at commit 60f13cb")
	}
}
func FuzzCheck(f *testing.F) {
	for _, source := range []string{"", "public int Main() { return 0; }", "public class Program {", "public void Main() { PrintLine(\"Olá 🌍\"); }", "public int Main() { unsafe { int x = 1; int* p = &x; *p += 1; } return 0; }"} {
		f.Add(source)
	}
	f.Add("public void Main() { defer PrintLine(1); { defer PrintLine(true && 4 / 2 == 2); } }")
	f.Add("public void Main() { for (int i = 0; i < 2; i++) { defer PrintLine(i); if (i == 0) continue; break; } }")
	f.Add("public void Main() { defer; }")
	f.Add("private class Program { private static int F() { return 42; } public static void Main() { Print(F()); PrintLine(); } }")
	f.Add("public private void Main() {}")
	f.Add("public class Program { static void Main() {} }")
	f.Fuzz(func(t *testing.T, source string) {
		_, err := Compile(source, "linux-x64")
		if err == nil {
			return
		}
		var diagnostic *Diagnostic
		if !errors.As(err, &diagnostic) {
			t.Fatal(err)
		}
		_ = diagnostic.Render("fuzz.gb", source)
	})
}
