use std::sync::Arc;

use futures_util::{SinkExt, StreamExt};
use kallisti_desktop::frames::{Frame, Splitter};
use kallisti_desktop::{bridge, pins};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;
use tokio_tungstenite::tungstenite::{client::IntoClientRequest, Message};

/// A TLS server with a self-signed certificate that echoes bytes back.
/// Returns its port and certificate fingerprint.
async fn tls_echo() -> (u16, String) {
    tls_echo_after(std::time::Duration::ZERO).await
}

/// The same, but it waits before the TLS handshake.
async fn tls_echo_after(delay: std::time::Duration) -> (u16, String) {
    let cert = rcgen::generate_simple_self_signed(vec!["localhost".into()]).unwrap();
    let key = rustls::pki_types::PrivateKeyDer::Pkcs8(cert.signing_key.serialize_der().into());
    let config = rustls::ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
        .with_safe_default_protocol_versions()
        .unwrap()
        .with_no_client_auth()
        .with_single_cert(vec![cert.cert.der().clone()], key)
        .unwrap();
    let fp = pins::fingerprint(cert.cert.der());
    let acceptor = tokio_rustls::TlsAcceptor::from(Arc::new(config));
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let port = listener.local_addr().unwrap().port();
    tokio::spawn(async move {
        loop {
            let (tcp, _) = listener.accept().await.unwrap();
            let acceptor = acceptor.clone();
            tokio::spawn(async move {
                tokio::time::sleep(delay).await;
                let mut tls = acceptor.accept(tcp).await.unwrap();
                let mut buf = [0u8; 1024];
                loop {
                    match tls.read(&mut buf).await {
                        Ok(0) | Err(_) => return,
                        Ok(n) => tls.write_all(&buf[..n]).await.unwrap(),
                    }
                }
            });
        }
    });
    (port, fp)
}

async fn connect(b: &bridge::Bridge, secret: &str, target: u16, origin: &str) -> Result<
    tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>,
    tokio_tungstenite::tungstenite::Error,
> {
    let url = format!("ws://127.0.0.1:{}/?secret={}&host=127.0.0.1&port={}", b.port, secret, target);
    let mut req = url.into_client_request().unwrap();
    req.headers_mut().insert("Origin", origin.parse().unwrap());
    tokio_tungstenite::connect_async(req).await.map(|(ws, _)| ws)
}

const ORIGIN: &str = "tauri://localhost";

/// A bridge that already trusts the echo server.
async fn trusting(port: u16, fp: &str) -> bridge::Bridge {
    let store = Arc::new(pins::PinStore::in_memory());
    store.trust(&format!("127.0.0.1:{port}"), fp).unwrap();
    bridge::start(vec![ORIGIN.into()], store).await.unwrap()
}

async fn close_reason(
    ws: &mut tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>,
) -> String {
    match ws.next().await {
        Some(Ok(Message::Close(Some(frame)))) => frame.reason.to_string(),
        other => panic!("got {other:?}, want a close frame"),
    }
}

#[tokio::test]
async fn forwards_bytes_both_ways() {
    let (target, fp) = tls_echo().await;
    let b = trusting(target, &fp).await;
    let mut ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    // The bridge reads the client's stream frame by frame.
    let msg = frame(3, &[1, 2, 3]);
    ws.send(Message::Binary(msg.clone().into())).await.unwrap();
    assert_eq!(read_bytes(&mut ws, msg.len()).await, msg);
}

type Ws = tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>;

async fn read_bytes(ws: &mut Ws, n: usize) -> Vec<u8> {
    let mut got = Vec::new();
    while got.len() < n {
        match tokio::time::timeout(std::time::Duration::from_secs(5), ws.next()).await {
            Ok(Some(Ok(Message::Binary(data)))) => got.extend_from_slice(&data),
            other => panic!("got {other:?} after {} bytes", got.len()),
        }
    }
    got
}

#[tokio::test]
async fn voice_stays_on_tcp_when_udp_does_not_answer() {
    // The echo server sends our CryptSetup back, so the bridge has keys,
    // but nothing answers UDP pings.
    let (target, fp) = tls_echo().await;
    let b = trusting(target, &fp).await;
    let mut ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    let mut setup = vec![0x0a, 16];
    setup.extend([1u8; 16]);
    setup.extend([0x12, 16]);
    setup.extend([2u8; 16]);
    setup.extend([0x1a, 16]);
    setup.extend([3u8; 16]);
    let setup = frame(15, &setup);
    ws.send(Message::Binary(setup.clone().into())).await.unwrap();
    assert_eq!(read_bytes(&mut ws, setup.len()).await, setup);
    tokio::time::sleep(std::time::Duration::from_millis(300)).await;
    let voice = frame(1, &[0, 0x20, 1, 0x2a, 1, 0xfc]);
    ws.send(Message::Binary(voice.clone().into())).await.unwrap();
    assert_eq!(read_bytes(&mut ws, voice.len()).await, voice);
}

