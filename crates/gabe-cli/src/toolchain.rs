use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};

pub fn execute(command: &mut Command, tool: &str) -> Result<Output, String> {
    let output = command
        .output()
        .map_err(|error| format!("could not start {tool}: {error}"))?;
    if !output.status.success() {
        return Err(format!(
            "{tool} failed ({}):\n{}{}",
            output.status,
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        ));
    }
    Ok(output)
}

pub fn link_msvc(
    object: &Path,
    output: &Path,
    triple: &str,
    linker: Option<&Path>,
    arguments: &[String],
) -> Result<(), String> {
    let arch = if triple.starts_with("aarch64") || triple.starts_with("arm64") {
        "arm64"
    } else if triple.starts_with("x86_64") {
        "x64"
    } else {
        return Err("the llc/MSVC linker path currently supports x64 and ARM64 targets".to_owned());
    };
    let program_files = env::var_os("ProgramFiles(x86)")
        .ok_or("cannot find ProgramFiles(x86); use Clang or build an object file")?;
    let vswhere =
        PathBuf::from(&program_files).join("Microsoft Visual Studio/Installer/vswhere.exe");
    let installation = execute(
        Command::new(vswhere).args([
            "-latest",
            "-products",
            "*",
            "-requires",
            "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
            "-property",
            "installationPath",
        ]),
        "vswhere (Visual Studio C++ discovery)",
    )?;
    let installation = String::from_utf8_lossy(&installation.stdout)
        .trim()
        .to_owned();
    if installation.is_empty() {
        return Err("install Visual Studio Build Tools with 'Desktop development with C++' and a Windows SDK".to_owned());
    }
    let vc = PathBuf::from(installation).join("VC");
    let version =
        fs::read_to_string(vc.join("Auxiliary/Build/Microsoft.VCToolsVersion.default.txt"))
            .map_err(|error| format!("cannot discover MSVC tool version: {error}"))?;
    let msvc = vc.join("Tools/MSVC").join(version.trim());
    let sdk = env::var_os("WindowsSdkDir")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from(program_files).join("Windows Kits/10"));
    let mut versions = fs::read_dir(sdk.join("Lib"))
        .map_err(|error| format!("cannot find Windows SDK libraries: {error}"))?
        .filter_map(|entry| entry.ok())
        .map(|entry| entry.path())
        .filter(|path| path.join("ucrt").join(arch).is_dir() && path.join("um").join(arch).is_dir())
        .collect::<Vec<_>>();
    versions.sort_by_key(|path| {
        path.file_name()
            .unwrap_or_default()
            .to_string_lossy()
            .split('.')
            .filter_map(|part| part.parse::<u32>().ok())
            .collect::<Vec<_>>()
    });
    let sdk_lib = versions
        .last()
        .ok_or_else(|| format!("Windows SDK libraries for {arch} are missing"))?;
    let host = if env::consts::ARCH == "aarch64" {
        "HostARM64"
    } else {
        "Hostx64"
    };
    let default_linker = msvc.join("bin").join(host).join(arch).join("link.exe");
    let mut command = Command::new(linker.unwrap_or(&default_linker));
    command
        .args(["/NOLOGO", "/INCREMENTAL:NO", "/SUBSYSTEM:CONSOLE"])
        .arg(format!("/OUT:{}", output.display()))
        .arg(object);
    for library_path in [
        msvc.join("lib").join(arch),
        sdk_lib.join("ucrt").join(arch),
        sdk_lib.join("um").join(arch),
    ] {
        command.arg(format!("/LIBPATH:{}", library_path.display()));
    }
    command.args([
        "libcmt.lib",
        "libvcruntime.lib",
        "libucrt.lib",
        "kernel32.lib",
        "legacy_stdio_definitions.lib",
    ]);
    command.args(arguments);
    execute(&mut command, "MSVC linker")?;
    Ok(())
}
