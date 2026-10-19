pub mod bridge;
pub mod crypt;
pub mod frames;
pub mod media;
pub mod ocb2;
pub mod pins;
pub mod udp;

use std::sync::Arc;

use serde::Serialize;
use tauri::Manager;
use tauri_plugin_notification::NotificationExt;

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

/// The user accepted the server certificate the bridge offered. Only that
/// fingerprint can be confirmed, so page scripts cannot pin their own.
#[tauri::command]
fn trust_server(host: String, port: u16, fingerprint: String, pins: tauri::State<'_, Arc<pins::PinStore>>) -> Result<(), String> {
    pins.confirm(&format!("{host}:{port}"), &fingerprint)
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
        // Push-to-talk while the app is in the background.
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        // System notifications (the web view's own: none on macOS, refused
        // on Linux).
        .plugin(tauri_plugin_notification::init())
        .setup(|app| {
            let pins = Arc::new(pins::PinStore::load(app.path().app_data_dir()?.join("pins.json")));
            let b = tauri::async_runtime::block_on(bridge::start(origins(), pins.clone()))?;
            app.manage(BridgeInfo { port: b.port, secret: b.secret });
            app.manage(pins);
            // A click on a notification brings the window forward; the page
            // opens the chat.
            let handle = app.handle().clone();
            app.notification().on_action(move |action| {
                if let Some(window) = handle.get_webview_window("main").filter(|_| action.action_id() == "tap") {
                    let _ = window.unminimize();
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            })?;
            #[cfg(target_os = "linux")]
            if let Some(window) = app.get_webview_window("main") {
                // Without it only voice is off: start anyway.
                if let Err(err) = media::allow_own_media(&window, origins()) {
                    eprintln!("microphone access not set up: {err}");
                }
            }
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![bridge_info, trust_server])
        .run(tauri::generate_context!())
        .expect("error while running the Kallisti app");
}
