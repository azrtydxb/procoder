// Clean fixture — real near-misses for every rule rust.js implements, all of
// which must stay silent.

use std::process::Command;

fn parse(input: &str) -> Result<i32, std::num::ParseIntError> {
    input.parse::<i32>()
}

fn propagates(input: &str) -> Result<i32, std::num::ParseIntError> {
    let v = parse(input)?;
    Ok(v)
}

fn also_propagates(input: &str) -> Result<i32, std::num::ParseIntError> {
    parse(input)
}

// unwrap() on a Mutex lock is near-universal Rust practice: a poisoned lock
// means a prior panic already corrupted shared state, and there is no
// sensible recovery short of aborting — so procoder does not flag it.
fn read_counter(counter: &std::sync::Mutex<i32>) -> i32 {
    *counter.lock().unwrap()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_ok() {
        assert_eq!(parse("1").unwrap(), 1);
    }
}

// This function comes back after the test module in source order — it must
// still be treated as library code, not swept into the test region.
fn after_tests(input: &str) -> Result<i32, std::num::ParseIntError> {
    parse(input)
}

// SAFETY: p is non-null and points to a live, aligned i32, checked by the
// caller before this function is invoked.
unsafe fn read_raw(p: *const i32) -> i32 {
    unsafe { *p }
}

fn lookup_user(id: &str) {
    sqlx::query("SELECT * FROM t WHERE id = $1").bind(id);
}

fn run_it(dir: &str) {
    Command::new("ls").arg(dir).spawn().ok();
}

fn secure_client() {
    let _builder = reqwest::Client::builder();
}

fn make_token() -> u64 {
    use rand::rngs::OsRng;
    use rand::RngCore;
    OsRng.next_u64()
}

fn debug_print(x: i32) {
    tracing::info!("here {}", x);
}

/// Documentation that warns against a practice must not be flagged for the
/// practice: every rule rust.js has, named in prose, still silent.
///
/// - never `parse(input).unwrap()` or `.expect("should parse")` in a library
/// - never `sqlx::query(&format!("SELECT * FROM t WHERE id = {}", id))`
/// - never `Command::new("sh").arg("-c").arg(user_input)`
/// - never `.danger_accept_invalid_certs(true)`
/// - never `let token = rand::random::<u64>();` for a secret
/// - no leftover `println!("here")` or `dbg!(value)`
/// - never `unsafe { ptr::read(p) }` without a SAFETY note
pub fn documented_clean() {}
