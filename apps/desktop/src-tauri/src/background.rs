//! Calls go on while the app is in the background. On macOS the window's
//! web view keeps running when hidden (tauri.conf.json: backgroundThrottling
//! "disabled", WKPreferences.inactiveSchedulingPolicy none, macOS 14+), and
//! here: no App Nap, which would slow the whole app down. Otherwise the page
//! stops answering the server's pings and loses the connection.

/// Turns App Nap off for the rest of the process's life (the idle system
/// may still sleep).
#[cfg(target_os = "macos")]
pub fn no_app_nap() {
    use objc2_foundation::{ns_string, NSActivityOptions, NSProcessInfo};
    let activity = NSProcessInfo::processInfo()
        .beginActivityWithOptions_reason(NSActivityOptions::UserInitiatedAllowingIdleSystemSleep, ns_string!("Voice chat"));
    std::mem::forget(activity); // ends when released
}

#[cfg(test)]
mod tests {
    #[cfg(target_os = "macos")]
    #[test]
    fn app_nap_off() {
        super::no_app_nap();
    }

    #[test]
    fn web_view_not_suspended_in_background() {
        let conf: serde_json::Value = serde_json::from_str(include_str!("../tauri.conf.json")).unwrap();
        assert_eq!(conf["app"]["windows"][0]["backgroundThrottling"], "disabled");
    }
}
