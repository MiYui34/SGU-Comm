//! Local bridge: the web client talks WebSocket to 127.0.0.1, the bridge
//! talks TLS to the Mumble server. The same unchanged web client then works
//! against any Mumble server, without the Kallisti server.
//!
//! Only this app may use it: it listens on 127.0.0.1 only, needs a random
//! secret in the URL, and checks the Origin (websites in a normal browser can
//! also open WebSockets to localhost).

use std::io::ErrorKind;
use std::net::SocketAddr;
use std::sync::{Arc, Mutex};
use std::time::Instant;

use crate::frames::{self, CryptSetup, Splitter, CRYPT_SETUP, UDP_TUNNEL, VERSION};
use crate::pins::{self, Check, PinStore};
use crate::udp::{self, Incoming, UdpVoice};

use futures_util::{SinkExt, StreamExt};
use rand::RngCore;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream, UdpSocket};
use tokio_tungstenite::tungstenite::{
    handshake::server::{ErrorResponse, Request, Response},
    http::StatusCode,
    protocol::{frame::coding::CloseCode, CloseFrame},
    Message,
};

pub struct Bridge {
    pub port: u16,
    pub secret: String,
}

const DEFAULT_PORT: u16 = 64738;

/// Reads the target server from the WebSocket URL query and checks the
/// secret.
pub fn parse_target(query: &str, secret: &str) -> Result<(String, u16), &'static str> {
    let mut given_secret = None;
    let mut host = None;
    let mut port = None;
    for (key, value) in url::form_urlencoded::parse(query.as_bytes()) {
        match key.as_ref() {
            "secret" => given_secret = Some(value.into_owned()),
            "host" => host = Some(value.into_owned()),
            "port" => port = Some(value.into_owned()),
            _ => {}
        }
    }
    if !given_secret.is_some_and(|g| constant_time_eq(g.as_bytes(), secret.as_bytes())) {
        return Err("wrong secret");
    }
    let host = host.filter(|h| !h.is_empty()).ok_or("no host")?;
    let port = match port {
        None => DEFAULT_PORT,
        Some(p) => p.parse::<u16>().ok().filter(|p| *p > 0).ok_or("bad port")?,
    };
    Ok((host, port))
}

fn constant_time_eq(a: &[u8], b: &[u8]) -> bool {
    a.len() == b.len() && a.iter().zip(b).fold(0u8, |acc, (x, y)| acc | (x ^ y)) == 0
}

/// Starts the bridge on 127.0.0.1 with a random port and secret.
pub async fn start(allowed_origins: Vec<String>, pins: Arc<PinStore>) -> std::io::Result<Bridge> {
    let listener = TcpListener::bind("127.0.0.1:0").await?;
    let port = listener.local_addr()?.port();
    let mut bytes = [0u8; 24];
    rand::rng().fill_bytes(&mut bytes);
    let secret = hex::encode(bytes);
    let shared = Arc::new((secret.clone(), allowed_origins));
    tokio::spawn(async move {
        loop {
            match listener.accept().await {
                Ok((tcp, _)) => {
                    let (shared, pins) = (shared.clone(), pins.clone());
                    tokio::spawn(async move {
                        let _ = serve(tcp, &shared.0, &shared.1, &pins).await;
                    });
                }
                // For example too many open files: keep the bridge alive.
                Err(err) => {
                    eprintln!("bridge: accept failed: {err}");
                    tokio::time::sleep(std::time::Duration::from_millis(200)).await;
                }
            }
        }
    });
    Ok(Bridge { port, secret })
}

