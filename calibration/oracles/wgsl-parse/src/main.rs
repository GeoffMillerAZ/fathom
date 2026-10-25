//! wgsl-parse oracle: the neutral JSON `indexbench lang-oracle -tool-oracle` reads (definitions and call pairs),
//! from the independent wgsl-parse 0.6.0 parser (README.md). Bench-only; never a product dependency.
//!
//!     wgsl-parse-oracle <subject-root> [out.json]   (stdout without the second argument)
use std::collections::BTreeSet;
use std::fs;
use std::path::Path;

use serde_json::{json, Value};
use walkdir::WalkDir;
use wgsl_parse::syntax::*;

// wgsl-parse is recursive and a pathologically nested file (wgpu's deep-template.wgsl) takes it exponential time,
// so each file parses on its own big-stack thread under a time limit; a file past it counts as rejected.
const PARSE_LIMIT: std::time::Duration = std::time::Duration::from_secs(20);

fn parse_limited(src: String) -> Option<TranslationUnit> {
    let (tx, rx) = std::sync::mpsc::channel();
    std::thread::Builder::new()
        .stack_size(1 << 30)
        .spawn(move || {
            let _ = tx.send(wgsl_parse::parse_str(&src).ok());
        })
        .ok()?;
    rx.recv_timeout(PARSE_LIMIT).ok().flatten()
}

fn main() {
    std::thread::Builder::new()
        .stack_size(1 << 30)
        .spawn(run)
        .unwrap()
        .join()
        .unwrap();
}

fn run() {
    let root = std::env::args().nth(1).unwrap_or_else(|| {
        eprintln!("usage: wgsl-parse-oracle <subject-root> [out.json]");
        std::process::exit(2)
    });
    let root = Path::new(&root);
    let mut files: Vec<_> = WalkDir::new(root)
        .into_iter()
        .filter_entry(|e| e.file_name() != ".git")
        .filter_map(Result::ok)
        .filter(|e| e.file_type().is_file() && e.path().extension().is_some_and(|x| x == "wgsl"))
        .map(|e| e.into_path())
        .collect();
    files.sort();
    let (mut defs, mut calls) = (Vec::new(), Vec::new());
    let (mut rejected, mut parsed) = (Vec::new(), 0usize);
    for f in &files {
        let rel = f.strip_prefix(root).unwrap().to_string_lossy().replace('\\', "/");
        let Ok(bytes) = fs::read(f) else { rejected.push(rel); continue };
        let src = String::from_utf8_lossy(&bytes).into_owned();
        let Some(unit) = parse_limited(src.clone()) else {
            rejected.push(rel);
            continue;
        };
        parsed += 1;
        let masked = mask_comments(&src);
        for node in &unit.global_declarations {
            let (kind, name, attrs, callees) = match &**node {
                GlobalDeclaration::Function(f) => {
                    let mut c = BTreeSet::new();
                    attrs_calls(&f.attributes, &mut c);
                    stmts_calls(&f.body.statements, &mut c);
                    ("fn", f.ident.name().clone(), &f.attributes, c)
                }
                GlobalDeclaration::Struct(s) => ("struct", s.ident.name().clone(), &s.attributes, BTreeSet::new()),
                GlobalDeclaration::TypeAlias(a) => ("alias", a.ident.name().clone(), &a.attributes, BTreeSet::new()),
                GlobalDeclaration::Declaration(d) => {
                    let kw = match d.kind {
                        DeclarationKind::Const => "const",
                        DeclarationKind::Override => "override",
                        DeclarationKind::Let => "let",
                        DeclarationKind::Var(_) => "var",
                    };
                    let mut c = BTreeSet::new();
                    attrs_calls(&d.attributes, &mut c);
                    if let Some(t) = &d.ty {
                        type_calls(t, &mut c);
                    }
                    if let Some(i) = &d.initializer {
                        expr_calls(i, &mut c);
                    }
                    (kw, d.ident.name().clone(), &d.attributes, c)
                }
                _ => continue,
            };
            let _ = attrs;
            let span = node.span();
            let line = name_line(&masked, span.start, kind, &name);
            let end = line_of(&src, last_byte(&src, span.end));
            defs.push(json!({"path": rel, "name": name, "line": line, "end": end, "kind": kind}));
            for to in callees {
                calls.push(json!({"path": rel, "from": name, "to": to}));
            }
        }
    }
    let mut disclose = vec![format!(
        "wgsl-parse rejects or times out on {} of {} .wgsl files (no definitions read from them)",
        rejected.len(),
        files.len()
    )];
    for r in &rejected {
        disclose.push(format!("wgsl-parse rejects or times out on {r}"));
    }
    if std::path::Path::new(&root).join("naga/tests/naga").is_dir()
        && !std::path::Path::new(&root).join("naga/tests/naga/deep-template.wgsl").exists()
    {
        disclose.push("product limit: naga/tests/naga/deep-template.wgsl (wgpu's recursion_depth_template negative test, naga/tests/naga/wgsl_errors.rs:998, which expects naga's own front end to reject it with a recursion-limit error; a template nested thousands deep) is left out of the subject by the sparse list; tree-sitter's GLR parse of it takes 6.1 to 8.1 s on the 4-core measuring VM (it exceeded the 10 s budget in one earlier run, a fault) and alone drops throughput to 0.31 MB/s against the 1 MB/s floor; a template-nesting ingest guard is a follow-up".to_string());
    }
    eprintln!("files {} parsed {} rejected {} definitions {} calls {}", files.len(), parsed, rejected.len(), defs.len(), calls.len());
    let out: Value = json!({
        "tool": "wgsl-parse", "version": "0.6.0", "tier": "B",
        "definitions": defs, "calls": calls, "disclose": disclose,
    });
    let text = serde_json::to_string(&out).unwrap();
    match std::env::args().nth(2) {
        Some(p) => fs::write(p, text).expect("write the oracle file"),
        None => println!("{text}"),
    }
}

