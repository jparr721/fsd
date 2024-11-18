use std::process::Termination;

use anyhow::{bail, Result};

use axum::Router;
use futures::{future::join_all, TryFutureExt};
use tokio::{
    signal::{self, unix::signal},
    time::timeout,
};
use tracing::{error, info, warn};

use crate::api::rest::{define_api, define_logging_layer};

use super::indexer::Indexer;

#[derive(Clone, Debug)]
pub struct ShutdownToken(());
impl ShutdownToken {
    fn new() -> Self {
        Self(())
    }
}

impl Termination for ShutdownToken {
    fn report(self) -> std::process::ExitCode {
        std::process::ExitCode::SUCCESS
    }
}

pub struct Fsd {
    api_port: u16,
    indexer_port: u16,
    shutdown_tx: async_broadcast::Sender<ShutdownToken>,
    shutdown_rx: async_broadcast::Receiver<ShutdownToken>,
}

async fn shutdown_signal(mut rx: async_broadcast::Receiver<ShutdownToken>) {
    let _ = rx.recv().await;
    warn!("shutdown token received, exiting");
}

impl Fsd {
    pub fn new(api_port: u16, indexer_port: u16) -> Self {
        let (tx, rx) = async_broadcast::broadcast(1);

        Self {
            api_port,
            indexer_port,
            shutdown_tx: tx,
            shutdown_rx: rx,
        }
    }

    pub async fn go(self) -> Result<()> {
        let rx = self.shutdown_rx.clone();
        let mut api_handle = tokio::spawn(async move {
            let router = define_logging_layer(define_api(Router::new()));
            let bind = "0.0.0.0:3000";
            let listener = tokio::net::TcpListener::bind(bind)
                .await
                .expect("failed to bind api");
            info!(%bind, "api running");
            axum::serve(listener, router)
                .with_graceful_shutdown(shutdown_signal(rx))
                .await
                .unwrap();
            Ok(())
        });

        let rx = self.shutdown_rx.clone();
        let mut indexer_handle = tokio::spawn(async move {
            let indexer = Indexer::new(rx);
            indexer.go().await
        });

        tokio::select! { biased;
            _ = tokio::signal::ctrl_c() => {
                warn!("termination signal received, exiting");
                self.shutdown_tx.broadcast(ShutdownToken::new()).await.expect("failed to broadcast shutdown token");

                // Wait for the system to shutdown gracefully
                if let Err(_) = timeout(std::time::Duration::from_secs(4), async {
                    let _ = join_all([api_handle, indexer_handle]);
                })
                .await
                {
                    bail!("failed to shutdown in time, force exiting");
                }

                Ok(())
            }
            _ = &mut api_handle => {
                bail!("api shutdown unexpectedly");
            }
            _ = &mut indexer_handle => {
                bail!("indexer shutdown unexpectedly");
            }
        }
    }
}
