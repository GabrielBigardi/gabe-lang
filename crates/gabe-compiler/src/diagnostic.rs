use std::fmt;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct Span {
    pub start: usize,
    pub end: usize,
}

impl Span {
    pub fn new(start: usize, end: usize) -> Self {
        Self { start, end }
    }

    pub fn join(self, other: Self) -> Self {
        Self::new(self.start.min(other.start), self.end.max(other.end))
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Diagnostic {
    pub message: String,
    pub span: Span,
}

impl Diagnostic {
    pub fn new(span: Span, message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            span,
        }
    }

    pub fn render(&self, path: &str, source: &str) -> String {
        let start = self.span.start.min(source.len());
        let prefix = &source[..start];
        let line_number = prefix.bytes().filter(|byte| *byte == b'\n').count() + 1;
        let line_start = prefix.rfind('\n').map_or(0, |index| index + 1);
        let column = source[line_start..start].chars().count() + 1;
        let line = source[line_start..].lines().next().unwrap_or("");
        let end = self.span.end.min(line_start + line.len());
        let width = if end > start {
            source[start..end].chars().count()
        } else {
            1
        };
        format!(
            "{path}:{line_number}:{column}: error: {}\n  {line}\n  {}{}",
            self.message,
            " ".repeat(column - 1),
            "^".repeat(width.max(1))
        )
    }
}

impl fmt::Display for Diagnostic {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.message)
    }
}

impl std::error::Error for Diagnostic {}

pub type Result<T> = std::result::Result<T, Diagnostic>;