// Comments (// to the line end, nested /* */) become spaces, newlines kept, so a name search and the line
// numbers both read the original text.
fn mask_comments(src: &str) -> Vec<u8> {
    let b = src.as_bytes();
    let mut out = b.to_vec();
    let (mut i, mut depth) = (0, 0usize);
    while i < b.len() {
        if depth == 0 && b[i] == b'/' && b.get(i + 1) == Some(&b'/') {
            while i < b.len() && b[i] != b'\n' {
                out[i] = b' ';
                i += 1;
            }
        } else if b[i] == b'/' && b.get(i + 1) == Some(&b'*') {
            depth += 1;
            out[i] = b' ';
            out[i + 1] = b' ';
            i += 2;
        } else if depth > 0 && b[i] == b'*' && b.get(i + 1) == Some(&b'/') {
            depth -= 1;
            out[i] = b' ';
            out[i + 1] = b' ';
            i += 2;
        } else {
            if depth > 0 && b[i] != b'\n' {
                out[i] = b' ';
            }
            i += 1;
        }
    }
    out
}

fn is_word(c: u8) -> bool {
    c.is_ascii_alphanumeric() || c == b'_' || c >= 0x80
}

// The line of the declaration's name: the first word equal to the name after the declaration's keyword,
// at or after the node's first byte (an attribute's), never an `@const` attribute.
fn name_line(masked: &[u8], from: usize, kw: &str, name: &str) -> usize {
    let mut i = from;
    let mut seen_kw = false;
    while i < masked.len() {
        if !is_word(masked[i]) {
            i += 1;
            continue;
        }
        let s = i;
        while i < masked.len() && is_word(masked[i]) {
            i += 1;
        }
        let w = &masked[s..i];
        if !seen_kw {
            let at = s > 0 && masked[..s].iter().rev().find(|c| !c.is_ascii_whitespace()) == Some(&b'@');
            seen_kw = w == kw.as_bytes() && !at;
        } else if w == name.as_bytes() {
            return masked[..s].iter().filter(|&&c| c == b'\n').count() + 1;
        }
    }
    masked[..from.min(masked.len())].iter().filter(|&&c| c == b'\n').count() + 1
}

fn last_byte(src: &str, end: usize) -> usize {
    let b = src.as_bytes();
    let mut e = end.min(b.len());
    while e > 0 && b[e - 1].is_ascii_whitespace() {
        e -= 1;
    }
    e.saturating_sub(1)
}

fn line_of(src: &str, byte: usize) -> usize {
    src.as_bytes()[..byte.min(src.len())].iter().filter(|&&c| c == b'\n').count() + 1
}

type Callees = BTreeSet<String>;

fn call(c: &FunctionCall, out: &mut Callees) {
    out.insert(c.ty.ident.name().clone());
    type_calls(&c.ty, out);
    for a in &c.arguments {
        expr_calls(a, out);
    }
}

