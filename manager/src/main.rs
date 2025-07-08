use anyhow::Result;
use clap::Parser;

use tonic::transport::Server;
use tracing::{error, info};

mod config;
mod crd;
mod manager;
mod proto {
    tonic::include_proto!("shiftpod.manager.v1");
}

use config::Args;

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();

    // use that subscriber to process traces emitted after this point
    let subscriber = tracing_subscriber::fmt()
        .with_max_level(args.log_level())
        .finish();

    let _ = tracing::subscriber::set_global_default(subscriber);

    info!("Starting Shiftpod Manager");
    info!("Node: {}", args.node_name());
    info!("Address: {}", args.node_address());
    info!("Checkpoint dir: {}", args.checkpoint_dir());

    // ensure checkpoint directory exists
    tokio::fs::create_dir_all(args.checkpoint_dir()).await?;

    // create manager instance
    let manager =
        manager::ShiftpodManager::new(args.node_name(), args.node_address(), args.checkpoint_dir())
            .await?;

    // Unix socket server for shim communication
    let unix_manager = manager.clone();
    let unix_socket_path = args.unix_socket().to_string();
    tokio::spawn(async move {
        if let Err(e) = start_unix_socket_server(unix_manager, &unix_socket_path).await {
            error!("Unix socket server failed: {}", e);
        }
    });

    // gRPC server for node-to-node communication
    let addr = format!("0.0.0.0:{}", args.grpc_port()).parse()?;
    info!("Starting gRPC server on {}", addr);

    Server::builder()
        .add_service(proto::manager_service_server::ManagerServiceServer::new(
            manager,
        ))
        .serve(addr)
        .await?;

    Ok(())
}

async fn start_unix_socket_server(
    manager: manager::ShiftpodManager,
    socket_path: &str,
) -> Result<()> {
    use tokio::net::UnixListener;
    use tokio_stream::wrappers::UnixListenerStream;

    // clean existing socket file
    let _ = tokio::fs::remove_file(&socket_path).await;

    let listener = UnixListener::bind(&socket_path)?;
    info!("Unix socket server listening on {}", socket_path);

    // apply gRPC service over Unix socket
    let incoming = UnixListenerStream::new(listener);

    Server::builder()
        .add_service(proto::manager_service_server::ManagerServiceServer::new(
            manager,
        ))
        .serve_with_incoming(incoming)
        .await?;

    Ok(())
}
