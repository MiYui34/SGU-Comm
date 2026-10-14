//! Mumble TCP frames (u16 type, u32 length, body) and the few protobuf
//! fields the bridge reads.

pub const UDP_TUNNEL: u16 = 1;
pub const CRYPT_SETUP: u16 = 15;
pub const VERSION: u16 = 0;

// Murmur's own limit for a TCP message is 8 MiB.
const MAX_BODY: usize = 8 << 20;

#[derive(Debug, Clone)]
pub struct Frame {
    pub typ: u16,
    /// The whole frame, header included, to forward it unchanged.
    pub raw: Vec<u8>,
}

impl Frame {
    pub fn body(&self) -> &[u8] {
        &self.raw[6..]
    }
}

pub fn make_frame(typ: u16, body: &[u8]) -> Vec<u8> {
    let mut out = Vec::with_capacity(body.len() + 6);
    out.extend_from_slice(&typ.to_be_bytes());
    out.extend_from_slice(&(body.len() as u32).to_be_bytes());
    out.extend_from_slice(body);
    out
}

/// Cuts a byte stream into frames. Bytes can arrive in any chunk size.
#[derive(Default)]
pub struct Splitter {
    buf: Vec<u8>,
}

impl Splitter {
    pub fn feed(&mut self, data: &[u8]) -> Result<Vec<Frame>, String> {
        self.buf.extend_from_slice(data);
        let mut frames = Vec::new();
        let mut at = 0;
        while self.buf.len() - at >= 6 {
            let typ = u16::from_be_bytes([self.buf[at], self.buf[at + 1]]);
            let len = u32::from_be_bytes(self.buf[at + 2..at + 6].try_into().unwrap()) as usize;
            if len > MAX_BODY {
                return Err(format!("frame of {len} bytes is too large"));
            }
            if self.buf.len() - at < 6 + len {
                break;
            }
            frames.push(Frame { typ, raw: self.buf[at..at + 6 + len].to_vec() });
            at += 6 + len;
        }
        self.buf.drain(..at);
        Ok(frames)
    }
}

#[derive(Debug, Default, PartialEq, Eq)]
pub struct CryptSetup {
    pub key: Option<[u8; 16]>,
    pub client_nonce: Option<[u8; 16]>,
    pub server_nonce: Option<[u8; 16]>,
}

fn varint(data: &[u8], at: &mut usize) -> Option<u64> {
    let mut value = 0u64;
    for shift in (0..64).step_by(7) {
        let b = *data.get(*at)?;
        *at += 1;
        value |= ((b & 0x7f) as u64) << shift;
        if b & 0x80 == 0 {
            return Some(value);
        }
    }
    None
}

pub enum Value<'a> {
    Varint(u64),
    Bytes(&'a [u8]),
    Fixed,
}

/// The fields of a protobuf message as (field number, value), or None if
/// the message is broken.
pub fn fields(body: &[u8]) -> Option<Vec<(u64, Value<'_>)>> {
    let mut out = Vec::new();
    let mut at = 0;
    while at < body.len() {
        let tag = varint(body, &mut at)?;
        let value = match tag & 7 {
            0 => Value::Varint(varint(body, &mut at)?),
            1 | 5 => {
                at += if tag & 7 == 1 { 8 } else { 4 };
                Value::Fixed
            }
            2 => {
                let len = varint(body, &mut at)? as usize;
                let bytes = body.get(at..at.checked_add(len)?)?;
                at += len;
                Value::Bytes(bytes)
            }
            _ => return None,
        };
        out.push((tag >> 3, value));
    }
    (at == body.len()).then_some(out)
}

impl CryptSetup {
    /// Reads fields 1-3. Values that are not 16 bytes long are ignored.
    pub fn parse(body: &[u8]) -> Option<CryptSetup> {
        let mut out = CryptSetup::default();
        for (field, value) in fields(body)? {
            let Value::Bytes(bytes) = value else { continue };
            let bytes: Option<[u8; 16]> = bytes.try_into().ok();
            match field {
                1 => out.key = bytes,
                2 => out.client_nonce = bytes,
                3 => out.server_nonce = bytes,
                _ => {}
            }
        }
        Some(out)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn frame(typ: u16, body: &[u8]) -> Vec<u8> {
        let mut out = typ.to_be_bytes().to_vec();
        out.extend((body.len() as u32).to_be_bytes());
        out.extend(body);
        out
    }

    #[test]
    fn splits_frames_across_chunks() {
        let mut stream = frame(1, &[9, 9, 9]);
        stream.extend(frame(15, &[1, 2]));
        stream.extend(frame(3, &[]));
        let mut s = Splitter::default();
        let mut got = Vec::new();
        for b in &stream {
            got.extend(s.feed(&[*b]).unwrap());
        }
        assert_eq!(got.len(), 3);
        assert_eq!((got[0].typ, got[0].body()), (1, &[9u8, 9, 9][..]));
        assert_eq!((got[1].typ, got[1].body()), (15, &[1u8, 2][..]));
        assert_eq!(got[2].typ, 3);
        // The raw bytes are kept, to forward frames unchanged.
        let raw: Vec<u8> = got.iter().flat_map(|f| f.raw.clone()).collect();
        assert_eq!(raw, stream);
    }

    #[test]
    fn refuses_oversized_frames() {
        let mut header = 1u16.to_be_bytes().to_vec();
        header.extend((9u32 << 20).to_be_bytes());
        assert!(Splitter::default().feed(&header).is_err());
    }

    #[test]
    fn reads_crypt_setup() {
        // key (1), client_nonce (2), server_nonce (3): length-delimited bytes.
        let mut body = vec![0x0a, 16];
        body.extend([1u8; 16]);
        body.extend([0x12, 16]);
        body.extend([2u8; 16]);
        body.extend([0x1a, 16]);
        body.extend([3u8; 16]);
        let cs = CryptSetup::parse(&body).unwrap();
        assert_eq!(cs.key, Some([1; 16]));
        assert_eq!(cs.client_nonce, Some([2; 16]));
        assert_eq!(cs.server_nonce, Some([3; 16]));
        // A resync only carries the server nonce.
        let resync = CryptSetup::parse(&[0x1a, 16, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7]).unwrap();
        assert_eq!((resync.key, resync.server_nonce), (None, Some([7; 16])));
        assert!(CryptSetup::parse(&[0x0a, 40, 1]).is_none(), "truncated");
    }

    #[test]
    fn makes_frames() {
        assert_eq!(make_frame(1, &[5, 6]), frame(1, &[5, 6]));
    }
}
