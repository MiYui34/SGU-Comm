use std::sync::Arc;

use futures_util::{SinkExt, StreamExt};
use kallisti_desktop::bridge;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;
use tokio_tungstenite::tungstenite::{client::IntoClientRequest, Message};

/// A TLS server with a self-signed certificate that echoes bytes back.
async fn tls_echo() -> u16 {
    let cert = rcgen::generate_simple_self_signed(vec!["localhost".into()]).unwrap();
    let key = rustls::pki_types::PrivateKeyDer::Pkcs8(cert.signing_key.serialize_der().into());
    let config = rustls::ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
        .with_safe_default_protocol_versions()
        .unwrap()
        .with_no_client_auth()
        .with_single_cert(vec![cert.cert.der().clone()], key)
        .unwrap();
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
    port
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

#[tokio::test]
async fn forwards_bytes_both_ways() {
    let target = tls_echo().await;
    let b = bridge::start(vec![ORIGIN.into()]).await.unwrap();
    let mut ws = connect(&b, &b.secret, target, ORIGIN).await.unwrap();
    ws.send(Message::Binary(vec![1, 2, 3].into())).await.unwrap();
    match ws.next().await {
        Some(Ok(Message::Binary(data))) => assert_eq!(&data[..], &[1, 2, 3]),
        other => panic!("got {other:?}"),
    }
}

#[tokio::test]
async fn refuses_a_wrong_secret() {
    let target = tls_echo().await;
    let b = bridge::start(vec![ORIGIN.into()]).await.unwrap();
    assert!(connect(&b, "wrong", target, ORIGIN).await.is_err());
}

#[tokio::test]
async fn refuses_other_origins() {
    let target = tls_echo().await;
    let b = bridge::start(vec![ORIGIN.into()]).await.unwrap();
    assert!(connect(&b, &b.secret, target, "https://evil.example").await.is_err());
}

#[tokio::test]
async fn secrets_are_random() {
    let a = bridge::start(vec![]).await.unwrap();
    let b = bridge::start(vec![]).await.unwrap();
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
    let b = bridge::start(vec![ORIGIN.into()]).await.unwrap();
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