async fn serve(
    tcp: TcpStream,
    secret: &str,
    origins: &[String],
    pins: &PinStore,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let mut target = None;
    let check = |req: &Request, resp: Response| -> Result<Response, ErrorResponse> {
        let origin = req.headers().get("Origin").and_then(|o| o.to_str().ok()).unwrap_or("");
        let refuse = |status: StatusCode| {
            let mut r = ErrorResponse::new(None);
            *r.status_mut() = status;
            r
        };
        if !origins.iter().any(|o| o == origin) {
            return Err(refuse(StatusCode::FORBIDDEN));
        }
        match parse_target(req.uri().query().unwrap_or(""), secret) {
            Ok(t) => {
                target = Some(t);
                Ok(resp)
            }
            Err(_) => Err(refuse(StatusCode::FORBIDDEN)),
        }
    };
    // Idle local sockets must not pile up.
    let mut ws = tokio::time::timeout(HANDSHAKE_TIMEOUT, tokio_tungstenite::accept_hdr_async(tcp, check)).await??;
    let (host, port) = target.ok_or("no target")?;

    let (upstream, fp, trusted_ca, peer) = match connect_upstream(&host, port).await {
        Ok(u) => u,
        Err(err) => {
            // Tell the client why, instead of just dropping the socket.
            close_with(&mut ws, CloseCode::Error, format!("cannot reach {host}:{port}: {err}")).await;
            return Err(err);
        }
    };
    // Trust on first use: nothing from the client reaches an unknown or
    // changed server. The client asks the user and trusts via the app.
    let server = format!("{host}:{port}");
    let refused = refusal(trusted_ca, pins.check(&server, &fp), &fp);
    if refused.is_some() {
        pins.offer(&server, &fp);
    }
    if let Some(reason) = refused {
        close_with(&mut ws, CloseCode::Policy, reason).await;
        return Ok(());
    }
    let udp = udp_socket(peer).await.ok();
    let voice = Mutex::new(UdpVoice::default());
    let (mut up_read, up_write) = tokio::io::split(upstream);
    let up_write = tokio::sync::Mutex::new(up_write);
    let (ws_write, mut ws_read) = ws.split();
    let ws_write = tokio::sync::Mutex::new(ws_write);

    // Browser to server: binary frames are the Mumble stream. Voice goes
    // over UDP while UDP works, everything else over TLS.
    let to_server = async {
        let mut splitter = Splitter::default();
        while let Some(Ok(msg)) = ws_read.next().await {
            let data = match msg {
                Message::Binary(data) => data,
                Message::Close(_) => break,
                _ => continue, // no text API without the Kallisti server
            };
            let mut tcp = Vec::with_capacity(data.len());
            for frame in splitter.feed(&data).map_err(std::io::Error::other)? {
                let via_udp = udp.as_ref().filter(|_| frame.typ == UDP_TUNNEL);
                let packet = via_udp.and_then(|_| voice.lock().unwrap().outgoing(Instant::now(), frame.body()));
                match (via_udp, packet) {
                    (Some(socket), Some(packet)) => {
                        let _ = socket.send(&packet).await;
                    }
                    _ => tcp.extend_from_slice(&frame.raw),
                }
            }
            if !tcp.is_empty() {
                up_write.lock().await.write_all(&tcp).await?;
            }
        }
        up_write.lock().await.shutdown().await
    };
    // Server to browser, unchanged. The bridge reads the server's Version
    // (UDP format) and CryptSetup (UDP keys) on the way. Only whole frames
    // go to the browser, so UDP voice never lands inside a TCP frame.
    let to_browser = async {
        let mut splitter = Splitter::default();
        let mut buf = vec![0u8; 32 * 1024];
        loop {
            let n = up_read.read(&mut buf).await?;
            if n == 0 {
                break;
            }
            let mut out = Vec::with_capacity(n);
            for frame in splitter.feed(&buf[..n]).map_err(std::io::Error::other)? {
                match frame.typ {
                    VERSION => voice.lock().unwrap().set_legacy(udp::is_legacy_server(frame.body())),
                    CRYPT_SETUP => {
                        let reply = CryptSetup::parse(frame.body()).and_then(|cs| voice.lock().unwrap().on_crypt_setup(&cs));
                        if let Some(reply) = reply {
                            up_write.lock().await.write_all(&reply).await?;
                        }
                        send_ping(&udp, &voice).await; // find out at once if UDP works
                    }
                    _ => {}
                }
                out.extend_from_slice(&frame.raw);
            }
            if !out.is_empty() && ws_write.lock().await.send(Message::Binary(out.into())).await.is_err() {
                break;
            }
        }
        let _ = ws_write.lock().await.close().await;
        Ok::<_, std::io::Error>(())
    };
    // UDP from the server, and pings.
    let udp_loop = async {
        let Some(socket) = &udp else { return std::future::pending::<std::io::Result<()>>().await };
        let mut buf = vec![0u8; 2048];
        let mut pings = tokio::time::interval(udp::PING_INTERVAL);
        loop {
            tokio::select! {
                _ = pings.tick() => {
                    let to_tcp = voice.lock().unwrap().check(Instant::now());
                    if let Some(frame) = to_tcp {
                        up_write.lock().await.write_all(&frame).await?;
                    }
                    send_ping(&udp, &voice).await;
                }
                r = socket.recv(&mut buf) => {
                    let n = match r {
                        Ok(n) => n,
                        // ICMP replies (port unreachable): no UDP for now.
                        Err(e) if matches!(e.kind(), ErrorKind::ConnectionRefused | ErrorKind::ConnectionReset) => continue,
                        // Anything else: give up on UDP, keep the TCP session.
                        Err(_) => return std::future::pending().await,
                    };
                    let incoming = voice.lock().unwrap().incoming(Instant::now(), &buf[..n]);
                    match incoming {
                        Incoming::Audio(plain) => {
                            let frame = frames::make_frame(UDP_TUNNEL, &plain);
                            if ws_write.lock().await.send(Message::Binary(frame.into())).await.is_err() {
                                return Ok(());
                            }
                        }
                        Incoming::Resync(request) => up_write.lock().await.write_all(&request).await?,
                        Incoming::Pong | Incoming::Dropped => {}
                    }
                }
            }
        }
    };
    // Whichever side ends first ends the connection.
    tokio::select! {
        r = to_server => r?,
        r = to_browser => r?,
        r = udp_loop => r?,
    }
    Ok(())
}

