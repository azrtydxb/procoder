// Deliberately unsafe/broken fixture — exercises every rust.js finding id.

use std::process::Command;

fn parse(input: &str) -> Result<i32, std::num::ParseIntError> {
    input.parse::<i32>()
}

fn will_panic(input: &str) -> i32 {
    parse(input).unwrap()
}

fn will_also_panic(input: &str) -> i32 {
    parse(input).expect("should parse")
}

unsafe fn read_raw(p: *const i32) -> i32 {
    unsafe { *p }
}

fn lookup_user(id: &str) {
    sqlx::query(&format!("SELECT * FROM t WHERE id = {}", id));
}

fn run_it(user_input: &str) {
    Command::new("sh").arg("-c").arg(user_input).spawn().unwrap();
}

fn insecure_client() {
    let _builder = reqwest::Client::builder()
        .danger_accept_invalid_certs(true);
}

fn make_token() -> u64 {
    let token = rand::random::<u64>();
    token
}

fn debug_print(x: i32) {
    println!("here {}", x);
    dbg!(x);
}
