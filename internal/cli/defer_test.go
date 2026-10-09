package cli_test

import "testing"

func TestDeferScopeAndCaptureBehavior(t *testing.T) {
	for _, test := range []struct{ name, source, expected string }{
		{"fallthrough_and_lifo", `void F() { Console.WriteLine("function"); } void Main() { defer F(); defer Console.WriteLine("second"); Console.WriteLine("body"); }`, "body\nsecond\nfunction\n"},
		{"empty_output_call", `void Main() { defer Console.WriteLine(); Console.Write("body"); }`, "body\n"},
		{"arguments_and_widening", `void Pair(long a, double b) { Console.WriteLine(a); Console.WriteLine(b); } void Main() { int x = 0; defer Pair(x++, ++x); Console.WriteLine(x); x = 99; }`, "2\n0\n2\n"},
		{"nested_blocks", `void Main() { defer Console.WriteLine(1); { defer Console.WriteLine(2); { defer Console.WriteLine(3); Console.WriteLine(4); } Console.WriteLine(5); } Console.WriteLine(6); }`, "4\n3\n5\n2\n6\n1\n"},
		{"conditional_and_single_statement_scopes", `void Main() { defer Console.WriteLine(9); if (false) defer Console.WriteLine(1); if (true) defer Console.WriteLine(2); if (false) { defer Console.WriteLine(3); } else { defer Console.WriteLine(4); defer Console.WriteLine(5); } Console.WriteLine(6); }`, "2\n5\n4\n6\n9\n"},
		{"loop_iterations", `void Main() { defer Console.WriteLine(9); for (int i = 0; i < 3; i++) { defer Console.WriteLine(i); Console.WriteLine(i + 10); } Console.WriteLine(8); }`, "10\n0\n11\n1\n12\n2\n8\n9\n"},
		{"single_statement_loop_scopes", `unsafe void Increment(int* p) { Console.WriteLine(*p); (*p)++; } void Main() { int x = 0; unsafe { while (x < 3) defer Increment(&x); for (int i = 0; i < 2; i++) defer Console.WriteLine(i + 10); } Console.WriteLine(x); }`, "0\n1\n2\n10\n11\n3\n"},
		{"recursive_function_scopes", `void F(int n) { defer Console.WriteLine(n); if (n > 0) F(n - 1); } void Main() { F(3); }`, "0\n1\n2\n3\n"},
		{"return_unwinds_reached_defers", `int F(bool early) { defer Console.WriteLine(1); { defer Console.WriteLine(2); if (early) return 3; } defer Console.WriteLine(4); return 5; } void Main() { Console.WriteLine(F(true)); Console.WriteLine(F(false)); }`, "2\n1\n3\n2\n4\n1\n5\n"},
		{"unreachable_registration", `int F() { defer Console.WriteLine(1); return 7; defer Console.WriteLine(2); } void Main() { Console.WriteLine(F()); }`, "1\n7\n"},
		{"void_return", `void F() { defer Console.WriteLine(1); { defer Console.WriteLine(2); return; } } void Main() { F(); Console.WriteLine(3); }`, "2\n1\n3\n"},
		{"boolean_control_flow_and_string_capture", `void Main() { string text = "100% %s %n"; defer Console.WriteLine(text); text = "later"; defer Console.WriteLine(false || (true && 8 / 2 == 4)); Console.WriteLine("body"); }`, "body\nTrue\n100% %s %n\n"},
		{"discarded_results_and_c_interop", `extern int putchar(int value); void Main() { unsafe { defer putchar(65); defer putchar(66); Console.WriteLine("body"); } Console.WriteLine(); Console.WriteLine("after"); }`, "body\nBA\nafter\n"},
		{"pointer_capture", `unsafe void Show(int* p) { Console.WriteLine(*p); } void Main() { unsafe { int first = 1; int second = 2; int* p = &first; defer Show(p); p = &second; first = 3; } }`, "3\n"},
		{"return_value_before_cleanup", `unsafe void Set(int* p) { *p = 99; Console.WriteLine("cleanup"); } int F() { int x = 7; unsafe { defer Set(&x); return x++; } } void Main() { Console.WriteLine(F()); }`, "cleanup\n7\n"},
	} {
		t.Run(test.name, func(t *testing.T) { t.Parallel(); assertProgram(t, test.source, test.expected) })
	}
}

func TestDeferExample(t *testing.T) {
	t.Parallel()
	assertProgram(t, readExample(t, "defer"), "inside block\nsecond cleanup\n1\nafter block\nfunction cleanup\n")
}

func TestDeferLoopExits(t *testing.T) {
	for _, test := range []struct{ name, source, expected string }{
		{"for_cleanup_before_step", `unsafe void Set(int* p, int value) { *p = value; } int Next(int i, int mark) { Console.WriteLine(mark); return i + 1; } int Main() { defer Console.WriteLine(99); int mark = 0; unsafe { for (int i = 0; i < 4; i = Next(i, mark)) { defer Set(&mark, i + 10); if (i == 0) continue; if (i == 2) break; Console.WriteLine(i + 100); } } Console.WriteLine(mark); return 0; }`, "10\n101\n11\n12\n99\n"},
		{"nested_loop_exits_preserve_outer_scope", `void Main() { defer Console.WriteLine(9); for (int i = 0; i < 2; i++) { defer Console.WriteLine(i + 10); for (int j = 0; j < 2; j++) { defer Console.WriteLine(j + 20); if (j == 0) continue; break; } Console.WriteLine(i + 30); } Console.WriteLine(40); }`, "20\n21\n30\n10\n20\n21\n31\n11\n40\n9\n"},
		{"while_cleanup_before_condition", `unsafe void Increment(int* p) { (*p)++; } void Main() { int x = 0; unsafe { while (x < 3) { defer Increment(&x); Console.WriteLine(x); continue; } } Console.WriteLine(x); }`, "0\n1\n2\n3\n"},
		{"return_from_nested_loops", `int F() { defer Console.WriteLine(1); for (int i = 0; i < 2; i++) { defer Console.WriteLine(2); while (true) { defer Console.WriteLine(3); { defer Console.WriteLine(4); return 5; } } } return 0; } void Main() { Console.WriteLine(F()); }`, "4\n3\n2\n1\n5\n"},
	} {
		t.Run(test.name, func(t *testing.T) { t.Parallel(); assertProgram(t, test.source, test.expected) })
	}
}
