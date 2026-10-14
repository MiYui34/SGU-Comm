//! Mumble's CryptState (CryptStateOCB2.cpp): OCB2 packets with a 4-byte
//! header (the IV's low byte and the first 3 tag bytes) and one 16-byte IV
//! per direction, counted up little-endian. The receiver accepts packets
//! that are lost or up to 30 late, and refuses replays.

use crate::ocb2;

pub struct CryptState {
    key: [u8; 16],
    encrypt_iv: [u8; 16],
    decrypt_iv: [u8; 16],
    // Per low IV byte: the second IV byte of the last packet seen with it.
    history: [u8; 256],
}

fn increment(iv: &mut [u8; 16], from: usize) {
    for b in iv[from..].iter_mut() {
        *b = b.wrapping_add(1);
        if *b != 0 {
            break;
        }
    }
}

fn decrement(iv: &mut [u8; 16], from: usize) {
    for b in iv[from..].iter_mut() {
        let was = *b;
        *b = b.wrapping_sub(1);
        if was != 0 {
            break;
        }
    }
}

impl CryptState {
    /// encrypt_iv and decrypt_iv come from Murmur's CryptSetup: for the
    /// client, client_nonce encrypts and server_nonce decrypts.
    pub fn new(key: [u8; 16], encrypt_iv: [u8; 16], decrypt_iv: [u8; 16]) -> Self {
        CryptState { key, encrypt_iv, decrypt_iv, history: [0; 256] }
    }

    /// Murmur sends a new server nonce when it resynchronises.
    pub fn set_decrypt_iv(&mut self, iv: [u8; 16]) {
        self.decrypt_iv = iv;
    }

    pub fn encrypt(&mut self, plain: &[u8]) -> Vec<u8> {
        increment(&mut self.encrypt_iv, 0);
        let (crypted, tag) = ocb2::encrypt(&self.key, &self.encrypt_iv, plain);
        let mut out = Vec::with_capacity(plain.len() + 4);
        out.extend_from_slice(&[self.encrypt_iv[0], tag[0], tag[1], tag[2]]);
        out.extend_from_slice(&crypted);
        out
    }

    pub fn decrypt(&mut self, packet: &[u8]) -> Option<Vec<u8>> {
        if packet.len() < 4 {
            return None;
        }
        let saved = self.decrypt_iv;
        let ivbyte = packet[0];
        let current = self.decrypt_iv[0];
        let mut restore = false;
        if current.wrapping_add(1) == ivbyte {
            // In order.
            if ivbyte < current {
                increment(&mut self.decrypt_iv, 1);
            }
            self.decrypt_iv[0] = ivbyte;
        } else {
            let mut diff = ivbyte as i32 - current as i32;
            if diff > 128 {
                diff -= 256;
            } else if diff < -128 {
                diff += 256;
            }
            if ivbyte < current && diff > -30 && diff < 0 {
                // Late, no wrap-around.
                self.decrypt_iv[0] = ivbyte;
                restore = true;
            } else if ivbyte > current && diff > -30 && diff < 0 {
                // Late, from before a wrap-around.
                self.decrypt_iv[0] = ivbyte;
                decrement(&mut self.decrypt_iv, 1);
                restore = true;
            } else if ivbyte > current && diff > 0 {
                // Some packets lost.
                self.decrypt_iv[0] = ivbyte;
            } else if ivbyte < current && diff > 0 {
                // Some packets lost, with a wrap-around.
                self.decrypt_iv[0] = ivbyte;
                increment(&mut self.decrypt_iv, 1);
            } else {
                return None;
            }
            if self.history[self.decrypt_iv[0] as usize] == self.decrypt_iv[1] {
                self.decrypt_iv = saved;
                return None; // replay
            }
        }
        let ok = ocb2::decrypt(&self.key, &self.decrypt_iv, &packet[4..]).filter(|(_, tag)| tag[..3] == packet[1..4]);
        let Some((plain, _)) = ok else {
            self.decrypt_iv = saved;
            return None;
        };
        self.history[self.decrypt_iv[0] as usize] = self.decrypt_iv[1];
        if restore {
            self.decrypt_iv = saved;
        }
        Some(plain)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pair() -> (CryptState, CryptState) {
        let key = [3u8; 16];
        let (client_nonce, server_nonce) = ([1u8; 16], [2u8; 16]);
        // Client encrypts with client_nonce, the server decrypts with it.
        let client = CryptState::new(key, client_nonce, server_nonce);
        let server = CryptState::new(key, server_nonce, client_nonce);
        (client, server)
    }

    #[test]
    fn packets_round_trip_in_order() {
        let (mut client, mut server) = pair();
        for i in 0..600u32 {
            let msg = format!("voice packet {i}");
            let packet = client.encrypt(msg.as_bytes());
            assert_eq!(packet.len(), msg.len() + 4);
            assert_eq!(server.decrypt(&packet).as_deref(), Some(msg.as_bytes()), "packet {i}");
        }
    }

    #[test]
    fn lost_and_late_packets_still_decrypt() {
        let (mut client, mut server) = pair();
        // Not all zeros: OCB2's countermeasure changes such blocks on purpose.
        let data = |i: u8| vec![0x41 + i; 20];
        let packets: Vec<Vec<u8>> = (0..10u8).map(|i| client.encrypt(&data(i))).collect();
        assert_eq!(server.decrypt(&packets[0]).unwrap(), data(0));
        // 1 and 2 are lost for now, 3 arrives.
        assert_eq!(server.decrypt(&packets[3]).unwrap(), data(3));
        // 2 arrives late.
        assert_eq!(server.decrypt(&packets[2]).unwrap(), data(2));
        assert_eq!(server.decrypt(&packets[4]).unwrap(), data(4));
    }

    #[test]
    fn replayed_and_tampered_packets_are_refused() {
        let (mut client, mut server) = pair();
        let first = client.encrypt(b"hello");
        let second = client.encrypt(b"world");
        assert!(server.decrypt(&first).is_some());
        assert!(server.decrypt(&second).is_some());
        assert!(server.decrypt(&first).is_none(), "replay");
        let mut bad = client.encrypt(b"tampered");
        let last = bad.len() - 1;
        bad[last] ^= 1;
        assert!(server.decrypt(&bad).is_none(), "tampered");
        assert!(server.decrypt(&[1, 2]).is_none(), "too short");
    }

    #[test]
    fn a_wrong_key_does_not_decrypt() {
        let mut client = CryptState::new([3u8; 16], [1u8; 16], [2u8; 16]);
        let mut other = CryptState::new([4u8; 16], [2u8; 16], [1u8; 16]);
        assert!(other.decrypt(&client.encrypt(b"secret")).is_none());
    }
}
