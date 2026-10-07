use crate::diagnostic::{Diagnostic, Result, Span};

#[derive(Debug, Clone, PartialEq)]
pub enum Kind {
    Word(String),
    Number(String),
    String(String),
    Symbol(String),
    Eof,
}

#[derive(Debug, Clone)]
pub struct Token {
    pub kind: Kind,
    pub span: Span,
}

pub fn lex(source: &str) -> Result<Vec<Token>> {
    let bytes = source.as_bytes();
    let mut position = 0;
    let mut tokens = Vec::new();
    while position < bytes.len() {
        let start = position;
        let byte = bytes[position];
        if byte.is_ascii_whitespace() {
            position += 1;
            continue;
        }
        if source[position..].starts_with("//") {
            while position < bytes.len() && bytes[position] != b'\n' {
                position += 1;
            }
            continue;
        }
        if source[position..].starts_with("/*") {
            let Some(end) = source[position + 2..].find("*/") else {
                return Err(Diagnostic::new(
                    Span::new(start, start + 2),
                    "unterminated block comment",
                ));
            };
            position += end + 4;
            continue;
        }
        let kind = if byte.is_ascii_alphabetic() || byte == b'_' {
            position += 1;
            while position < bytes.len()
                && (bytes[position].is_ascii_alphanumeric() || bytes[position] == b'_')
            {
                position += 1;
            }
            Kind::Word(source[start..position].to_owned())
        } else if byte.is_ascii_digit() {
            position += 1;
            while position < bytes.len() && bytes[position].is_ascii_digit() {
                position += 1;
            }
            if position + 1 < bytes.len()
                && bytes[position] == b'.'
                && bytes[position + 1].is_ascii_digit()
            {
                position += 1;
                while position < bytes.len() && bytes[position].is_ascii_digit() {
                    position += 1;
                }
            }
            if position < bytes.len() && matches!(bytes[position], b'e' | b'E') {
                position += 1;
                if position < bytes.len() && matches!(bytes[position], b'+' | b'-') {
                    position += 1;
                }
                let digits = position;
                while position < bytes.len() && bytes[position].is_ascii_digit() {
                    position += 1;
                }
                if digits == position {
                    return Err(Diagnostic::new(
                        Span::new(start, position),
                        "expected exponent digits",
                    ));
                }
            }
            if position < bytes.len() && matches!(bytes[position], b'l' | b'L') {
                position += 1;
            }
            Kind::Number(source[start..position].to_owned())
        } else if byte == b'"' {
            position += 1;
            let mut value = String::new();
            let mut closed = false;
            while position < bytes.len() {
                let character = source[position..].chars().next().expect("valid UTF-8");
                position += character.len_utf8();
                match character {
                    '"' => {
                        closed = true;
                        break;
                    }
                    '\n' | '\r' => {
                        return Err(Diagnostic::new(
                            Span::new(start, position),
                            "newline in string literal; use \\n",
                        ));
                    }
                    '\0' => {
                        return Err(Diagnostic::new(
                            Span::new(start, position),
                            "NUL bytes are not supported in strings",
                        ));
                    }
                    '\\' => {
                        let Some(escape) = source[position..].chars().next() else {
                            break;
                        };
                        position += escape.len_utf8();
                        value.push(match escape {
                            'n' => '\n',
                            'r' => '\r',
                            't' => '\t',
                            '\\' => '\\',
                            '"' => '"',
                            _ => {
                                return Err(Diagnostic::new(
                                    Span::new(position - escape.len_utf8() - 1, position),
                                    format!("unsupported escape \\{escape}"),
                                ));
                            }
                        });
                    }
                    _ => value.push(character),
                }
            }
            if !closed {
                return Err(Diagnostic::new(
                    Span::new(start, position),
                    "unterminated string literal",
                ));
            }
            Kind::String(value)
        } else {
            let pair = source.get(position..position + 2).unwrap_or("");
            if matches!(
                pair,
                "==" | "!="
                    | "<="
                    | ">="
                    | "&&"
                    | "||"
                    | "++"
                    | "--"
                    | "+="
                    | "-="
                    | "*="
                    | "/="
                    | "%="
            ) {
                position += 2;
                Kind::Symbol(pair.to_owned())
            } else if b"{}();,.+-*/%=!<>&".contains(&byte) {
                position += 1;
                Kind::Symbol((byte as char).to_string())
            } else {
                let character = source[position..].chars().next().expect("valid UTF-8");
                return Err(Diagnostic::new(
                    Span::new(start, start + character.len_utf8()),
                    format!("unexpected character '{character}'"),
                ));
            }
        };
        tokens.push(Token {
            kind,
            span: Span::new(start, position),
        });
    }
    tokens.push(Token {
        kind: Kind::Eof,
        span: Span::new(position, position),
    });
    Ok(tokens)
}
