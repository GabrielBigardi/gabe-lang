// Package cli implements the Gabe driver and external LLVM tool invocation.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/GabrielBigardi/gabe-lang/compiler"
)

const help = `Gabe 0.1.0 — gabe-lang native compiler

Usage:
  gabe check <file.gb>
  gabe build <file.gb> [options]
  gabe run <file.gb> [options] [-- program arguments]
  gabe targets

Options:
  -o, --output <path>       Output path (default: build/<name>)
  --emit <exe|obj|llvm>     Native executable, object file, or LLVM IR
  --target <alias|triple>   LLVM target (default: host)
  -O0, -O1, -O2, -O3       Optimization level (default: -O0)
  --release                Equivalent to -O3
  --clang <path>           Clang driver (or GABE_CLANG / clang on PATH)
  --llc <path>             LLVM object backend (or GABE_LLC); Windows can link via MSVC
  --linker <path>          MSVC-compatible linker for the llc Windows backend
  --sysroot <path>         Target SDK/sysroot for Clang
  --link-arg <argument>    Extra linker argument; repeat as needed
  -h, --help               Show this help
  -V, --version            Show version
`

type emitKind string

const (
	emitExe    emitKind = "exe"
	emitObject emitKind = "obj"
	emitLLVM   emitKind = "llvm"
)

type options struct {
	source, output              string
	emit                        emitKind
	triple                      string
	optimization                int
	clang, llc, linker, sysroot string
	linkArgs, runArgs           []string
}

