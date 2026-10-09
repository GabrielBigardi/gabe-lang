package compiler

import "testing"

func TestExplicitAccessAcceptsDeclarations(t *testing.T) {
	for _, source := range []string{
		"public void Main() {}",
		"private int Main() { return 0; }",
		"private class Program { private static void Main() {} }",
		"public static class Program { private static int F() { return 1; } public static void Main() { PrintLine(F()); PrintLine(Program.F()); } }",
		"private extern int abs(int value); public class Program { public static void Main() { unsafe { PrintLine(abs(-1)); } } }",
		"private int F() { return 1; } public class Program { public static void Main() { PrintLine(F()); } }",
		"static public class Program { static private void Main() {} }",
		"public class A { private static int F() { return 1; } public static int G() { return F(); } } public class B { public static void Main() { PrintLine(A.G()); } }",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Compile(source, "linux-x64"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExplicitAccessRejectsInvalidDeclarations(t *testing.T) {
	for _, test := range []struct{ source, expected string }{
		{"void Main() {}", "requires an explicit 'public' or 'private'"},
		{"public void Main() {} void Helper() {}", "requires an explicit 'public' or 'private'"},
		{"class Program { public static void Main() {} }", "requires an explicit 'public' or 'private'"},
		{"public class Program { static void Main() {} }", "requires an explicit 'public' or 'private'"},
		{"extern int abs(int value); public void Main() {}", "requires an explicit 'public' or 'private'"},
		{"public private void Main() {}", "exactly one access modifier"},
		{"private public class Program { public static void Main() {} }", "exactly one access modifier"},
		{"public public void Main() {}", "exactly one access modifier"},
		{"public class Program { private private static void Main() {} }", "exactly one access modifier"},
		{"internal void Main() {}", "'internal' is not supported"},
		{"public internal class Program { public static void Main() {} }", "'internal' is not supported"},
		{"public class Program { internal static void Main() {} }", "'internal' is not supported"},
		{"public class A { private static void F() {} } public void Main() { A.F(); }", "private to class 'A'"},
		{"public class A { private static void F() {} } public class B { public static void Main() { A.F(); } }", "private to class 'A'"},
		{"public class A { private static void F() {} } public void Main() { defer A.F(); }", "private to class 'A'"},
		{"public class Program { private static void Main() {} } public void F() { Program.Main(); }", "private to class 'Program'"},
	} {
		t.Run(test.source, func(t *testing.T) { assertDiagnostic(t, test.source, test.expected) })
	}
}

func TestPrintRejectsInvalidCallsAndDeclarations(t *testing.T) {
	for _, test := range []struct{ source, expected string }{
		{"public void Main() { Print(); }", "Print takes one value"},
		{"public void Main() { Print(1, 2); }", "Print takes one value"},
		{"public void Main() { PrintLine(1, 2); }", "PrintLine takes zero or one"},
		{"public void F() {} public void Main() { Print(F()); }", "support int, long, double, bool, and string"},
		{"public void Main() { int x = 1; unsafe { PrintLine(&x); } }", "support int, long, double, bool, and string"},
		{"public void Print() {} public void Main() {}", "'Print' is reserved"},
		{"private extern int PrintLine(); public void Main() {}", "'PrintLine' is reserved"},
		{"public void Main() { Console.Write(1); }", "unknown function 'Console.Write'"},
		{"public void Main() { Console.WriteLine(1); }", "unknown function 'Console.WriteLine'"},
		{"public void Main() { System.Console.Write(1); }", "unknown function 'System.Console.Write'"},
		{"public void Main() { System.Console.WriteLine(1); }", "unknown function 'System.Console.WriteLine'"},
		{"public void Main() { defer Console.WriteLine(); }", "unknown function 'Console.WriteLine'"},
		{"public void Main() { Printline(1); }", "unknown function 'Printline'"},
	} {
		t.Run(test.source, func(t *testing.T) { assertDiagnostic(t, test.source, test.expected) })
	}
}
