package cli_test

import "testing"

func TestPrintAndPrintLineAreNative(t *testing.T) {
	for _, test := range []struct{ name, source, expected string }{
		{"same_line_and_newline", `public void Main() { Print("a\nb"); Print("c"); PrintLine("d"); PrintLine(); PrintLine(1); Print(false); }`, "a\nbcd\n\n1\nFalse"},
		{"primitive_values_and_literal_formats", `public void Main() { Print(42); Print(","); Print(9000000000L); Print(","); Print(3.5); Print(","); Print(true); Print(","); PrintLine("Olá 🌍 %s %n"); }`, "42,9000000000,3.5,True,Olá 🌍 %s %n\n"},
		{"private_method_and_entry_point", `public class Program { private static int Value() { return 42; } private static void Main() { defer PrintLine(Value()); PrintLine(Program.Value()); } }`, "42\n42\n"},
		{"public_calls_across_classes", `public class Values { private static int Hidden() { return 42; } public static int Get() { return Hidden(); } } public class Program { public static void Main() { PrintLine(Values.Get()); } }`, "42\n"},
		{"qualified_method_named_print", `public class Output { public static void Print(string text) { Print(text); PrintLine(); } } public void Main() { Output.Print("text"); }`, "text\n"},
		{"private_file_declarations", `private extern int abs(int value); private int Value() { return 42; } private class Program { private static void Main() { unsafe { PrintLine(abs(-Value())); } } }`, "42\n"},
	} {
		t.Run(test.name, func(t *testing.T) { t.Parallel(); assertProgram(t, test.source, test.expected) })
	}
}

func TestPrintingExample(t *testing.T) {
	t.Parallel()
	assertProgram(t, readExample(t, "printing"), "Hello, Gabe!\nFirst line\nSecond line\n42\n")
}
