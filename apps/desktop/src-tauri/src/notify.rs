//! Linux notifications over D-Bus. tauri-plugin-notification sends the
//! icon by name (an AppImage installs none: a broken icon) and the click
//! action without a label (servers that show actions as buttons, like
//! LXQt's, show an empty one).

use std::collections::HashMap;
use std::path::Path;

pub const ICON: &[u8] = include_bytes!("../icons/128x128.png");

/// Writes the app icon into `dir` (unless it is there) and returns it as a
/// file URI, which the notification servers load.
pub fn write_icon(dir: &Path) -> std::io::Result<String> {
    let path = dir.join("notification.png");
    if std::fs::read(&path).ok().as_deref() != Some(ICON) {
        std::fs::create_dir_all(dir)?;
        std::fs::write(&path, ICON)?;
    }
    url::Url::from_file_path(&path).map(String::from).map_err(|_| std::io::Error::other("not an absolute path"))
}

pub enum Signal {
    Action(u32, String),
    Closed(u32),
}

/// The shown notifications: the page's id by server id.
#[derive(Default)]
pub struct Shown(HashMap<u32, i32>);

impl Shown {
    pub fn add(&mut self, server_id: u32, id: i32) {
        self.0.insert(server_id, id);
    }

    /// The page's id of a clicked notification (the spec's "default"
    /// action). Kept until the server closes it.
    pub fn on(&mut self, signal: Signal) -> Option<i32> {
        match signal {
            Signal::Action(server_id, action) if action == "default" => self.0.get(&server_id).copied(),
            Signal::Action(..) => None,
            Signal::Closed(server_id) => {
                self.0.remove(&server_id);
                None
            }
        }
    }
}

#[cfg(target_os = "linux")]
pub use linux::Notifier;

#[cfg(target_os = "linux")]
mod linux {
    use std::collections::HashMap;
    use std::sync::{Arc, Mutex, OnceLock, PoisonError};

    use zbus::blocking::{Connection, MessageIterator};
    use zbus::zvariant::Value;

    use super::{Shown, Signal};

    const BUS: (&str, &str, &str) =
        ("org.freedesktop.Notifications", "/org/freedesktop/Notifications", "org.freedesktop.Notifications");

    pub struct Notifier {
        icon: String,
        on_click: Arc<dyn Fn(i32) + Send + Sync>,
        shown: Arc<Mutex<Shown>>,
        connection: OnceLock<Option<Connection>>,
    }

    impl Notifier {
        pub fn new(icon: String, on_click: impl Fn(i32) + Send + Sync + 'static) -> Self {
            Notifier { icon, on_click: Arc::new(on_click), shown: Arc::default(), connection: OnceLock::new() }
        }

        /// Sends it on the connection the signals arrive on (dunst sends
        /// them only to the sender), which stays open (GNOME closes a
        /// notification once its sender goes away).
        pub fn show(&self, id: i32, title: &str, body: &str) -> Result<(), String> {
            let connection = self
                .connection
                .get_or_init(|| self.listen().map_err(|err| eprintln!("no notification service: {err}")).ok())
                .as_ref()
                .ok_or("no notification service")?;
            // Held while sending, so a click that arrives before the server
            // id is known waits for it.
            let mut shown = self.shown.lock().unwrap_or_else(PoisonError::into_inner);
            let (destination, path, interface) = BUS;
            let actions = ["default", "Open"];
            let hints = HashMap::<&str, Value>::new();
            let server_id: u32 = connection
                .call_method(
                    Some(destination),
                    path,
                    Some(interface),
                    "Notify",
                    &("Kallisti", 0u32, &self.icon, title, body, &actions[..], hints, -1i32),
                )
                .and_then(|reply| reply.body().deserialize())
                .map_err(|err| err.to_string())?;
            shown.add(server_id, id);
            Ok(())
        }

        /// Connects to the session bus and handles the server's signals on a
        /// thread of its own.
        fn listen(&self) -> zbus::Result<Connection> {
            let (_, path, interface) = BUS;
            let connection = Connection::session()?;
            let rule =
                zbus::MatchRule::builder().msg_type(zbus::message::Type::Signal).path(path)?.interface(interface)?.build();
            let signals = MessageIterator::for_match_rule(rule, &connection, None)?;
            let (shown, on_click) = (self.shown.clone(), self.on_click.clone());
            std::thread::Builder::new().name("notifications".into()).spawn(move || {
                for message in signals.flatten() {
                    let clicked = signal(&message).and_then(|s| shown.lock().unwrap_or_else(PoisonError::into_inner).on(s));
                    if let Some(id) = clicked {
                        on_click(id);
                    }
                }
            })?;
            Ok(connection)
        }
    }

    fn signal(message: &zbus::Message) -> Option<Signal> {
        let body = message.body();
        match message.header().member()?.as_str() {
            "ActionInvoked" => body.deserialize::<(u32, String)>().ok().map(|(id, action)| Signal::Action(id, action)),
            "NotificationClosed" => body.deserialize::<(u32, u32)>().ok().map(|(id, _)| Signal::Closed(id)),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_icon_is_written_once_as_a_file_uri() {
        let dir = std::env::temp_dir().join(format!("kallisti-icon-{}", std::process::id()));
        let uri = write_icon(&dir).unwrap();
        assert_eq!(uri, url::Url::from_file_path(dir.join("notification.png")).unwrap().to_string());
        assert_eq!(std::fs::read(dir.join("notification.png")).unwrap(), ICON);
        assert_eq!(write_icon(&dir).unwrap(), uri);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn a_click_reports_the_notification_until_it_closes() {
        let mut shown = Shown::default();
        shown.add(7, 42);
        assert_eq!(shown.on(Signal::Action(7, "default".into())), Some(42));
        // The server may keep showing it: a second click counts too.
        assert_eq!(shown.on(Signal::Action(7, "default".into())), Some(42));
        assert_eq!(shown.on(Signal::Action(7, "other".into())), None);
        assert_eq!(shown.on(Signal::Action(8, "default".into())), None);
        assert_eq!(shown.on(Signal::Closed(7)), None);
        assert_eq!(shown.on(Signal::Action(7, "default".into())), None);
    }
}
