use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::mpsc::{Receiver, RecvTimeoutError, Sender};
use std::time::Duration;

static USED: AtomicUsize = AtomicUsize::new(0);

pub(crate) struct Guard(Sender<()>);

impl Drop for Guard {
    fn drop(&mut self) {
        let _ = self.0.send(());
    }
}

pub(crate) fn enabled() -> bool {
    std::env::var_os("PANGU_CODEX_TOOL_CALLS").is_some()
}

pub(crate) fn enter() -> Result<Option<Guard>, String> {
    let value = match std::env::var("PANGU_CODEX_TOOL_CALLS") {
        Ok(value) => value,
        Err(std::env::VarError::NotPresent) => return Ok(None),
        Err(_) => return Err("PANGU_POLICY_INVALID".into()),
    };
    let limit = value
        .parse::<usize>()
        .ok()
        .filter(|v| *v <= 10)
        .ok_or_else(|| "PANGU_POLICY_INVALID".to_owned())?;
    reserve(&USED, limit)?;
    let (sender, receiver) = std::sync::mpsc::channel();
    std::thread::Builder::new()
        .name("pangu-tool-deadline".into())
        .spawn(move || {
            if expired(receiver, Duration::from_secs(30)) {
                eprintln!("PANGU_POLICY_TIMEOUT");
                // 立即退出，让 Pangu 的 Job/进程组停止全部子进程并拒绝发布尝试。
                std::process::exit(86);
            }
        })
        .map_err(|_| "PANGU_POLICY_WATCHDOG_UNAVAILABLE".to_owned())?;
    Ok(Some(Guard(sender)))
}

fn reserve(used: &AtomicUsize, limit: usize) -> Result<(), String> {
    used.fetch_update(Ordering::SeqCst, Ordering::SeqCst, |value| {
        if value < limit { Some(value + 1) } else { None }
    })
    .map(|_| ())
    .map_err(|_| "PANGU_POLICY_TOOL_LIMIT".to_owned())
}

fn expired(receiver: Receiver<()>, timeout: Duration) -> bool {
    matches!(
        receiver.recv_timeout(timeout),
        Err(RecvTimeoutError::Timeout)
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn concurrent_calls_never_exceed_budget() {
        let used = std::sync::Arc::new(AtomicUsize::new(0));
        let threads: Vec<_> = (0..40)
            .map(|_| {
                let used = used.clone();
                std::thread::spawn(move || reserve(&used, 10).is_ok())
            })
            .collect();
        let accepted = threads
            .into_iter()
            .filter_map(|t| t.join().ok())
            .filter(|v| *v)
            .count();
        assert_eq!(accepted, 10);
        assert!(reserve(&AtomicUsize::new(0), 0).is_err());
    }

    #[test]
    fn deadline_expires_but_completed_calls_cancel_watchdog() {
        let (sender, receiver) = std::sync::mpsc::channel();
        assert!(expired(receiver, Duration::from_millis(10)));
        drop(sender);
        let (sender, receiver) = std::sync::mpsc::channel();
        drop(sender);
        assert!(!expired(receiver, Duration::from_secs(30)));
    }
}
