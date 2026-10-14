//! Server certificate pins: trust on first use. Placeholder.

use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Mutex;

use sha2::{Digest, Sha256};

/// Accepted server certificates by "host:port". Only the Rust side writes
/// them (a page cannot change its own trust).
pub struct PinStore {
    pins: Mutex<HashMap<String, String>>,
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
        PinStore { pins: Mutex::new(HashMap::new()), path: None }
    }

    /// Loads the pins from a JSON file (missing or broken: none).
    pub fn load(path: PathBuf) -> Self {
        let pins = std::fs::read(&path)
            .ok()
            .and_then(|b| serde_json::from_slice(&b).ok())
            .unwrap_or_default();
        PinStore { pins: Mutex::new(pins), path: Some(path) }
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
            std::fs::write(path, serde_json::to_vec_pretty(&*pins)?)?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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
