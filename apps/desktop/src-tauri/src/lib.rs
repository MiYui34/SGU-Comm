pub mod bridge;

use serde::Serialize;

/// The local bridge's address, for the web client.
#[derive(Clone, Serialize)]
struct BridgeInfo {
    port: u16,
    secret: String,
}

#[tauri::command]
fn bridge_info(state: tauri::State<'_, BridgeInfo>) -> BridgeInfo {
    state.inner().clone()
}

// The app window's origins (macOS/Linux, Windows); debug builds also the
// Vite dev server.
fn origins() -> Vec<String> {
    let mut origins = vec!["tauri://localhost".to_string(), "http://tauri.localhost".to_string()];
    if cfg!(debug_assertions) {
        origins.push("http://localhost:5173".to_string());
    }
    origins
}

pub fn run() {
    tauri::Builder::default()
        .setup(|app| {
            let b = tauri::async_runtime::block_on(bridge::start(origins()))?;
            use tauri::Manager;
            app.manage(BridgeInfo { port: b.port, secret: b.secret });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![bridge_info])
        .run(tauri::generate_context!())
        .expect("error while running the Kallisti app");
}