async fn udp_socket(peer: SocketAddr) -> std::io::Result<UdpSocket> {
    let local = if peer.is_ipv4() { "0.0.0.0:0" } else { "[::]:0" };
    let socket = UdpSocket::bind(local).await?;
    socket.connect(peer).await?;
    Ok(socket)
}

async fn send_ping(udp: &Option<UdpSocket>, voice: &Mutex<UdpVoice>) {
    let Some(socket) = udp else { return };
    let ping = voice.lock().unwrap().ping();
    if let Some(ping) = ping {
        let _ = socket.send(&ping).await;
    }
}

const HANDSHAKE_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(10);

/// Closes with a reason and waits (briefly) for the client's close reply.
/// The client may already have sent data (the Mumble handshake): dropping
/// the socket with unread data makes the OS send a TCP reset, and browsers
/// then lose the close frame (code 1006 instead of our reason).
async fn close_with(ws: &mut tokio_tungstenite::WebSocketStream<TcpStream>, code: CloseCode, reason: String) {
    if ws.close(Some(CloseFrame { code, reason: reason.into() })).await.is_err() {
        return;
    }
    let _ = tokio::time::timeout(std::time::Duration::from_secs(2), async {
        while let Some(Ok(_)) = ws.next().await {}
    })
    .await;
}

/// What to refuse a server for: nothing when the web PKI trusts its
/// certificate for the name (renewals need no new pin), else trust on
/// first use with the pins.
fn refusal(ca_valid: bool, check: Check, fp: &str) -> Option<String> {
    match (ca_valid, check) {
        (true, _) | (false, Check::Trusted) => None,
        (false, Check::Unknown) => Some(format!("certificate-unknown {fp}")),
        (false, Check::Changed) => Some(format!("certificate-changed {fp}")),
    }
}

