use std::sync::Arc;

use futures_util::{SinkExt, StreamExt};
use kallisti_desktop::{bridge, pins};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;
use tokio_tungstenite::tungstenite::{client::IntoClientRequest, Message};

/// A TLS server with a self-signed certificate that echoes bytes back.
/// Returns its port and certificate fingerprint.
async fn tls_echo() -> (u16, String) {
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
    ws.send(Message::Binary(vec![1, 2, 3].into())).await.unwrap();
    match ws.next().await {
        Some(Ok(Message::Binary(data))) => assert_eq!(&data[..], &[1, 2, 3]),
        other => panic!("got {other:?}"),
    }
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

/// Against a real Murmur: KALLISTI_TEST_MURMUR=127.0.0.1:64738 cargo test
#[tokio::test]
async fn syncs_with_a_real_murmur() {
    let Ok(target) = std::env::var("KALLISTI_TEST_MURMUR") else { return };
    let (host, port) = target.rsplit_once(':').unwrap();
    // Trust whatever the test server has: this test is about the protocol.
    let store = Arc::new(pins::PinStore::in_memory());
    let b = bridge::start(vec![ORIGIN.into()], store.clone()).await.unwrap();
    let probe = format!("ws://127.0.0.1:{}/?secret={}&host={}&port={}", b.port, b.secret, host, port);
    let mut req = probe.into_client_request().unwrap();
    req.headers_mut().insert("Origin", ORIGIN.parse().unwrap());
    let (mut first, _) = tokio_tungstenite::connect_async(req).await.unwrap();
    let reason = close_reason(&mut first).await;
    let fp = reason.strip_prefix("certificate-unknown ").expect("first connection must ask");
    store.trust(&target, fp).unwrap();
    let url = format!("ws://127.0.0.1:{}/?secret={}&host={}&port={}", b.port, b.secret, host, port);
    let mut req = url.into_client_request().unwrap();
    req.headers_mut().insert("Origin", ORIGIN.parse().unwrap());
    let (mut ws, _) = tokio_tungstenite::connect_async(req).await.unwrap();
    ws.send(Message::Binary(handshake("rust-bridge-test").into())).await.unwrap();
    let mut stream = Vec::new();
    let found = tokio::time::timeout(std::time::Duration::from_secs(5), async {
        while let Some(Ok(Message::Binary(data))) = ws.next().await {
            stream.extend_from_slice(&data);
            // Walk the frames: type 5 is ServerSync.
            let mut i = 0;
            while i + 6 <= stream.len() {
                let typ = u16::from_be_bytes([stream[i], stream[i + 1]]);
                let len = u32::from_be_bytes([stream[i + 2], stream[i + 3], stream[i + 4], stream[i + 5]]) as usize;
                if i + 6 + len > stream.len() {
                    break;
                }
                if typ == 5 {
                    return true;
                }
                i += 6 + len;
            }
        }
        false
    })
    .await;
    assert_eq!(found, Ok(true), "no ServerSync from Murmur");
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
