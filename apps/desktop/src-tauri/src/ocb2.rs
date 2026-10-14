//! OCB2-AES128 as Mumble uses it for UDP voice (CryptStateOCB2.cpp), with
//! Mumble's countermeasure against the XEX* attack (eprint 2019/311, sec. 9).
//! Blocks are 128-bit big-endian numbers; "double" is multiplication by x in
//! GF(2^128) with the polynomial 0x87.

use aes::cipher::{generic_array::GenericArray, BlockDecrypt, BlockEncrypt, KeyInit};
use aes::Aes128;

const BLOCK: usize = 16;

fn double(x: u128) -> u128 {
    (x << 1) ^ if x >> 127 == 1 { 0x87 } else { 0 }
}

fn triple(x: u128) -> u128 {
    x ^ double(x)
}

fn load(b: &[u8]) -> u128 {
    u128::from_be_bytes(b.try_into().expect("16 bytes"))
}

fn enc(aes: &Aes128, x: u128) -> u128 {
    let mut b = GenericArray::from(x.to_be_bytes());
    aes.encrypt_block(&mut b);
    load(&b)
}

fn dec(aes: &Aes128, x: u128) -> u128 {
    let mut b = GenericArray::from(x.to_be_bytes());
    aes.decrypt_block(&mut b);
    load(&b)
}

// The first byte in memory is the highest byte of the big-endian number.
const FIRST_BYTE_BIT: u128 = 1 << 120;

/// Encrypts and returns the ciphertext (same length) and the full tag.
pub fn encrypt(key: &[u8; 16], nonce: &[u8; 16], plain: &[u8]) -> (Vec<u8>, [u8; 16]) {
    let aes = Aes128::new(key.into());
    let mut delta = enc(&aes, load(nonce));
    let mut checksum = 0u128;
    let mut out = Vec::with_capacity(plain.len());
    let mut rest = plain;
    while rest.len() > BLOCK {
        let block = load(&rest[..BLOCK]);
        // Countermeasure: an attack needs the second to last block to be all
        // zeros except its last byte. Flip a bit instead (as Mumble does;
        // digital silence produces such blocks).
        let flip = rest.len() - BLOCK <= BLOCK && rest[..BLOCK - 1].iter().all(|b| *b == 0);
        delta = double(delta);
        let mut tmp = delta ^ block;
        if flip {
            tmp ^= FIRST_BYTE_BIT;
        }
        out.extend_from_slice(&(delta ^ enc(&aes, tmp)).to_be_bytes());
        checksum ^= block;
        if flip {
            checksum ^= FIRST_BYTE_BIT;
        }
        rest = &rest[BLOCK..];
    }
    delta = double(delta);
    let pad = enc(&aes, delta ^ (rest.len() as u128 * 8)).to_be_bytes();
    let mut last = pad;
    last[..rest.len()].copy_from_slice(rest);
    checksum ^= load(&last);
    let crypted = load(&pad) ^ load(&last);
    out.extend_from_slice(&crypted.to_be_bytes()[..rest.len()]);
    let tag = enc(&aes, triple(delta) ^ checksum).to_be_bytes();
    (out, tag)
}

/// Decrypts and returns the plaintext and the full tag (the caller compares
/// the tag). None: the packet matches the XEX* attack pattern.
pub fn decrypt(key: &[u8; 16], nonce: &[u8; 16], encrypted: &[u8]) -> Option<(Vec<u8>, [u8; 16])> {
    let aes = Aes128::new(key.into());
    let mut delta = enc(&aes, load(nonce));
    let mut checksum = 0u128;
    let mut out = Vec::with_capacity(encrypted.len());
    let mut rest = encrypted;
    while rest.len() > BLOCK {
        delta = double(delta);
        let plain = delta ^ dec(&aes, delta ^ load(&rest[..BLOCK]));
        out.extend_from_slice(&plain.to_be_bytes());
        checksum ^= plain;
        rest = &rest[BLOCK..];
    }
    delta = double(delta);
    let pad = enc(&aes, delta ^ (rest.len() as u128 * 8));
    let mut last = [0u8; 16];
    last[..rest.len()].copy_from_slice(rest);
    let tmp = (load(&last) ^ pad).to_be_bytes();
    checksum ^= load(&tmp);
    out.extend_from_slice(&tmp[..rest.len()]);
    // Countermeasure: in an attack the decrypted last block equals delta
    // except for the last byte.
    if tmp[..BLOCK - 1] == delta.to_be_bytes()[..BLOCK - 1] {
        return None;
    }
    let tag = enc(&aes, triple(delta) ^ checksum).to_be_bytes();
    Some((out, tag))
}

#[cfg(test)]
mod tests {
    use super::*;

    // Test vectors from draft-krovetz-ocb-00.txt, as in Mumble's TestCrypt.
    const KEY: [u8; 16] = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15];

    #[test]
    fn empty_message_vector() {
        let (crypted, tag) = encrypt(&KEY, &KEY, &[]);
        assert!(crypted.is_empty());
        assert_eq!(tag, [0xBF, 0x31, 0x08, 0x13, 0x07, 0x73, 0xAD, 0x5E, 0xC7, 0x0E, 0xC6, 0x9E, 0x78, 0x75, 0xA7, 0xB0]);
    }

    #[test]
    fn forty_byte_vector() {
        let plain: Vec<u8> = (0..40).collect();
        let (crypted, tag) = encrypt(&KEY, &KEY, &plain);
        assert_eq!(tag, [0x9D, 0xB0, 0xCD, 0xF8, 0x80, 0xF7, 0x3E, 0x3E, 0x10, 0xD4, 0xEB, 0x32, 0x17, 0x76, 0x66, 0x88]);
        assert_eq!(
            crypted,
            [
                0xF7, 0x5D, 0x6B, 0xC8, 0xB4, 0xDC, 0x8D, 0x66, 0xB8, 0x36, 0xA2, 0xB0, 0x8B, 0x32, 0xA6, 0x36, 0x9F, 0x1C, 0xD3,
                0xC5, 0x22, 0x8D, 0x79, 0xFD, 0x6C, 0x26, 0x7F, 0x5F, 0x6A, 0xA7, 0xB2, 0x31, 0xC7, 0xDF, 0xB9, 0xD5, 0x99, 0x51,
                0xAE, 0x9C
            ]
        );
    }

    #[test]
    fn round_trips_all_lengths() {
        let nonce = [0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00];
        for len in 0..128 {
            let plain: Vec<u8> = (0..len).map(|i| (i * 7 + 3) as u8).collect();
            let (crypted, tag) = encrypt(&KEY, &nonce, &plain);
            let (decrypted, dectag) = decrypt(&KEY, &nonce, &crypted).expect("decrypts");
            assert_eq!(decrypted, plain, "length {len}");
            assert_eq!(dectag, tag, "length {len}");
        }
    }

    #[test]
    fn refuses_the_xex_star_attack_pattern() {
        // Mumble's countermeasure (eprint 2019/311, section 9): a second to
        // last block of zeros (except the last byte) is changed on encryption,
        // and the attack's decrypted last block is refused.
        let nonce = [7u8; 16];
        let mut plain = vec![0u8; 32];
        plain[15] = 128;
        let (crypted, tag) = encrypt(&KEY, &nonce, &plain);
        let (decrypted, dectag) = decrypt(&KEY, &nonce, &crypted).expect("decrypts");
        assert_ne!(decrypted, plain, "the critical block must be changed");
        assert_eq!(dectag, tag);
    }
}
