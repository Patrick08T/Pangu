use std::collections::HashMap;
use std::io::Write;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{LazyLock, Mutex};
use std::time::Instant;

static NEXT: AtomicU64 = AtomicU64::new(1);
static PARENTS: LazyLock<Mutex<HashMap<String, String>>> = LazyLock::new(|| Mutex::new(HashMap::new()));

pub(crate) fn enabled() -> bool {
    std::env::var("PANGU_CODEX_TRACE").as_deref() == Ok("1")
}

pub(crate) struct Span {
    id: String,
    parent: String,
    kind: &'static str,
    name: String,
    started: Instant,
    finished: bool,
}

impl Span {
    pub(crate) fn start(kind: &'static str, name: &str, input: &str, call: &str) -> Self {
        let span = Self {
            id: NEXT.fetch_add(1, Ordering::Relaxed).to_string(),
            parent: PARENTS.lock().unwrap().remove(call).unwrap_or_default(),
            kind, name: name.to_owned(), started: Instant::now(), finished: false,
        };
        span.emit("start", "running", input, "", "");
        span
    }

    pub(crate) fn link(&self, call: &str) {
        if enabled() {
            PARENTS.lock().unwrap().insert(call.to_owned(), self.id.clone());
        }
    }

    pub(crate) fn finish(&mut self, status: &str, output: &str, error: &str) {
        if !self.finished {
            self.emit("end", status, "", output, error);
            self.finished = true;
        }
    }

    fn emit(&self, phase: &str, status: &str, input: &str, output: &str, error: &str) {
        if !enabled() { return; }
        let (input, a) = clip(input);
        let (output, b) = clip(output);
        let (error, c) = clip(error);
        let line = format!(
            "{{\"type\":\"pangu.trace\",\"span_id\":{},\"parent_span_id\":{},\"kind\":{},\"phase\":{},\"name\":{},\"duration_ms\":{},\"status\":{},\"input\":{},\"output\":{},\"error\":{},\"truncated\":{}}}",
            quote(&self.id), quote(&self.parent), quote(self.kind), quote(phase), quote(&self.name),
            self.started.elapsed().as_millis(), quote(status), quote(input), quote(output), quote(error), a || b || c
        );
        // 追踪管道失败时停止执行，避免继续产生无法记录的行为。
        if writeln!(std::io::stdout().lock(), "{line}").is_err() { std::process::exit(88); }
    }
}

impl Drop for Span {
    fn drop(&mut self) {
        self.finish("cancelled", "", "操作中断，未收到完成结果");
    }
}

fn clip(value: &str) -> (&str, bool) {
    let mut end = value.len().min(65536);
    while !value.is_char_boundary(end) { end -= 1; }
    (&value[..end], end < value.len())
}

fn quote(value: &str) -> String {
    let mut out = String::from("\"");
    for c in value.chars() {
        match c {
            '"' => out.push_str("\\\""), '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"), '\r' => out.push_str("\\r"), '\t' => out.push_str("\\t"),
            c if c < ' ' => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

#[cfg(test)]
mod trace_tests {
    #[test]
    fn escapes_control_characters_and_utf8() {
        assert_eq!(super::quote("a\n\"\\中\u{0001}"), "\"a\\n\\\"\\\\中\\u0001\"");
    }
    #[test]
    fn events_are_bounded_without_splitting_utf8() {
        let text = "中".repeat(40000);
        let (short, clipped) = super::clip(&text);
        assert!(clipped);
        assert!(short.len() <= 65536);
        assert!(text.starts_with(short));
    }
}
