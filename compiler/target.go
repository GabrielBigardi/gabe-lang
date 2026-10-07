package compiler

import (
	"runtime"
	"strings"
)

// Target maps a friendly name to an LLVM target triple.
type Target struct{ Alias, Triple string }

// Targets lists named targets. Actual support depends on the installed LLVM and SDKs.
func Targets() []Target {
	return []Target{
		{"windows-x64", "x86_64-pc-windows-msvc"}, {"windows-arm64", "aarch64-pc-windows-msvc"},
		{"linux-x64", "x86_64-unknown-linux-gnu"}, {"linux-arm64", "aarch64-unknown-linux-gnu"},
		{"macos-x64", "x86_64-apple-darwin"}, {"macos-arm64", "arm64-apple-darwin"},
		{"linux-riscv64", "riscv64-unknown-linux-gnu"}, {"wasm32", "wasm32-unknown-unknown"},
	}
}

// HostTarget returns the LLVM triple for the compiler's current OS and architecture.
func HostTarget() string {
	aliases := map[string]string{
		"windows/amd64": "windows-x64", "windows/arm64": "windows-arm64",
		"linux/amd64": "linux-x64", "linux/arm64": "linux-arm64",
		"darwin/amd64": "macos-x64", "darwin/arm64": "macos-arm64",
	}
	if alias, ok := aliases[runtime.GOOS+"/"+runtime.GOARCH]; ok {
		triple, _ := ResolveTarget(alias)
		return triple
	}
	return "unknown"
}

// ResolveTarget accepts an alias or a syntactically valid LLVM target triple.
func ResolveTarget(name string) (string, error) {
	for _, target := range Targets() {
		if target.Alias == name {
			return target.Triple, nil
		}
	}
	valid := strings.Count(name, "-") >= 2
	for i := 0; i < len(name); i++ {
		b := name[i]
		if !asciiLetter(b) && !asciiDigit(b) && !strings.ContainsRune("-_.", rune(b)) {
			valid = false
			break
		}
	}
	if valid {
		return name, nil
	}
	return "", diagnostic(Span{}, "expected a target alias or LLVM target triple; see 'gabe targets'")
}
