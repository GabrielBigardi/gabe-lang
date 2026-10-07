mod toolchain;

use gabe_compiler::target;
use std::env;
use std::fs;
use std::path::PathBuf;
use std::process::{Command, ExitCode};
use std::time::{SystemTime, UNIX_EPOCH};

const HELP: &str = "Gabe 0.1.0 — gabe-lang native compiler\n\nUsage:\n  gabe check <file.gb>\n  gabe build <file.gb> [options]\n  gabe run <file.gb> [options] [-- program arguments]\n  gabe targets\n\nOptions:\n  -o, --output <path>       Output path (default: build/<name>)\n  --emit <exe|obj|llvm>     Native executable, object file, or LLVM IR\n  --target <alias|triple>   LLVM target (default: host)\n  -O0, -O1, -O2, -O3       Optimization level (default: -O0)\n  --release                Equivalent to -O3\n  --clang <path>           Clang driver (or GABE_CLANG / clang on PATH)\n  --llc <path>             LLVM object backend (or GABE_LLC); Windows can link via MSVC\n  --linker <path>          MSVC-compatible linker for the llc Windows backend\n  --sysroot <path>         Target SDK/sysroot for Clang\n  --link-arg <argument>    Extra linker argument; repeat as needed\n  -h, --help               Show this help\n  -V, --version            Show version\n";

#[derive(Clone, Copy, PartialEq, Eq)]
enum Emit {
    Exe,
    Object,
    Llvm,
}

struct Options {
    source: PathBuf,
    output: Option<PathBuf>,
    emit: Emit,
    triple: String,
    optimization: u8,
    clang: PathBuf,
    llc: Option<PathBuf>,
    linker: Option<PathBuf>,
    sysroot: Option<PathBuf>,
    link_args: Vec<String>,
    run_args: Vec<String>,
}

fn main() -> ExitCode {
    match driver() {
        Ok(code) => ExitCode::from(code),
        Err(error) => {
            eprintln!("{error}");
            ExitCode::FAILURE
        }
    }
}