fn type_calls(t: &TypeExpression, out: &mut Callees) {
    if let Some(args) = &t.template_args {
        for a in args {
            expr_calls(&a.expression, out);
        }
    }
}

fn expr_calls(e: &ExpressionNode, out: &mut Callees) {
    match &**e {
        Expression::Literal(_) => {}
        Expression::Parenthesized(p) => expr_calls(&p.expression, out),
        Expression::NamedComponent(n) => expr_calls(&n.base, out),
        Expression::Indexing(i) => {
            expr_calls(&i.base, out);
            expr_calls(&i.index, out)
        }
        Expression::Unary(u) => expr_calls(&u.operand, out),
        Expression::Binary(b) => {
            expr_calls(&b.left, out);
            expr_calls(&b.right, out)
        }
        Expression::FunctionCall(c) => call(c, out),
        Expression::TypeOrIdentifier(t) => type_calls(t, out),
    }
}

fn opt_expr(e: &Option<ExpressionNode>, out: &mut Callees) {
    if let Some(e) = e {
        expr_calls(e, out)
    }
}

fn attrs_calls(attrs: &Attributes, out: &mut Callees) {
    for a in attrs {
        match &**a {
            Attribute::Align(e)
            | Attribute::Binding(e)
            | Attribute::BlendSrc(e)
            | Attribute::Group(e)
            | Attribute::Id(e)
            | Attribute::Location(e)
            | Attribute::Size(e)
            | Attribute::Payload(e)
            | Attribute::Mesh(e)
            | Attribute::IncomingPayload(e) => expr_calls(e, out),
            Attribute::WorkgroupSize(w) => {
                expr_calls(&w.x, out);
                opt_expr(&w.y, out);
                opt_expr(&w.z, out)
            }
            Attribute::Custom(c) => {
                for e in c.arguments.iter().flatten() {
                    expr_calls(e, out)
                }
            }
            _ => {}
        }
    }
}

fn stmts_calls(ss: &[StatementNode], out: &mut Callees) {
    for s in ss {
        stmt_calls(s, out)
    }
}

fn stmt_calls(s: &StatementNode, out: &mut Callees) {
    match &**s {
        Statement::Void
        | Statement::Break(_)
        | Statement::Continue(_)
        | Statement::Discard(_) => {}
        Statement::Compound(c) => stmts_calls(&c.statements, out),
        Statement::Assignment(a) => {
            expr_calls(&a.lhs, out);
            expr_calls(&a.rhs, out)
        }
        Statement::Increment(i) => expr_calls(&i.expression, out),
        Statement::Decrement(d) => expr_calls(&d.expression, out),
        Statement::If(i) => {
            expr_calls(&i.if_clause.expression, out);
            stmts_calls(&i.if_clause.body.statements, out);
            for c in &i.else_if_clauses {
                expr_calls(&c.expression, out);
                stmts_calls(&c.body.statements, out)
            }
            if let Some(c) = &i.else_clause {
                stmts_calls(&c.body.statements, out)
            }
        }
        Statement::Switch(sw) => {
            expr_calls(&sw.expression, out);
            for c in &sw.clauses {
                for sel in &c.case_selectors {
                    if let CaseSelector::Expression(e) = sel {
                        expr_calls(e, out)
                    }
                }
                stmts_calls(&c.body.statements, out)
            }
        }
        Statement::Loop(l) => {
            stmts_calls(&l.body.statements, out);
            if let Some(c) = &l.continuing {
                stmts_calls(&c.body.statements, out);
                if let Some(b) = &c.break_if {
                    expr_calls(&b.expression, out)
                }
            }
        }
        Statement::For(f) => {
            if let Some(i) = &f.initializer {
                stmt_calls(i, out)
            }
            opt_expr(&f.condition, out);
            if let Some(u) = &f.update {
                stmt_calls(u, out)
            }
            stmts_calls(&f.body.statements, out)
        }
        Statement::While(w) => {
            expr_calls(&w.condition, out);
            stmts_calls(&w.body.statements, out)
        }
        Statement::Return(r) => opt_expr(&r.expression, out),
        Statement::FunctionCall(c) => call(&c.call, out),
        Statement::ConstAssert(c) => expr_calls(&c.expression, out),
        Statement::Declaration(d) => {
            if let Some(t) = &d.ty {
                type_calls(t, out)
            }
            opt_expr(&d.initializer, out)
        }
    }
}