#[tokio::test]
async fn refuses_a_wrong_secret() {
    let (target, fp) = tls_echo().await;
    let b = trusting(target, &fp).await;
    assert!(connect(&b, "wrong", target, ORIGIN).await.is_err());
}

#[tokio::test]
async fn refuses_other_origins() {
    let (target, fp) = tls_echo().await;
    let b = trusting(target, &fp).await;
    assert!(connect(&b, &b.secret, target, "https://evil.example").await.is_err());
}

#[tokio::test]
async fn secrets_are_random() {
    let a = bridge::start(vec![], Arc::new(pins::PinStore::in_memory())).await.unwrap();
    let b = bridge::start(vec![], Arc::new(pins::PinStore::in_memory())).await.unwrap();
    assert_ne!(a.secret, b.secret);
    assert!(a.secret.len() >= 32);
}

// Protobuf by hand: enough for Version and Authenticate.
fn varint(mut v: u64, out: &mut Vec<u8>) {
    while v >= 0x80 {
        out.push((v as u8) | 0x80);
        v >>= 7;
    }
    out.push(v as u8);
}

fn frame(typ: u16, body: &[u8]) -> Vec<u8> {
    let mut out = typ.to_be_bytes().to_vec();
    out.extend((body.len() as u32).to_be_bytes());
    out.extend(body);
    out
}

fn handshake(name: &str) -> Vec<u8> {
    let mut version = vec![0x08];
    varint(0x010500, &mut version);
    let mut auth = vec![0x0a];
    varint(name.len() as u64, &mut auth);
    auth.extend(name.as_bytes());
    auth.extend([0x28, 0x01]); // opus = true
    let mut out = frame(0, &version);
    out.extend(frame(2, &auth));
    out
}

/// Connects through a bridge to the real Murmur in KALLISTI_TEST_MURMUR
/// (for example 127.0.0.1:64738), trusting its certificate.
async fn murmur() -> Option<Ws> {
    let target = std::env::var("KALLISTI_TEST_MURMUR").ok()?;
    let (host, port) = target.rsplit_once(':').unwrap();
    let store = Arc::new(pins::PinStore::in_memory());
    let b = bridge::start(vec![ORIGIN.into()], store.clone()).await.unwrap();
    let url = format!("ws://127.0.0.1:{}/?secret={}&host={}&port={}", b.port, b.secret, host, port);
    let open = || async {
        let mut req = url.clone().into_client_request().unwrap();
        req.headers_mut().insert("Origin", ORIGIN.parse().unwrap());
        tokio_tungstenite::connect_async(req).await.unwrap().0
    };
    let reason = close_reason(&mut open().await).await;
    let fp = reason.strip_prefix("certificate-unknown ").unwrap_or_else(|| panic!("first connection must ask, got: {reason}"));
    store.trust(&target, fp).unwrap();
    Some(open().await)
}

/// Reads frames until one of the given type arrives.
async fn wait_for(ws: &mut Ws, splitter: &mut Splitter, typ: u16) -> Frame {
    let found = tokio::time::timeout(std::time::Duration::from_secs(5), async {
        while let Some(Ok(Message::Binary(data))) = ws.next().await {
            if let Some(f) = splitter.feed(&data).unwrap().into_iter().find(|f| f.typ == typ) {
                return Some(f);
            }
        }
        None
    })
    .await;
    found.ok().flatten().unwrap_or_else(|| panic!("no frame of type {typ} from Murmur"))
}

fn varint_field(body: &[u8], field: u64) -> Option<u64> {
    use kallisti_desktop::frames::{fields, Value};
    fields(body)?.into_iter().find_map(|(f, v)| match v {
        Value::Varint(n) if f == field => Some(n),
        _ => None,
    })
}

/// Protobuf audio: target 0, frame number, a tiny Opus frame.
fn voice_frame(i: u8) -> Vec<u8> {
    frame(1, &[0, 0x08, 0, 0x20, i, 0x2a, 1, 0xfc])
}

#[tokio::test]
async fn syncs_with_a_real_murmur() {
    let Some(mut ws) = murmur().await else { return };
    ws.send(Message::Binary(handshake("rust-bridge-test").into())).await.unwrap();
    wait_for(&mut ws, &mut Splitter::default(), 5).await; // ServerSync
}

