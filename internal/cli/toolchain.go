package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

func execute(command *exec.Cmd, tool string) ([]byte, error) {
	output, err := command.CombinedOutput()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("%s failed (%s):\n%s", tool, exitError, output)
		}
		return nil, fmt.Errorf("could not start %s: %w", tool, err)
	}
	return output, nil
}
func linkMSVC(object, output, triple, linker string, arguments []string) error {
	arch := ""
	switch {
	case strings.HasPrefix(triple, "aarch64"), strings.HasPrefix(triple, "arm64"):
		arch = "arm64"
	case strings.HasPrefix(triple, "x86_64"):
		arch = "x64"
	default:
		return errors.New("the llc/MSVC linker path currently supports x64 and ARM64 targets")
	}
	programFiles := os.Getenv("ProgramFiles(x86)")
	if programFiles == "" {
		return errors.New("cannot find ProgramFiles(x86); use Clang or build an object file")
	}
	vswhere := filepath.Join(programFiles, "Microsoft Visual Studio", "Installer", "vswhere.exe")
	installation, err := execute(exec.Command(vswhere, "-latest", "-products", "*", "-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "-property", "installationPath"), "vswhere (Visual Studio C++ discovery)")
	if err != nil {
		return err
	}
	installationPath := strings.TrimSpace(string(installation))
	if installationPath == "" {
		return errors.New("install Visual Studio Build Tools with 'Desktop development with C++' and a Windows SDK")
	}
	vc := filepath.Join(installationPath, "VC")
	version, err := os.ReadFile(filepath.Join(vc, "Auxiliary", "Build", "Microsoft.VCToolsVersion.default.txt"))
	if err != nil {
		return fmt.Errorf("cannot discover MSVC tool version: %w", err)
	}
	msvc := filepath.Join(vc, "Tools", "MSVC", strings.TrimSpace(string(version)))
	sdk := os.Getenv("WindowsSdkDir")
	if sdk == "" {
		sdk = filepath.Join(programFiles, "Windows Kits", "10")
	}
	entries, err := os.ReadDir(filepath.Join(sdk, "Lib"))
	if err != nil {
		return fmt.Errorf("cannot find Windows SDK libraries: %w", err)
	}
	var versions []string
	for _, entry := range entries {
		path := filepath.Join(sdk, "Lib", entry.Name())
		if isDirectory(filepath.Join(path, "ucrt", arch)) && isDirectory(filepath.Join(path, "um", arch)) {
			versions = append(versions, path)
		}
	}
	slices.SortFunc(versions, func(a, b string) int { return slices.Compare(versionNumbers(a), versionNumbers(b)) })
	if len(versions) == 0 {
		return fmt.Errorf("Windows SDK libraries for %s are missing", arch)
	}
	sdkLib := versions[len(versions)-1]
	if linker == "" {
		host := "Hostx64"
		if runtime.GOARCH == "arm64" {
			host = "HostARM64"
		}
		linker = filepath.Join(msvc, "bin", host, arch, "link.exe")
	}
	args := []string{"/NOLOGO", "/INCREMENTAL:NO", "/SUBSYSTEM:CONSOLE", "/OUT:" + output, object}
	for _, libraryPath := range []string{filepath.Join(msvc, "lib", arch), filepath.Join(sdkLib, "ucrt", arch), filepath.Join(sdkLib, "um", arch)} {
		args = append(args, "/LIBPATH:"+libraryPath)
	}
	args = append(args, "libcmt.lib", "libvcruntime.lib", "libucrt.lib", "kernel32.lib", "legacy_stdio_definitions.lib")
	args = append(args, arguments...)
	_, err = execute(exec.Command(linker, args...), "MSVC linker")
	return err
}
func isDirectory(path string) bool { info, err := os.Stat(path); return err == nil && info.IsDir() }
func versionNumbers(path string) []int {
	var numbers []int
	for _, part := range strings.Split(filepath.Base(path), ".") {
		if number, err := strconv.Atoi(part); err == nil {
			numbers = append(numbers, number)
		}
	}
	return numbers
}
