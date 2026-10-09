# Gabe v0.1 language reference

This describes implemented behavior. `.gb` files are UTF-8. C# is the syntax starting point; Gabe does not depend on .NET or reproduce the complete C# language/runtime.

## Program structure

Compile one source file at a time. Functions can appear at file scope or as static methods inside class containers:

```csharp
using System;

class Program
{
    static int Add(int left, int right)
    {
        return left + right;
    }

    public static int Main()
    {
        Console.WriteLine(Add(20, 22));
        return 0;
    }
}
```

Exactly one defined `Main` function must exist. It takes no parameters and returns `int` (process exit code) or `void` (exit code zero). `Main` must be safe. Even analysis and object generation currently require an entry point; library-only compilation is planned.

Classes group static methods only. Methods require `static`, including inside a `static class`. `public`, `private`, and `internal` modifiers are accepted but accessibility is not enforced. There are no fields, object instances, constructors, nested classes, namespaces, or method overloads. `using System;` is accepted; other imports are not implemented. `Console` and `System.Console` output names are reserved.

Unqualified calls inside a class look for a method in that class, then a file-scope function. Qualified calls such as `Program.Add(1, 2)` work. Forward calls and recursion are supported. Identifiers use ASCII letters, digits, and underscores and cannot begin with a digit or be a keyword.

## Types and values

| Type | Representation | Values |
| --- | --- | --- |
| `int` | signed 32-bit integer | decimal literals, e.g. `42` |
| `long` | signed 64-bit integer | `42L` or a literal outside the `int` range |
| `double` | IEEE 754 binary64 | `3.5`, `1e3` |
| `bool` | boolean | `true`, `false` |
| `string` | borrowed immutable NUL-terminated UTF-8 bytes | `"Hello"` |
| `void` | no value | function return type only |
| `T*` | native pointer to `T` | unsafe address expressions |

Integer literal widths are chosen after an immediately preceding unary minus, so `-2147483648` is an `int` and `-9223372036854775808L` is a `long`. Hexadecimal, binary, separators, unsigned integers, and single-precision floats are not implemented.

Implicit numeric conversions are `int` → `long`, `int` → `double`, and `long` → `double`. Narrowing, explicit casts, and boolean/numeric conversions are not implemented. Binary numeric operations widen both operands to the larger type. Large `long` values converted to `double` can lose precision under IEEE rounding.

String literals support `\n`, `\r`, `\t`, `\\`, and `\"`, plus literal Unicode text. Embedded NUL and unescaped newlines are rejected. Strings can be passed and returned, but there is no concatenation, interpolation, indexing, value equality, or allocation. Literal bytes have program lifetime. Reassigning a string local changes its reference.

## Variables and scopes

```csharp
int count = 3;
long total = count;
var factor = 2.5;
count += 1;
```

Every declaration requires an initializer. `var` infers the initializer's type; variables retain that type. Locals and parameters are mutable. Lexical blocks may shadow outer names, including parameters; duplicate names in the same scope are errors. A `for` initializer's locals are limited to that loop. Variables declared inside a conditional or loop body do not leak into surrounding scope.

No global variables, constants, destructuring, multiple declarations in one statement, or uninitialized locals exist yet.

## Operators and evaluation

From higher to lower precedence:

| Operators | Behavior |
| --- | --- |
| postfix `x++`, `x--` | update, return old value |
| prefix `++x`, `--x`, `+`, `-`, `!`, `&`, `*` | update/new value; numeric sign; boolean not; unsafe address/dereference |
| `*`, `/`, `%` | numeric multiplication/division/remainder |
| `+`, `-` | numeric addition/subtraction |
| `<`, `<=`, `>`, `>=` | numeric ordered comparisons |
| `==`, `!=` | numeric, boolean, or equal-type pointer comparisons |
| `&&` | short-circuit boolean and |
| `||` | short-circuit boolean or |
| `=`, `+=`, `-=`, `*=`, `/=`, `%=` | right-associative assignment |

Operands and function arguments evaluate left to right. Assignment computes its target address once, before the right-hand side. Compound assignment loads the old target value before evaluating the right-hand side. The right side of `&&` or `||` is evaluated only when needed. Assignment expressions return the assigned value.

Integer add, subtract, multiply, negate, increment, and decrement wrap in two's complement. Integer division truncates toward zero; remainder has the dividend's sign. **Integer division and remainder trap on a zero divisor or signed minimum divided by -1**, at every optimization level. There are no catchable exceptions.

Floating operations use normal IEEE semantics without fast-math. Floating division by zero can produce infinity/NaN. NaN compares unequal to itself and fails ordered comparisons. Floating `%` lowers to LLVM `frem`.

Conditions require `bool`; numbers and pointers are not truthy. Compound assignment requires the right-hand side to convert to the target type; it does not perform an implicit narrowing cast. Pointer arithmetic, bitwise operators, shifts, ternary expressions, and casts are planned.

## Control flow