fn driver() -> Result<u8, String> {
    let mut arguments = env::args().skip(1);
    let Some(command) = arguments.next() else {
        print!("{HELP}");
        return Ok(0);
    };
    match command.as_str() {
        "help" | "--help" | "-h" => {
            print!("{HELP}");
            return Ok(0);
        }
        "--version" | "-V" | "version" => {
            println!("gabe {}", env!("CARGO_PKG_VERSION"));
            return Ok(0);
        }
        "targets" => {
            println!("Host: {}\n", target::host());
            for (alias, triple) in target::TARGETS {
                println!("{alias:18} {triple}");
            }
            println!(
                "\nOther LLVM triples are accepted when your LLVM build supports them.\nCross-linking requires the target SDK, runtime, and linker."
            );
            return Ok(0);
        }
        "check" | "build" | "run" => {}
        _ => return Err(format!("unknown command '{command}'; run 'gabe --help'")),
    }
    let rest = arguments.collect::<Vec<_>>();
    if rest.iter().any(|arg| arg == "--help" || arg == "-h") {
        print!("{HELP}");
        return Ok(0);
    }
    let options = parse_options(&command, rest)?;
    let source = fs::read_to_string(&options.source)
        .map_err(|error| format!("cannot read {}: {error}", options.source.display()))?;
    if command == "check" {
        gabe_compiler::check(&source).map_err(|diagnostic| {
            diagnostic.render(&options.source.display().to_string(), &source)
        })?;
        println!("Checked {}", options.source.display());
        return Ok(0);
    }
    if command == "run" && options.triple != target::host() {
        return Err(
            "gabe run requires the host target; use gabe build for cross-compilation".to_owned(),
        );
    }
    let ir = gabe_compiler::compile(&source, &options.triple)
        .map_err(|diagnostic| diagnostic.render(&options.source.display().to_string(), &source))?;
    let output = output_path(&options)?;
    if output.exists() && fs::canonicalize(&output).ok() == fs::canonicalize(&options.source).ok() {
        return Err("output path would overwrite the source file".to_owned());
    }
    if let Some(parent) = output
        .parent()
        .filter(|parent| !parent.as_os_str().is_empty())
    {
        fs::create_dir_all(parent)
            .map_err(|error| format!("cannot create output directory: {error}"))?;
    }
    if options.emit == Emit::Llvm {
        fs::write(&output, ir).map_err(|error| format!("cannot write LLVM IR: {error}"))?;
    } else {
        let temporary = TemporaryDirectory::new()?;
        let input = temporary.0.join("module.ll");
        fs::write(&input, ir)
            .map_err(|error| format!("cannot write temporary LLVM IR: {error}"))?;
        if let Some(llc) = &options.llc {
            if options.sysroot.is_some() {
                return Err("--sysroot is supported by the Clang backend; configure MSVC/SDK libraries for llc".to_owned());
            }
            let object = if options.emit == Emit::Object {
                output.clone()
            } else {
                temporary.0.join("module.obj")
            };
            let mut backend = Command::new(llc);
            backend
                .arg("-filetype=obj")
                .arg(format!("-mtriple={}", options.triple))
                .arg(format!("-O={}", options.optimization))
                .arg(&input)
                .arg("-o")
                .arg(&object);
            toolchain::execute(&mut backend, "LLVM llc")?;
            if options.emit == Emit::Exe {
                if !cfg!(windows) || !options.triple.contains("windows-msvc") {
                    return Err("the llc executable linker currently supports Windows/MSVC; use Clang for other executables or --emit obj".to_owned());
                }
                toolchain::link_msvc(
                    &object,
                    &output,
                    &options.triple,
                    options.linker.as_deref(),
                    &options.link_args,
                )?;
            }
        } else {
            let mut clang = Command::new(&options.clang);
            clang
                .arg("-x")
                .arg("ir")
                .arg(&input)
                .arg(format!("--target={}", options.triple))
                .arg(format!("-O{}", options.optimization))
                .arg("-o")
                .arg(&output);
            if options.emit == Emit::Object {
                clang.arg("-c");
            }
            if let Some(sysroot) = &options.sysroot {
                clang.arg("--sysroot").arg(sysroot);
            }
            if options.emit == Emit::Exe {
                clang.args(&options.link_args);
                // LLVM can lower floating remainder to fmod; glibc keeps it in libm.
                if options.triple.contains("linux") {
                    clang.arg("-lm");
                }
            }
            toolchain::execute(
                &mut clang,
                "Clang (install LLVM 17+, set GABE_CLANG, or pass --clang; LLVM llc is also supported with --llc)",
            )?;
        }
    }
    if command != "run" {
        println!("Built {} ({})", output.display(), options.triple);
        return Ok(0);
    }
    let executable = fs::canonicalize(&output)
        .map_err(|error| format!("cannot find built executable: {error}"))?;
    let status = Command::new(executable)
        .args(&options.run_args)
        .status()
        .map_err(|error| format!("cannot run program: {error}"))?;
    // Unix only exposes 8-bit exit codes; Windows failures also remain failures.
    Ok(status.code().map_or(1, |code| {
        if code == 0 {
            0
        } else {
            u8::try_from(code)
                .ok()
                .filter(|code| *code != 0)
                .unwrap_or(1)
        }
    }))
}

