package compiler

import (
	"strings"
	"testing"
)

func TestDeferAcceptsTypedCalls(t *testing.T) {
	for _, source := range []string{
		"void F() {} void Main() { defer F(); }",
		"int F(long x) { return 1; } void Main() { defer F(2); }",
		"class Program { static void F() {} static void Main() { defer Program.F(); } }",
		"void Main() { defer Console.WriteLine(); defer System.Console.WriteLine(true); defer Console.Write(\"literal %s\"); }",
		"unsafe void F(int* p) { (*p)++; } void Main() { int x = 1; unsafe { defer F(&x); } }",
		"extern int abs(int value); void Main() { unsafe { defer abs(-1); } }",
		"void Main() { for (int i = 0; i < 2; i++) { defer Console.WriteLine(i); } }",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Compile(source, "linux-x64"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeferRejectsInvalidStatements(t *testing.T) {
	for _, test := range []struct{ source, expected string }{
		{"void Main() { defer 1; }", "defer requires a function call"},
		{"void Main() { int x = 1; defer x; }", "defer requires a function call"},
		{"void Main() { int x = 1; defer x = 2; }", "defer requires a function call"},
		{"void Main() { int x = 1; defer x++; }", "defer requires a function call"},
		{"void Main() { defer Missing(); }", "unknown function"},
		{"void F(int x) {} void Main() { defer F(); }", "expects 1 arguments"},
		{"void F(int x) {} void Main() { defer F(true); }", "cannot convert bool to int"},
		{"void Main() { defer Console.WriteLine(1, 2); }", "zero or one"},
		{"void Main() { defer Console.Write(); }", "one value"},
		{"extern int abs(int x); void Main() { defer abs(-1); }", "require an unsafe"},
		{"unsafe void F() {} void Main() { defer F(); }", "require an unsafe"},
		{"void Main() { int x = 1; defer Console.WriteLine(&x); }", "require an unsafe"},
		{"void Main() { defer { Console.WriteLine(1); } }", "expected an expression"},
		{"void Main() { defer Console.WriteLine(1) }", "expected ';'"},
		{"void Main() { defer; }", "expected an expression"},
		{"void Main() { int defer = 1; }", "found a keyword"},
		{"void defer() {} void Main() {}", "found a keyword"},
		{"int Main() { defer Console.WriteLine(1); }", "not all paths"},
		{"void Main() { if (true) { int x = 1; } defer Console.WriteLine(x); }", "unknown variable"},
	} {
		t.Run(test.source, func(t *testing.T) { assertDiagnostic(t, test.source, test.expected) })
	}
}

func TestDeferCapturesStayInEntryBlock(t *testing.T) {
	source := "void F(long x, bool y) {} void Main() { for (int i = 0; i < 3; i++) { defer F(i, true && 8 / 2 == 4); } }"
	ir, err := Compile(source, "linux-arm64")
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range strings.Split(ir, "define ")[1:] {
		entry := true
		for _, line := range strings.Split(fn, "\n") {
			if strings.HasSuffix(line, ":") {
				entry = line == "entry:"
			}
			if strings.Contains(line, " = alloca ") && !entry {
				t.Fatalf("capture alloca outside entry: %s", line)
			}
		}
	}
	next, err := Compile(source, "linux-arm64")
	if err != nil || ir != next {
		t.Fatalf("defer IR is not deterministic: %v", err)
	}
}
