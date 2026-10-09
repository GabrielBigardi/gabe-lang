package cli_test

import "testing"

func TestDeferScopeAndCaptureBehavior(t *testing.T) {
	for _, test := range []struct{ name, source, expected string }{
		{"fallthrough_and_lifo", `public void F() { PrintLine("function"); } public void Main() { defer F(); defer PrintLine("second"); PrintLine("body"); }`, "body\nsecond\nfunction\n"},
		{"empty_output_call", `public void Main() { defer PrintLine(); Print("body"); }`, "body\n"},
		{"arguments_and_widening", `public void Pair(long a, double b) { PrintLine(a); PrintLine(b); } public void Main() { int x = 0; defer Pair(x++, ++x); PrintLine(x); x = 99; }`, "2\n0\n2\n"},
		{"nested_blocks", `public void Main() { defer PrintLine(1); { defer PrintLine(2); { defer PrintLine(3); PrintLine(4); } PrintLine(5); } PrintLine(6); }`, "4\n3\n5\n2\n6\n1\n"},
		{"conditional_and_single_statement_scopes", `public void Main() { defer PrintLine(9); if (false) defer PrintLine(1); if (true) defer PrintLine(2); if (false) { defer PrintLine(3); } else { defer PrintLine(4); defer PrintLine(5); } PrintLine(6); }`, "2\n5\n4\n6\n9\n"},
		{"loop_iterations", `public void Main() { defer PrintLine(9); for (int i = 0; i < 3; i++) { defer PrintLine(i); PrintLine(i + 10); } PrintLine(8); }`, "10\n0\n11\n1\n12\n2\n8\n9\n"},
		{"single_statement_loop_scopes", `public unsafe void Increment(int* p) { PrintLine(*p); (*p)++; } public void Main() { int x = 0; unsafe { while (x < 3) defer Increment(&x); for (int i = 0; i < 2; i++) defer PrintLine(i + 10); } PrintLine(x); }`, "0\n1\n2\n10\n11\n3\n"},
		{"recursive_function_scopes", `public void F(int n) { defer PrintLine(n); if (n > 0) F(n - 1); } public void Main() { F(3); }`, "0\n1\n2\n3\n"},
		{"return_unwinds_reached_defers", `public int F(bool early) { defer PrintLine(1); { defer PrintLine(2); if (early) return 3; } defer PrintLine(4); return 5; } public void Main() { PrintLine(F(true)); PrintLine(F(false)); }`, "2\n1\n3\n2\n4\n1\n5\n"},
		{"unreachable_registration", `public int F() { defer PrintLine(1); return 7; defer PrintLine(2); } public void Main() { PrintLine(F()); }`, "1\n7\n"},
		{"void_return", `public void F() { defer PrintLine(1); { defer PrintLine(2); return; } } public void Main() { F(); PrintLine(3); }`, "2\n1\n3\n"},
		{"boolean_control_flow_and_string_capture", `public void Main() { string text = "100% %s %n"; defer PrintLine(text); text = "later"; defer PrintLine(false || (true && 8 / 2 == 4)); PrintLine("body"); }`, "body\nTrue\n100% %s %n\n"},
		{"discarded_results_and_c_interop", `public extern int putchar(int value); public void Main() { unsafe { defer putchar(65); defer putchar(66); PrintLine("body"); } PrintLine(); PrintLine("after"); }`, "body\nBA\nafter\n"},
		{"pointer_capture", `public unsafe void Show(int* p) { PrintLine(*p); } public void Main() { unsafe { int first = 1; int second = 2; int* p = &first; defer Show(p); p = &second; first = 3; } }`, "3\n"},
		{"return_value_before_cleanup", `public unsafe void Set(int* p) { *p = 99; PrintLine("cleanup"); } public int F() { int x = 7; unsafe { defer Set(&x); return x++; } } public void Main() { PrintLine(F()); }`, "cleanup\n7\n"},
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
		{"for_cleanup_before_step", `public unsafe void Set(int* p, int value) { *p = value; } public int Next(int i, int mark) { PrintLine(mark); return i + 1; } public int Main() { defer PrintLine(99); int mark = 0; unsafe { for (int i = 0; i < 4; i = Next(i, mark)) { defer Set(&mark, i + 10); if (i == 0) continue; if (i == 2) break; PrintLine(i + 100); } } PrintLine(mark); return 0; }`, "10\n101\n11\n12\n99\n"},
		{"nested_loop_exits_preserve_outer_scope", `public void Main() { defer PrintLine(9); for (int i = 0; i < 2; i++) { defer PrintLine(i + 10); for (int j = 0; j < 2; j++) { defer PrintLine(j + 20); if (j == 0) continue; break; } PrintLine(i + 30); } PrintLine(40); }`, "20\n21\n30\n10\n20\n21\n31\n11\n40\n9\n"},
		{"while_cleanup_before_condition", `public unsafe void Increment(int* p) { (*p)++; } public void Main() { int x = 0; unsafe { while (x < 3) { defer Increment(&x); PrintLine(x); continue; } } PrintLine(x); }`, "0\n1\n2\n3\n"},
		{"return_from_nested_loops", `public int F() { defer PrintLine(1); for (int i = 0; i < 2; i++) { defer PrintLine(2); while (true) { defer PrintLine(3); { defer PrintLine(4); return 5; } } } return 0; } public void Main() { PrintLine(F()); }`, "4\n3\n2\n1\n5\n"},
	} {
		t.Run(test.name, func(t *testing.T) { t.Parallel(); assertProgram(t, test.source, test.expected) })
	}
}
