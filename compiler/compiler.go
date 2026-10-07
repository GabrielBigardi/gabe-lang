// Package compiler checks Gabe source and emits deterministic LLVM IR.
// It needs no LLVM installation, cgo, or third-party Go dependencies.
package compiler

// Check lexes, parses, and analyzes one UTF-8 Gabe source file.
func Check(source string) (*Program, error) {
	tokens, err := lex(source)
	if err != nil {
		return nil, err
	}
	parsed, err := parse(tokens)
	if err != nil {
		return nil, err
	}
	return analyze(parsed)
}

// Compile returns LLVM IR for a target alias or triple after semantic analysis.
func Compile(source, target string) (string, error) {
	triple, err := ResolveTarget(target)
	if err != nil {
		return "", err
	}
	program, err := Check(source)
	if err != nil {
		return "", err
	}
	return emitLLVM(program, triple), nil
}
