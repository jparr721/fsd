use api::rest::{define_api, define_logging_layer};
use axum::Router;
use fsd::{
    service::{Fsd, ShutdownToken},
    woccy::bfs,
};
use std::{env, path::Path, sync::Once};
use tracing::info;
use tracing_subscriber::{fmt::format::FmtSpan, EnvFilter};

pub mod api;
pub mod fsd;

static LOG_INIT: Once = Once::new();

pub fn init_logging() {
    LOG_INIT.call_once(|| {
        if env::var("RUST_LOG_FORMAT") == Ok("json".to_string()) {
            tracing_subscriber::fmt()
                .with_env_filter(EnvFilter::from_default_env())
                .with_span_events(FmtSpan::NEW)
                .json()
                .init();
        } else {
            tracing_subscriber::fmt()
                .with_env_filter(EnvFilter::from_default_env())
                .with_span_events(FmtSpan::NEW)
                .with_ansi(use_color())
                .init();
        }
    });
}

fn use_color() -> bool {
    env::var("NO_COLOR").map(|v| v.is_empty()).unwrap_or(true)
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    init_logging();

    let r = Path::new(".");

    let hm = bfs(&r);

    println!("{hm:?}");

    // info!("Starting server");

    // let fsd = Fsd::new(3000, 3001);

    // fsd.go().await

    Ok(())
}
