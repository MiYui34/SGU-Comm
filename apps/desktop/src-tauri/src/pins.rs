//! Server certificate pins: trust on first use.

use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Mutex;

use sha2::{Digest, Sha256};

/// Accepted server certificates by "host:port". The page can only confirm
/// a fingerprint the bridge itself offered (saw on a real connection), not
/// pin arbitrary ones.
pub struct PinStore {
    pins: Mutex<HashMap<String, String>>,
    offers: Mutex<HashMap<String, String>>,
    path: Option<PathBuf>,
}

#[derive(Debug, PartialEq, Eq)]
pub enum Check {
    Trusted,
    Unknown,
    Changed,
}

pub fn fingerprint(der: &[u8]) -> String {
    format!("sha256:{}", hex::encode(Sha256::digest(der)))
}

impl PinStore {
    pub fn in_memory() -> Self {
        PinStore { pins: Mutex::new(HashMap::new()), offers: Mutex::new(HashMap::new()), path: None }
    }

    /// Loads the pins from a JSON file. A broken file is kept aside as
    /// ".broken" (and the user sees "new server" prompts), not silently lost.
    pub fn load(path: PathBuf) -> Self {
        let pins = match std::fs::read(&path) {
            Err(_) => HashMap::new(),
            Ok(bytes) => serde_json::from_slice(&bytes).unwrap_or_else(|_| {
                let _ = std::fs::rename(&path, path.with_extension("json.broken"));
                HashMap::new()
            }),
        };
        PinStore { pins: Mutex::new(pins), offers: Mutex::new(HashMap::new()), path: Some(path) }
    }

    /// The bridge saw this certificate and asks the user about it.
    pub fn offer(&self, server: &str, fp: &str) {
        self.offers.lock().unwrap().insert(server.to_string(), fp.to_string());
    }

    /// The user accepted the offered certificate. Anything else is refused.
    pub fn confirm(&self, server: &str, fp: &str) -> Result<(), String> {
        let mut offers = self.offers.lock().unwrap();
        if offers.get(server).map(String::as_str) != Some(fp) {
            return Err("this certificate was not offered for this server".into());
        }
        offers.remove(server);
        drop(offers);
        self.trust(server, fp).map_err(|e| e.to_string())
    }

    pub fn check(&self, server: &str, fp: &str) -> Check {
        match self.pins.lock().unwrap().get(server) {
            None => Check::Unknown,
            Some(pinned) if pinned == fp => Check::Trusted,
            Some(_) => Check::Changed,
        }
    }

    pub fn trust(&self, server: &str, fp: &str) -> std::io::Result<()> {
        let mut pins = self.pins.lock().unwrap();
        pins.insert(server.to_string(), fp.to_string());
        if let Some(path) = &self.path {
            if let Some(dir) = path.parent() {
                std::fs::create_dir_all(dir)?;
            }
            // Write and rename: a crash never leaves a half-written file.
            let tmp = path.with_extension("json.tmp");
            std::fs::write(&tmp, serde_json::to_vec_pretty(&*pins)?)?;
            std::fs::rename(&tmp, path)?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn only_the_offered_fingerprint_can_be_trusted() {
        let pins = PinStore::in_memory();
        // A page script cannot pin anything the bridge did not see.
        assert!(pins.confirm("a.example:1", "sha256:aa").is_err());
        pins.offer("a.example:1", "sha256:aa");
        assert!(pins.confirm("a.example:1", "sha256:bb").is_err());
        assert!(pins.confirm("a.example:1", "sha256:aa").is_ok());
        assert_eq!(pins.check("a.example:1", "sha256:aa"), Check::Trusted);
        // An offer is used once.
        assert!(pins.confirm("a.example:1", "sha256:aa").is_err());
    }

    #[test]
    fn a_broken_file_is_kept_aside() {
        let dir = std::env::temp_dir().join(format!("kallisti-pins-broken-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).unwrap();
        let path = dir.join("pins.json");
        std::fs::write(&path, b"{ broken").unwrap();
        let _store = PinStore::load(path.clone());
        assert!(dir.join("pins.json.broken").exists(), "the broken file must be kept");
        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn unknown_servers_have_no_pin() {
        let pins = PinStore::in_memory();
        assert_eq!(pins.check("a.example:64738", "sha256:aa"), Check::Unknown);
    }

    #[test]
    fn a_trusted_fingerprint_matches_and_others_do_not() {
        let pins = PinStore::in_memory();
        pins.trust("a.example:64738", "sha256:aa").unwrap();
        assert_eq!(pins.check("a.example:64738", "sha256:aa"), Check::Trusted);
        assert_eq!(pins.check("a.example:64738", "sha256:bb"), Check::Changed);
        assert_eq!(pins.check("b.example:64738", "sha256:aa"), Check::Unknown);
    }

    #[test]
    fn pins_survive_a_restart() {
        let dir = std::env::temp_dir().join(format!("kallisti-pins-{}", std::process::id()));
        let path = dir.join("pins.json");
        let _ = std::fs::remove_dir_all(&dir);
        PinStore::load(path.clone()).trust("a.example:1", "sha256:aa").unwrap();
        assert_eq!(PinStore::load(path).check("a.example:1", "sha256:aa"), Check::Trusted);
        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn fingerprints_are_sha256_of_the_certificate() {
        assert_eq!(
            fingerprint(b"abc"),
            "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
        );
    }
}