// Main runs the compiler CLI and returns a process exit code.
func Main(arguments []string, stdout, stderr io.Writer) int {
	code, err := driver(arguments, stdout, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return code
}
func driver(arguments []string, stdout, stderr io.Writer) (int, error) {
	if len(arguments) == 0 {
		fmt.Fprint(stdout, help)
		return 0, nil
	}
	command, rest := arguments[0], arguments[1:]
	switch command {
	case "help", "--help", "-h":
		fmt.Fprint(stdout, help)
		return 0, nil
	case "--version", "-V", "version":
		fmt.Fprintln(stdout, "gabe 0.1.0")
		return 0, nil
	case "targets":
		fmt.Fprintf(stdout, "Host: %s\n\n", compiler.HostTarget())
		for _, target := range compiler.Targets() {
			fmt.Fprintf(stdout, "%-18s %s\n", target.Alias, target.Triple)
		}
		fmt.Fprintln(stdout, "\nOther LLVM triples are accepted when your LLVM build supports them.\nCross-linking requires the target SDK, runtime, and linker.")
		return 0, nil
	case "check", "build", "run":
	default:
		return 0, fmt.Errorf("unknown command '%s'; run 'gabe --help'", command)
	}
	for _, argument := range rest {
		if argument == "--help" || argument == "-h" {
			fmt.Fprint(stdout, help)
			return 0, nil
		}
	}
	opts, err := parseOptions(command, rest)
	if err != nil {
		return 0, err
	}
	bytes, err := os.ReadFile(opts.source)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", opts.source, err)
	}
	source := string(bytes)
	if command == "check" {
		if _, err := compiler.Check(source); err != nil {
			return 0, sourceError(err, opts.source, source)
		}
		fmt.Fprintf(stdout, "Checked %s\n", opts.source)
		return 0, nil
	}
	if command == "run" && opts.triple != compiler.HostTarget() {
		return 0, errors.New("gabe run requires the host target; use gabe build for cross-compilation")
	}
	ir, err := compiler.Compile(source, opts.triple)
	if err != nil {
		return 0, sourceError(err, opts.source, source)
	}
	output := outputPath(opts)
	sourceInfo, err := os.Stat(opts.source)
	if err != nil {
		return 0, err
	}
	if outputInfo, err := os.Stat(output); err == nil && os.SameFile(sourceInfo, outputInfo) {
		return 0, errors.New("output path would overwrite the source file")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return 0, fmt.Errorf("cannot create output directory: %w", err)
	}
	if opts.emit == emitLLVM {
		if err := os.WriteFile(output, []byte(ir), 0o644); err != nil {
			return 0, fmt.Errorf("cannot write LLVM IR: %w", err)
		}
	} else if err := nativeBuild(opts, ir, output); err != nil {
		return 0, err
	}
	if command != "run" {
		fmt.Fprintf(stdout, "Built %s (%s)\n", output, opts.triple)
		return 0, nil
	}
	executable, err := filepath.Abs(output)
	if err != nil {
		return 0, fmt.Errorf("cannot find built executable: %w", err)
	}
	program := exec.Command(executable, opts.runArgs...)
	program.Stdin, program.Stdout, program.Stderr = os.Stdin, stdout, stderr
	if err := program.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			code := exitError.ExitCode()
			if code > 0 && code <= 255 {
				return code, nil
			}
			return 1, nil
		}
		return 0, fmt.Errorf("cannot run program: %w", err)
	}
	return 0, nil
}
func sourceError(err error, path, source string) error {
	var diagnostic *compiler.Diagnostic
	if errors.As(err, &diagnostic) {
		return errors.New(diagnostic.Render(path, source))
	}
	return err
}
func nativeBuild(opts options, ir, output string) error {
	temporary, err := os.MkdirTemp("", "gabe-*")
	if err != nil {
		return fmt.Errorf("cannot create temporary directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	input := filepath.Join(temporary, "module.ll")
	if err := os.WriteFile(input, []byte(ir), 0o644); err != nil {
		return fmt.Errorf("cannot write temporary LLVM IR: %w", err)
	}
	if opts.llc != "" {
		if opts.sysroot != "" {
			return errors.New("--sysroot is supported by the Clang backend; configure MSVC/SDK libraries for llc")
		}
		object := output
		if opts.emit == emitExe {
			object = filepath.Join(temporary, "module.obj")
		}
		_, err := execute(exec.Command(opts.llc, "-filetype=obj", "-mtriple="+opts.triple, fmt.Sprintf("-O=%d", opts.optimization), input, "-o", object), "LLVM llc")
		if err != nil {
			return err
		}
		if opts.emit == emitExe {
			if runtime.GOOS != "windows" || !strings.Contains(opts.triple, "windows-msvc") {
				return errors.New("the llc executable linker currently supports Windows/MSVC; use Clang for other executables or --emit obj")
			}
			return linkMSVC(object, output, opts.triple, opts.linker, opts.linkArgs)
		}
		return nil
	}
	arguments := []string{"-x", "ir", input, "--target=" + opts.triple, fmt.Sprintf("-O%d", opts.optimization), "-o", output}
	if opts.emit == emitObject {
		arguments = append(arguments, "-c")
	}
	if opts.sysroot != "" {
		arguments = append(arguments, "--sysroot", opts.sysroot)
	}
	if opts.emit == emitExe {
		arguments = append(arguments, opts.linkArgs...)
		// LLVM can lower floating remainder to fmod; glibc keeps it in libm.
		if strings.Contains(opts.triple, "linux") {
			arguments = append(arguments, "-lm")
		}
		// Direct LLVM calls bypass the CRT headers' inline printf definition.
		if strings.Contains(opts.triple, "windows-msvc") {
			arguments = append(arguments, "-llegacy_stdio_definitions")
		}
	}
	_, err = execute(exec.Command(opts.clang, arguments...), "Clang (install LLVM 17+, set GABE_CLANG, or pass --clang; LLVM llc is also supported with --llc)")
	return err
}
func parseOptions(command string, arguments []string) (options, error) {
	opts := options{emit: emitExe, triple: compiler.HostTarget(), clang: os.Getenv("GABE_CLANG"), llc: os.Getenv("GABE_LLC")}
	if opts.clang == "" {
		opts.clang = "clang"
	}
	for i := 0; i < len(arguments); i++ {
		argument := arguments[i]
		switch argument {
		case "--output", "-o", "--target", "--emit", "--clang", "--llc", "--linker", "--sysroot", "--link-arg":
			if i+1 == len(arguments) {
				return opts, fmt.Errorf("%s requires a value", argument)
			}
			i++
			value := arguments[i]
			switch argument {
			case "--output", "-o":
				opts.output = value
			case "--target":
				triple, err := compiler.ResolveTarget(value)
				if err != nil {
					return opts, err
				}
				opts.triple = triple
			case "--emit":
				if value != "llvm" && value != "obj" && value != "exe" {
					return opts, fmt.Errorf("unknown output kind '%s'; use exe, obj, or llvm", value)
				}
				opts.emit = emitKind(value)
			case "--clang":
				opts.clang, opts.llc = value, ""
			case "--llc":
				opts.llc = value
			case "--linker":
				opts.linker = value
			case "--sysroot":
				opts.sysroot = value
			case "--link-arg":
				opts.linkArgs = append(opts.linkArgs, value)
			}
		case "--release", "-O3":
			opts.optimization = 3
		case "-O0":
			opts.optimization = 0
		case "-O1":
			opts.optimization = 1
		case "-O2":
			opts.optimization = 2
		case "--":
			if command != "run" {
				return opts, errors.New("unknown option '--'")
			}
			opts.runArgs = arguments[i+1:]
			i = len(arguments)
		default:
			if strings.HasPrefix(argument, "-") {
				return opts, fmt.Errorf("unknown option '%s'", argument)
			}
			if opts.source != "" {
				return opts, errors.New("expected exactly one .gb source file")
			}
			opts.source = argument
		}
	}
	if opts.source == "" {
		return opts, errors.New("expected a .gb source file; run 'gabe --help'")
	}
	if filepath.Ext(opts.source) != ".gb" {
		return opts, errors.New("Gabe source files must use the .gb extension")
	}
	if opts.triple == "unknown" {
		return opts, errors.New("this host needs an explicit --target LLVM triple")
	}
	if command == "run" && opts.emit != emitExe {
		return opts, errors.New("gabe run requires --emit exe")
	}
	if opts.linker != "" && opts.llc == "" {
		return opts, errors.New("--linker applies to the llc backend; use --link-arg with Clang")
	}
	if opts.emit != emitExe && len(opts.linkArgs) > 0 {
		return opts, errors.New("--link-arg requires --emit exe")
	}
	if command == "check" && (opts.output != "" || opts.sysroot != "" || len(opts.linkArgs) > 0 || len(opts.runArgs) > 0) {
		return opts, errors.New("gabe check only analyzes a source file")
	}
	return opts, nil
}
func outputPath(opts options) string {
	if opts.output != "" {
		return opts.output
	}
	name := strings.TrimSuffix(filepath.Base(opts.source), filepath.Ext(opts.source))
	extension := ""
	switch opts.emit {
	case emitLLVM:
		extension = ".ll"
	case emitObject:
		extension = ".o"
		if strings.Contains(opts.triple, "windows") {
			extension = ".obj"
		}
	case emitExe:
		if strings.Contains(opts.triple, "windows") {
			extension = ".exe"
		}
	}
	return filepath.Join("build", name+extension)
}