/// Whether the web PKI (the usual certificate authorities) trusts the
/// certificate for the name, for example one from Let's Encrypt.
fn ca_valid(
    end_entity: &rustls::pki_types::CertificateDer<'_>,
    intermediates: &[rustls::pki_types::CertificateDer<'_>],
    name: &rustls::pki_types::ServerName<'_>,
    now: rustls::pki_types::UnixTime,
) -> bool {
    use rustls::client::danger::ServerCertVerifier;
    static VERIFIER: std::sync::OnceLock<Option<Arc<rustls::client::WebPkiServerVerifier>>> = std::sync::OnceLock::new();
    let verifier = VERIFIER.get_or_init(|| {
        let roots = rustls::RootCertStore { roots: webpki_roots::TLS_SERVER_ROOTS.to_vec() };
        rustls::client::WebPkiServerVerifier::builder_with_provider(Arc::new(roots), Arc::new(rustls::crypto::ring::default_provider()))
            .build()
            .ok()
    });
    verifier.as_ref().is_some_and(|v| v.verify_server_cert(end_entity, intermediates, name, &[], now).is_ok())
}

/// Connects with TLS. Returns the server certificate's fingerprint, whether
/// the web PKI trusts it, and the server address (for UDP).
async fn connect_upstream(
    host: &str,
    port: u16,
) -> Result<(tokio_rustls::client::TlsStream<TcpStream>, String, bool, SocketAddr), Box<dyn std::error::Error + Send + Sync>> {
    let tcp = tokio::time::timeout(HANDSHAKE_TIMEOUT, TcpStream::connect((host, port))).await??;
    tcp.set_nodelay(true)?;
    let peer = tcp.peer_addr()?;
    let name = rustls::pki_types::ServerName::try_from(host.to_string())?;
    let seen = Arc::new(Mutex::new(None));
    let tls = tokio_rustls::TlsConnector::from(Arc::new(tls_config(seen.clone())));
    let stream = tls.connect(name, tcp).await?;
    let (fp, trusted) = seen.lock().unwrap().take().ok_or("no server certificate")?;
    Ok((stream, fp, trusted, peer))
}

/// Mumble servers usually have self-signed certificates, so the CA check
/// does not decide alone: the verifier records the certificate's
/// fingerprint and whether the web PKI trusts it, and the bridge decides
/// (see refusal).
fn tls_config(seen: Arc<Mutex<Option<(String, bool)>>>) -> rustls::ClientConfig {
    let provider = Arc::new(rustls::crypto::ring::default_provider());
    rustls::ClientConfig::builder_with_provider(provider.clone())
        .with_safe_default_protocol_versions()
        .expect("TLS versions")
        .dangerous()
        .with_custom_certificate_verifier(Arc::new(Recording { provider, seen }))
        .with_no_client_auth()
}

#[derive(Debug)]
struct Recording {
    provider: Arc<rustls::crypto::CryptoProvider>,
    seen: Arc<Mutex<Option<(String, bool)>>>,
}

impl rustls::client::danger::ServerCertVerifier for Recording {
    fn verify_server_cert(
        &self,
        end_entity: &rustls::pki_types::CertificateDer<'_>,
        intermediates: &[rustls::pki_types::CertificateDer<'_>],
        server_name: &rustls::pki_types::ServerName<'_>,
        _ocsp: &[u8],
        now: rustls::pki_types::UnixTime,
    ) -> Result<rustls::client::danger::ServerCertVerified, rustls::Error> {
        let trusted = ca_valid(end_entity, intermediates, server_name, now);
        *self.seen.lock().unwrap() = Some((pins::fingerprint(end_entity), trusted));
        Ok(rustls::client::danger::ServerCertVerified::assertion())
    }

    // Signatures are still checked: the connection is encrypted to whoever
    // holds the certificate's key.
    fn verify_tls12_signature(
        &self,
        message: &[u8],
        cert: &rustls::pki_types::CertificateDer<'_>,
        dss: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls12_signature(message, cert, dss, &self.provider.signature_verification_algorithms)
    }

    fn verify_tls13_signature(
        &self,
        message: &[u8],
        cert: &rustls::pki_types::CertificateDer<'_>,
        dss: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls13_signature(message, cert, dss, &self.provider.signature_verification_algorithms)
    }

    fn supported_verify_schemes(&self) -> Vec<rustls::SignatureScheme> {
        self.provider.signature_verification_algorithms.supported_schemes()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn certificates_valid_for_the_name_need_no_pin() {
        assert_eq!(refusal(true, Check::Unknown, "ab"), None);
        assert_eq!(refusal(true, Check::Changed, "ab"), None);
        assert_eq!(refusal(false, Check::Trusted, "ab"), None);
        assert_eq!(refusal(false, Check::Unknown, "ab"), Some("certificate-unknown ab".into()));
        assert_eq!(refusal(false, Check::Changed, "ab"), Some("certificate-changed ab".into()));
    }

    // Needs the network: cargo test -- --ignored
    #[tokio::test]
    #[ignore]
    async fn a_lets_encrypt_certificate_is_ca_valid() {
        let (_, _, trusted, _) = connect_upstream("chat.kallisti.chat", 443).await.unwrap();
        assert!(trusted);
        let (_, _, trusted, _) = connect_upstream("chat.kallisti.chat", 64738).await.unwrap();
        assert!(!trusted); // self-signed until the server uses the web certificate
    }

    #[test]
    fn self_signed_is_not_ca_valid() {
        let cert = rcgen::generate_simple_self_signed(vec!["chat.example".into()]).unwrap();
        let der = rustls::pki_types::CertificateDer::from(cert.cert.der().to_vec());
        let name = rustls::pki_types::ServerName::try_from("chat.example").unwrap();
        assert!(!ca_valid(&der, &[], &name, rustls::pki_types::UnixTime::now()));
    }

    #[test]
    fn needs_the_right_secret() {
        assert!(parse_target("host=a.example", "s3cret").is_err());
        assert!(parse_target("secret=wrong&host=a.example", "s3cret").is_err());
    }

    #[test]
    fn reads_host_and_port() {
        assert_eq!(parse_target("secret=s3cret&host=a.example&port=1234", "s3cret"), Ok(("a.example".into(), 1234)));
        assert_eq!(parse_target("secret=s3cret&host=a.example", "s3cret"), Ok(("a.example".into(), 64738)));
    }

    #[test]
    fn refuses_bad_targets() {
        assert!(parse_target("secret=s3cret", "s3cret").is_err());
        assert!(parse_target("secret=s3cret&host=a.example&port=99999", "s3cret").is_err());
        assert!(parse_target("secret=s3cret&host=a.example&port=x", "s3cret").is_err());
    }
}