#[tokio::test]
async fn voice_goes_over_udp_to_a_real_murmur() {
    let Some(mut listener) = murmur().await else { return };
    let mut listener_frames = Splitter::default();
    listener.send(Message::Binary(handshake("rust-udp-listener").into())).await.unwrap();
    wait_for(&mut listener, &mut listener_frames, 5).await;
    // Murmur sends voice over UDP only to clients that sent voice over UDP.
    tokio::time::sleep(std::time::Duration::from_secs(1)).await;
    listener.send(Message::Binary(voice_frame(0).into())).await.unwrap();
    let mut ws = murmur().await.unwrap();
    let mut splitter = Splitter::default();
    ws.send(Message::Binary(handshake("rust-udp-test").into())).await.unwrap();
    let sync = wait_for(&mut ws, &mut splitter, 5).await;
    let session = varint_field(sync.body(), 1).expect("session");
    // Time for the UDP ping and pong.
    tokio::time::sleep(std::time::Duration::from_secs(1)).await;
    for i in 0..10u8 {
        ws.send(Message::Binary(voice_frame(i).into())).await.unwrap();
    }
    tokio::time::sleep(std::time::Duration::from_millis(300)).await;
    let mut request = vec![0x08];
    varint(session, &mut request);
    ws.send(Message::Binary(frame(22, &request).into())).await.unwrap();
    let stats = wait_for(&mut ws, &mut splitter, 22).await;
    // from_client.good: packets Murmur decrypted from our UDP socket.
    // (udp_packets is only what clients report about themselves.)
    use kallisti_desktop::frames::{fields, Value};
    let from_client = fields(stats.body()).unwrap().into_iter().find_map(|(f, v)| match v {
        Value::Bytes(b) if f == 4 => Some(b.to_vec()),
        _ => None,
    });
    let good = from_client.and_then(|b| varint_field(&b, 1)).unwrap_or(0);
    assert!(good >= 10, "Murmur decrypted {good} UDP packets");
    // Murmur sends the listener voice over UDP now; the bridge hands it over
    // as UDPTunnel frames.
    let heard = wait_for(&mut listener, &mut listener_frames, 1).await;
    assert_eq!(heard.body()[0], 0, "protobuf audio");
}

#[tokio::test]
async fn tells_the_client_when_the_server_cannot_be_reached() {
    // A port where nothing listens.
    let closed = TcpListener::bind("127.0.0.1:0").await.unwrap().local_addr().unwrap().port();
    let b = bridge::start(vec![ORIGIN.into()], Arc::new(pins::PinStore::in_memory())).await.unwrap();
    let mut ws = connect(&b, &b.secret, closed, ORIGIN).await.unwrap();
    match ws.next().await {
        Some(Ok(Message::Close(Some(frame)))) => {
            assert_eq!(u16::from(frame.code), 1011);
            assert!(!frame.reason.is_empty());
        }
        other => panic!("got {other:?}, want a close frame with code 1011"),
    }
}

#[tokio::test]
async fn asks_about_unknown_certificates() {
    let (target, fp) = tls_echo().await;
    let b = bridge::start(vec![ORIGIN.into()], Arc::new(pins::PinStore::in_memory())).await.unwrap();
    let mut ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    assert_eq!(close_reason(&mut ws).await, format!("certificate-unknown {fp}"));
}

#[tokio::test]
async fn refuses_changed_certificates() {
    let (target, fp) = tls_echo().await;
    let b = trusting(target, "sha256:somethingelse").await;
    let mut ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    assert_eq!(close_reason(&mut ws).await, format!("certificate-changed {fp}"));
}

#[tokio::test]
async fn the_close_reason_arrives_even_when_the_client_already_sent_data() {
    // Like the web client: it sends the Mumble handshake at once. Unread
    // data must not turn the close into a TCP reset (code 1006 in browsers).
    let (target, fp) = tls_echo().await;
    let b = bridge::start(vec![ORIGIN.into()], Arc::new(pins::PinStore::in_memory())).await.unwrap();
    let mut ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    ws.send(Message::Binary(vec![0u8; 4096].into())).await.unwrap();
    tokio::time::sleep(std::time::Duration::from_millis(100)).await;
    assert_eq!(close_reason(&mut ws).await, format!("certificate-unknown {fp}"));
}

#[tokio::test]
async fn data_sent_while_the_bridge_connects_reaches_the_server() {
    let (target, fp) = tls_echo_after(std::time::Duration::from_millis(300)).await;
    let b = trusting(target, &fp).await;
    let mut ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    let msg = frame(3, &[1, 2, 3]);
    ws.send(Message::Binary(msg.clone().into())).await.unwrap();
    assert_eq!(read_bytes(&mut ws, msg.len()).await, msg);
}

#[tokio::test]
async fn a_client_that_leaves_cancels_the_connection_to_the_server() {
    // The server accepts TCP but never answers TLS.
    let stalled = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let target = stalled.local_addr().unwrap().port();
    let b = bridge::start(vec![ORIGIN.into()], Arc::new(pins::PinStore::in_memory())).await.unwrap();
    let ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    let (mut upstream, _) = stalled.accept().await.unwrap();
    drop(ws);
    // Well before the 10 s timeout, the bridge closes its side (after its
    // TLS hello).
    let closed = tokio::time::timeout(std::time::Duration::from_secs(2), async {
        while let Ok(1..) = upstream.read(&mut [0u8; 1024]).await {}
    })
    .await;
    assert!(closed.is_ok(), "still open");
}
