package compiler

import (
	"strings"
	"testing"
)

func TestDeferAcceptsTypedCalls(t *testing.T) {
	for _, source := range []string{
		"public void F() {} public void Main() { defer F(); }",
		"public int F(long x) { return 1; } public void Main() { defer F(2); }",
		"public class Program { public static void F() {} public static void Main() { defer Program.F(); } }",
		"public void Main() { defer PrintLine(); defer PrintLine(true); defer Print(\"literal %s\"); }",
		"public unsafe void F(int* p) { (*p)++; } public void Main() { int x = 1; unsafe { defer F(&x); } }",
		"public extern int abs(int value); public void Main() { unsafe { defer abs(-1); } }",
		"public void Main() { for (int i = 0; i < 2; i++) { defer PrintLine(i); } }",
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
		{"public void Main() { defer 1; }", "defer requires a function call"},
		{"public void Main() { int x = 1; defer x; }", "defer requires a function call"},
		{"public void Main() { int x = 1; defer x = 2; }", "defer requires a function call"},
		{"public void Main() { int x = 1; defer x++; }", "defer requires a function call"},
		{"public void Main() { defer Missing(); }", "unknown function"},
		{"public void F(int x) {} public void Main() { defer F(); }", "expects 1 arguments"},
		{"public void F(int x) {} public void Main() { defer F(true); }", "cannot convert bool to int"},
		{"public void Main() { defer PrintLine(1, 2); }", "zero or one"},
		{"public void Main() { defer Print(); }", "one value"},
		{"public extern int abs(int x); public void Main() { defer abs(-1); }", "require an unsafe"},
		{"public unsafe void F() {} public void Main() { defer F(); }", "require an unsafe"},
		{"public void Main() { int x = 1; defer PrintLine(&x); }", "require an unsafe"},
		{"public void Main() { defer { PrintLine(1); } }", "expected an expression"},
		{"public void Main() { defer PrintLine(1) }", "expected ';'"},
		{"public void Main() { defer; }", "expected an expression"},
		{"public void Main() { int defer = 1; }", "found a keyword"},
		{"public void defer() {} public void Main() {}", "found a keyword"},
		{"public int Main() { defer PrintLine(1); }", "not all paths"},
		{"public void Main() { if (true) { int x = 1; } defer PrintLine(x); }", "unknown variable"},
	} {
		t.Run(test.source, func(t *testing.T) { assertDiagnostic(t, test.source, test.expected) })
	}
}

func TestDeferCapturesStayInEntryBlock(t *testing.T) {
	source := "public void F(long x, bool y) {} public void Main() { for (int i = 0; i < 3; i++) { defer F(i, true && 8 / 2 == 4); } }"
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