```csharp
for (int i = 0; i < 10; i++)
{
    if (i == 2) continue;
    if (i == 5) break;
    Console.WriteLine(i);
}

int remaining = 3;
while (remaining > 0)
{
    remaining--;
}
```

`if` / `else`, `while`, `for`, nested blocks, `break`, `continue`, and `return` are supported. The `for` initializer, condition, and step can be omitted. `continue` in a `for` runs the step before checking the condition again. `break` exits the innermost loop.

A non-void function must return a value on all analyzed paths. Analysis is conservative about loops: a return after a loop may be required even for an obviously infinite loop. A void function may fall through. Expression statements must be calls, assignments, or updates.

## Deferred calls

```csharp
defer DoSomething();

{
    int value = 1;
    defer Console.WriteLine(value);
    defer Console.WriteLine("second cleanup");
    value = 2;
    Console.WriteLine("body");
}
// Prints body, then second cleanup, then 1.
// DoSomething runs when the surrounding scope exits.
```

`defer` is a reserved keyword. A deferred statement requires a function call and a trailing semicolon. Defined functions, static methods, extern calls, and console output are supported; any returned value is discarded. Argument expressions and their implicit conversions run immediately, once, from left to right. Their values are saved, and the call runs later. Only reached `defer` statements register a call.

Calls run in reverse registration order when the **current lexical scope exits**. A nested `{ ... }` block runs its own defers before execution continues in the surrounding block. An `unsafe { ... }` block also has its own scope. A conditional or loop body without braces has an implicit scope, so its defers run when that body finishes. Each loop iteration cleans up independently.

Normal fallthrough, `return`, `break`, and `continue` run cleanup for every scope they leave, from innermost to outermost. `break` and `continue` preserve defers in scopes surrounding the loop. Cleanup precedes the next `for` step or loop condition. A return expression is evaluated before cleanup, so a deferred mutation cannot change an already computed return value.

Like Go, Gabe captures arguments at registration and calls defers in reverse order. Unlike Go's function-exit rule, Gabe runs them at block/scope exit. There are no deferred blocks, closures, or catchable exceptions. Integer traps, fatal faults, and process termination do not unwind defers.

Ordinary type and unsafe checks still apply at the `defer` statement. Saving a pointer copies its address, not its pointee; changes to the pointee remain visible to the deferred call. `defer` does not extend a pointee's lifetime or add borrow checking. Unsafe code must keep captured addresses valid until the call finishes.

See [examples/defer.gb](../examples/defer.gb) for a runnable program and [ADR 0003](decisions/0003-scope-defer.md) for the lowering design.

## Console output

`Console.Write(value)` and `Console.WriteLine(value)` accept one `int`, `long`, `double`, `bool`, or `string`. `Console.WriteLine()` writes just a newline. `System.Console` is also supported. Output calls return `void`.

Integers print in decimal, booleans as `True` / `False`, and doubles using the C runtime's `%.17g` format. Output uses the platform C runtime and its locale. Unicode string literals are emitted as UTF-8; terminal rendering depends on the terminal. Percent signs in strings are printed literally. Output redirection works through ordinary process standard output.

## Unsafe pointers and C interop

```csharp
extern int abs(int value);

int Main()
{
    int value = -21;
    unsafe
    {
        int* pointer = &value;
        *pointer = abs(*pointer) * 2;
    }
    Console.WriteLine(value);
    return 0;
}
```

Pointer declarations, pointer value access, address-of, dereference, and calls to unsafe functions or externs require an `unsafe` block or an `unsafe` function. `unsafe` can modify a class to give its static methods unsafe bodies. An unsafe method must still be called from an unsafe context.

`&` accepts a local or dereferenced pointer; `*` requires a typed non-void pointer. Pointer assignments and equality require matching pointee types. `void*` can be passed/stored inside unsafe code but cannot be dereferenced. Pointer arithmetic, null literals, integer/pointer casts, allocation, and lifetime/borrow checking are not implemented.

Unsafe code must uphold pointer validity, pointee layout, initialization, and lifetime. Returning a pointer to a local or retaining an address after its source lifetime is invalid even though the current checker cannot detect it. This first version has no claim of Rust-like memory safety.

File-scope `extern` declarations use the platform C ABI for scalar values and pointers. `int` maps to 32-bit C integers, Gabe `long` to a 64-bit integer such as `int64_t` (not C `long` on every OS), `double` to binary64, and `string` to a NUL-terminated UTF-8 pointer. Extern boolean parameters/results, aggregates, variadics, and calling-convention selection are deferred. A returned external string must be a valid live NUL-terminated pointer before any safe output uses it.

The linker must provide the external symbols. C runtime symbols such as `abs` are available in the default executable link; other libraries require `--link-arg`. Symbols `main`, `printf`, `puts`, and the `gb_` prefix are reserved for the compiler/runtime.

## Known boundaries

This version compiles one file and links executables through external tools. It has no structs, arrays, objects, generics, modules, references, ownership system, standard library packages, async functions, exceptions, debugger source metadata, or language server. These are roadmap items, not accepted-but-unimplemented syntax.