fn parse_options(command: &str, arguments: Vec<String>) -> Result<Options, String> {
    let mut source = None;
    let mut options = Options {
        source: PathBuf::new(),
        output: None,
        emit: Emit::Exe,
        triple: target::host().to_owned(),
        optimization: 0,
        clang: env::var_os("GABE_CLANG").map_or_else(|| PathBuf::from("clang"), PathBuf::from),
        llc: env::var_os("GABE_LLC").map(PathBuf::from),
        linker: None,
        sysroot: None,
        link_args: Vec::new(),
        run_args: Vec::new(),
    };
    let mut arguments = arguments.into_iter();
    while let Some(argument) = arguments.next() {
        let mut value = || {
            arguments
                .next()
                .ok_or_else(|| format!("{argument} requires a value"))
        };
        match argument.as_str() {
            "--output" | "-o" => options.output = Some(PathBuf::from(value()?)),
            "--target" => {
                options.triple =
                    target::resolve(&value()?).map_err(|diagnostic| diagnostic.message)?
            }
            "--emit" => {
                options.emit = match value()?.as_str() {
                    "llvm" => Emit::Llvm,
                    "obj" => Emit::Object,
                    "exe" => Emit::Exe,
                    other => {
                        return Err(format!(
                            "unknown output kind '{other}'; use exe, obj, or llvm"
                        ));
                    }
                }
            }
            "--release" => options.optimization = 3,
            "-O0" => options.optimization = 0,
            "-O1" => options.optimization = 1,
            "-O2" => options.optimization = 2,
            "-O3" => options.optimization = 3,
            "--clang" => {
                options.clang = PathBuf::from(value()?);
                options.llc = None;
            }
            "--llc" => options.llc = Some(PathBuf::from(value()?)),
            "--linker" => options.linker = Some(PathBuf::from(value()?)),
            "--sysroot" => options.sysroot = Some(PathBuf::from(value()?)),
            "--link-arg" => options.link_args.push(value()?),
            "--" if command == "run" => {
                options.run_args.extend(arguments);
                break;
            }
            option if option.starts_with('-') => return Err(format!("unknown option '{option}'")),
            path => {
                if source.replace(PathBuf::from(path)).is_some() {
                    return Err("expected exactly one .gb source file".to_owned());
                }
            }
        }
    }
    options.source = source.ok_or("expected a .gb source file; run 'gabe --help'")?;
    if options.source.extension().and_then(|value| value.to_str()) != Some("gb") {
        return Err("Gabe source files must use the .gb extension".to_owned());
    }
    if options.triple == "unknown" {
        return Err("this host needs an explicit --target LLVM triple".to_owned());
    }
    if command == "run" && options.emit != Emit::Exe {
        return Err("gabe run requires --emit exe".to_owned());
    }
    if options.linker.is_some() && options.llc.is_none() {
        return Err("--linker applies to the llc backend; use --link-arg with Clang".to_owned());
    }
    if options.emit != Emit::Exe && !options.link_args.is_empty() {
        return Err("--link-arg requires --emit exe".to_owned());
    }
    if command == "check"
        && (options.output.is_some()
            || options.sysroot.is_some()
            || !options.link_args.is_empty()
            || !options.run_args.is_empty())
    {
        return Err("gabe check only analyzes a source file".to_owned());
    }
    Ok(options)
}

fn output_path(options: &Options) -> Result<PathBuf, String> {
    if let Some(output) = &options.output {
        return Ok(output.clone());
    }
    let stem = options
        .source
        .file_stem()
        .ok_or("source path has no filename")?;
    let mut filename = stem.to_os_string();
    let extension = match options.emit {
        Emit::Llvm => "ll",
        Emit::Object if options.triple.contains("windows") => "obj",
        Emit::Object => "o",
        Emit::Exe if options.triple.contains("windows") => "exe",
        Emit::Exe => "",
    };
    if !extension.is_empty() {
        filename.push(".");
        filename.push(extension);
    }
    Ok(PathBuf::from("build").join(filename))
}

struct TemporaryDirectory(PathBuf);

impl TemporaryDirectory {
    fn new() -> Result<Self, String> {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_err(|error| error.to_string())?
            .as_nanos();
        let path = env::temp_dir().join(format!("gabe-{}-{now}", std::process::id()));
        fs::create_dir(&path)
            .map_err(|error| format!("cannot create temporary directory: {error}"))?;
        Ok(Self(path))
    }
}

impl Drop for TemporaryDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
