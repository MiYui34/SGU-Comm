//! Local bridge: the web client talks WebSocket to 127.0.0.1, the bridge
//! talks TLS to the Mumble server. The same unchanged web client then works
//! against any Mumble server, without the Kallisti server.
//!
//! Only this app may use it: it listens on 127.0.0.1 only, needs a random
//! secret in the URL, and checks the Origin (websites in a normal browser can
//! also open WebSockets to localhost).

use std::sync::Arc;

use futures_util::{SinkExt, StreamExt};
use rand::RngCore;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};
use tokio_tungstenite::tungstenite::{
    handshake::server::{ErrorResponse, Request, Response},
    http::StatusCode,
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
    if given_secret.as_deref() != Some(secret) {
        return Err("wrong secret");
    }
    let host = host.filter(|h| !h.is_empty()).ok_or("no host")?;
    let port = match port {
        None => DEFAULT_PORT,
        Some(p) => p.parse::<u16>().ok().filter(|p| *p > 0).ok_or("bad port")?,
    };
    Ok((host, port))
}

/// Starts the bridge on 127.0.0.1 with a random port and secret.
pub async fn start(allowed_origins: Vec<String>) -> std::io::Result<Bridge> {
    let listener = TcpListener::bind("127.0.0.1:0").await?;
    let port = listener.local_addr()?.port();
    let mut bytes = [0u8; 24];
    rand::rng().fill_bytes(&mut bytes);
    let secret = hex::encode(bytes);
    let tls = tokio_rustls::TlsConnector::from(Arc::new(tls_config()));
    let shared = Arc::new((secret.clone(), allowed_origins));
    tokio::spawn(async move {
        while let Ok((tcp, _)) = listener.accept().await {
            let (shared, tls) = (shared.clone(), tls.clone());
            tokio::spawn(async move {
                let _ = serve(tcp, &shared.0, &shared.1, tls).await;
            });
        }
    });
    Ok(Bridge { port, secret })
}

async fn serve(
    tcp: TcpStream,
    secret: &str,
    origins: &[String],
    tls: tokio_rustls::TlsConnector,
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
    let ws = tokio_tungstenite::accept_hdr_async(tcp, check).await?;
    let (host, port) = target.ok_or("no target")?;

    let upstream = TcpStream::connect((host.as_str(), port)).await?;
    upstream.set_nodelay(true)?;
    let name = rustls::pki_types::ServerName::try_from(host.clone())?;
    let upstream = tls.connect(name, upstream).await?;
    let (mut up_read, mut up_write) = tokio::io::split(upstream);
    let (mut ws_write, mut ws_read) = ws.split();

    // Browser to server: binary frames are the Mumble stream.
    let to_server = async {
        while let Some(Ok(msg)) = ws_read.next().await {
            match msg {
                Message::Binary(data) => up_write.write_all(&data).await?,
                Message::Close(_) => break,
                _ => {} // no text API without the Kallisti server
            }
        }
        up_write.shutdown().await
    };
    // Server to browser.
    let to_browser = async {
        let mut buf = vec![0u8; 32 * 1024];
        loop {
            let n = up_read.read(&mut buf).await?;
            if n == 0 {
                break;
            }
            if ws_write.send(Message::Binary(buf[..n].to_vec().into())).await.is_err() {
                break;
            }
        }
        let _ = ws_write.close().await;
        Ok::<_, std::io::Error>(())
    };
    // Whichever side ends first ends the connection.
    tokio::select! {
        r = to_server => r?,
        r = to_browser => r?,
    }
    Ok(())
}

/// Mumble servers usually have self-signed certificates. Like the desktop
/// Mumble client's first connection, accept them (trust on first use with
/// pinning is a follow-up).
fn tls_config() -> rustls::ClientConfig {
    let provider = Arc::new(rustls::crypto::ring::default_provider());
    rustls::ClientConfig::builder_with_provider(provider.clone())
        .with_safe_default_protocol_versions()
        .expect("TLS versions")
        .dangerous()
        .with_custom_certificate_verifier(Arc::new(AcceptAny(provider)))
        .with_no_client_auth()
}

#[derive(Debug)]
struct AcceptAny(Arc<rustls::crypto::CryptoProvider>);

impl rustls::client::danger::ServerCertVerifier for AcceptAny {
    fn verify_server_cert(
        &self,
        _end_entity: &rustls::pki_types::CertificateDer<'_>,
        _intermediates: &[rustls::pki_types::CertificateDer<'_>],
        _server_name: &rustls::pki_types::ServerName<'_>,
        _ocsp: &[u8],
        _now: rustls::pki_types::UnixTime,
    ) -> Result<rustls::client::danger::ServerCertVerified, rustls::Error> {
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
        rustls::crypto::verify_tls12_signature(message, cert, dss, &self.0.signature_verification_algorithms)
    }

    fn verify_tls13_signature(
        &self,
        message: &[u8],
        cert: &rustls::pki_types::CertificateDer<'_>,
        dss: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls13_signature(message, cert, dss, &self.0.signature_verification_algorithms)
    }

    fn supported_verify_schemes(&self) -> Vec<rustls::SignatureScheme> {
        self.0.signature_verification_algorithms.supported_schemes()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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
