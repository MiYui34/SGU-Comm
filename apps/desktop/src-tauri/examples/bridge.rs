//! Runs only the local bridge, for testing the web client's desktop mode in
//! a normal browser: cargo run --example bridge
#[tokio::main]
async fn main() {
    let b = kallisti_desktop::bridge::start(vec!["http://localhost:5173".into()]).await.unwrap();
    println!("{{\"port\":{},\"secret\":\"{}\"}}", b.port, b.secret);
    std::future::pending::<()>().await;
}
