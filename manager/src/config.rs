use clap::Parser;
use tracing;

#[derive(Parser)]
#[command(name = "shiftpod-manager")]
#[command(about = "Shiftpod migration manager")]
pub struct Args {
    #[arg(long, env = "NODE_NAME", default_value = "localhost")]
    node_name: String,

    #[arg(long, env = "NODE_IP", default_value = "127.0.0.1")]
    node_address: String,

    #[arg(
        long,
        env = "CHECKPOINT_DIR",
        default_value = "/var/lib/shiftpod/checkpoints"
    )]
    checkpoint_dir: String,

    #[arg(
        long,
        env = "SOCKET_PATH",
        default_value = "/var/run/shiftpod/manager.sock"
    )]
    unix_socket: String,

    #[arg(long, env = "GRPC_PORT", default_value = "9090")]
    grpc_port: u16,

    #[arg(long, env = "RUST_LOG", default_value = "info")]
    log_level: String,
}

impl Args {
    pub fn node_name(&self) -> &str {
        &self.node_name
    }

    pub fn node_address(&self) -> &str {
        &self.node_address
    }

    pub fn checkpoint_dir(&self) -> &str {
        &self.checkpoint_dir
    }

    pub fn unix_socket(&self) -> &str {
        &self.unix_socket
    }

    pub fn grpc_port(&self) -> u16 {
        self.grpc_port
    }

    pub fn log_level(&self) -> tracing::Level {
        self.log_level
            .parse::<tracing::Level>()
            .expect("Invalid log level")
    }
}
