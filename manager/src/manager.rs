use anyhow::Result;
use kube::{Api, Client, ResourceExt};
use std::pin::Pin;
use tokio_stream::wrappers::ReceiverStream;
use tonic::{Request, Response, Status};
use tracing::{error, info};

use crate::crd::{
    ContainerMigration, ContainerMigrationSpec, ContainerMigrationStatus, MigrationPhase,
};
use crate::proto::manager_service_server::ManagerService;
use crate::proto::*;

#[derive(Clone)]
pub struct ShiftpodManager {
    migrations_api: Api<ContainerMigration>,
    node_info: NodeInfo,
    checkpoint_dir: String,
}

#[derive(Debug, Clone)]
pub struct NodeInfo {
    pub name: String,
    pub address: String,
}

impl ShiftpodManager {
    pub async fn new(node_name: &str, node_address: &str, checkpoint_dir: &str) -> Result<Self> {
        let k8s_client = Client::try_default().await?;
        let migrations_api: Api<ContainerMigration> = Api::all(k8s_client.clone());

        Ok(Self {
            migrations_api,
            node_info: NodeInfo {
                name: node_name.to_string(),
                address: node_address.to_string(),
            },
            checkpoint_dir: checkpoint_dir.to_string(),
        })
    }
}

#[tonic::async_trait]
impl ManagerService for ShiftpodManager {
    type PullImageStream =
        Pin<Box<dyn tokio_stream::Stream<Item = Result<PullImageResponse, Status>> + Send>>;

    async fn notify_checkpoint(
        &self,
        request: Request<NotifyCheckpointRequest>,
    ) -> Result<Response<NotifyCheckpointResponse>, Status> {
        let req = request.into_inner();
        info!(
            "Received checkpoint notification for container: {}",
            req.container_id
        );

        let pod_info = req
            .pod_info
            .ok_or_else(|| Status::invalid_argument("Missing pod info"))?;

        // Create Migration CRD
        let migration_name = format!("migration-{}-{}", pod_info.template_hash, req.container_id);

        let migration = ContainerMigration::new(
            &migration_name,
            ContainerMigrationSpec {
                pod_template_hash: pod_info.template_hash,
                source_node: self.node_info.name.clone(),
                source_pod: pod_info.name,
                target_node: None,
                target_pod: None,
                containers: vec![crate::crd::ContainerMigrationContainer {
                    name: pod_info.container_name,
                    id: req.container_id,
                    image_server: self.node_info.address.clone(),
                    checkpoint_path: req.checkpoint_path,
                }],
            },
        );

        match self
            .migrations_api
            .create(&Default::default(), &migration)
            .await
        {
            Ok(_) => {
                info!("Created migration CRD: {}", migration_name);
                Ok(Response::new(NotifyCheckpointResponse {}))
            }
            Err(e) => {
                error!("Failed to create migration CRD: {}", e);
                Err(Status::internal("Failed to create migration CRD"))
            }
        }
    }

    async fn request_migration_restore(
        &self,
        request: Request<MigrationRestoreRequest>,
    ) -> Result<Response<MigrationRestoreResponse>, Status> {
        let req = request.into_inner();
        info!(
            "Received migration restore request for pod template hash: {}",
            req.pod_template_hash
        );

        // Find unclaimed migration CRD
        let migrations = self
            .migrations_api
            .list(&Default::default())
            .await
            .map_err(|e| Status::internal(format!("Failed to list migrations: {}", e)))?;

        let target_migration = migrations.items.into_iter().find(|m| {
            m.spec.pod_template_hash == req.pod_template_hash && m.spec.target_node.is_none()
        });

        let Some(mut migration) = target_migration else {
            return Ok(Response::new(MigrationRestoreResponse {
                found: false,
                checkpoint_path: String::new(),
            }));
        };

        // Claim the migration
        migration.spec.target_node = Some(self.node_info.name.clone());
        migration.spec.target_pod = Some(req.pod_name);
        migration.status = Some(ContainerMigrationStatus {
            phase: MigrationPhase::Claimed,
            message: None,
        });

        // Update CRD
        self.migrations_api
            .replace(&migration.name_any(), &Default::default(), &migration)
            .await
            .map_err(|e| Status::internal(format!("Failed to update migration: {}", e)))?;

        // Download checkpoint
        let container = &migration.spec.containers[0];
        let local_path = format!("{}/{}", self.checkpoint_dir, container.id);

        match self
            .download_checkpoint(
                &container.image_server,
                &container.checkpoint_path,
                &local_path,
            )
            .await
        {
            Ok(_) => {
                info!("Successfully downloaded checkpoint to: {}", local_path);

                // Update status to migrating
                migration.status = Some(ContainerMigrationStatus {
                    phase: MigrationPhase::Migrating,
                    message: Some("Checkpoint downloaded successfully".to_string()),
                });

                let _ = self
                    .migrations_api
                    .replace(&migration.name_any(), &Default::default(), &migration)
                    .await;

                Ok(Response::new(MigrationRestoreResponse {
                    found: true,
                    checkpoint_path: local_path,
                }))
            }
            Err(e) => {
                error!("Failed to download checkpoint: {}", e);
                Err(Status::internal("Failed to download checkpoint"))
            }
        }
    }

    async fn pull_image(
        &self,
        request: Request<PullImageRequest>,
    ) -> Result<Response<Self::PullImageStream>, Status> {
        let req = request.into_inner();
        info!("Received pull image request for: {}", req.checkpoint_path);

        let (tx, rx) = tokio::sync::mpsc::channel(4);
        let checkpoint_path = req.checkpoint_path.clone();

        tokio::spawn(async move {
            if let Err(e) = Self::stream_checkpoint_file(&checkpoint_path, tx).await {
                error!("Failed to stream checkpoint file: {}", e);
            }
        });

        let output_stream = ReceiverStream::new(rx);
        Ok(Response::new(Box::pin(output_stream)))
    }

    async fn finish_restore(
        &self,
        request: Request<FinishRestoreRequest>,
    ) -> Result<Response<FinishRestoreResponse>, Status> {
        let req = request.into_inner();
        info!(
            "Received finish restore request for container: {}",
            req.container_id
        );

        // Find and update migration status
        let migrations = self
            .migrations_api
            .list(&Default::default())
            .await
            .map_err(|e| Status::internal(format!("Failed to list migrations: {}", e)))?;

        if let Some(mut migration) = migrations
            .items
            .into_iter()
            .find(|m| m.spec.containers.iter().any(|c| c.id == req.container_id))
        {
            migration.status = Some(ContainerMigrationStatus {
                phase: if req.success {
                    MigrationPhase::Completed
                } else {
                    MigrationPhase::Failed
                },
                message: if req.success {
                    Some("Restore completed successfully".to_string())
                } else {
                    Some("Restore failed".to_string())
                },
            });

            let _ = self
                .migrations_api
                .replace(&migration.name_any(), &Default::default(), &migration)
                .await;
        }

        Ok(Response::new(FinishRestoreResponse {}))
    }
}

impl ShiftpodManager {
    async fn download_checkpoint(
        &self,
        source_node: &str,
        remote_path: &str,
        local_path: &str,
    ) -> Result<()> {
        let mut client = crate::proto::manager_service_client::ManagerServiceClient::connect(
            format!("http://{}:9090", source_node),
        )
        .await?;

        let request = tonic::Request::new(PullImageRequest {
            checkpoint_path: remote_path.to_string(),
        });

        let mut stream = client.pull_image(request).await?.into_inner();

        let mut file = tokio::fs::File::create(local_path).await?;

        while let Some(response) = stream.message().await? {
            use tokio::io::AsyncWriteExt;
            file.write_all(&response.chunk).await?;
        }

        Ok(())
    }

    async fn stream_checkpoint_file(
        checkpoint_path: &str,
        tx: tokio::sync::mpsc::Sender<Result<PullImageResponse, Status>>,
    ) -> Result<()> {
        use tokio::io::AsyncReadExt;

        let mut file = tokio::fs::File::open(checkpoint_path).await?;
        let mut buffer = [0; 8192];

        loop {
            let bytes_read = file.read(&mut buffer).await?;
            if bytes_read == 0 {
                break;
            }

            let response = PullImageResponse {
                chunk: buffer[..bytes_read].to_vec(),
            };

            if tx.send(Ok(response)).await.is_err() {
                break;
            }
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;
    use tokio::fs;

    #[tokio::test]
    async fn test_shiftpod_manager_new() {
        let temp_dir = TempDir::new().unwrap();
        let checkpoint_dir = temp_dir.path().to_str().unwrap();

        // This might fail if no k8s cluster is available, but let's test the structure
        let result = ShiftpodManager::new(
            "test-node".as_ref(),
            "127.0.0.1".as_ref(),
            checkpoint_dir.as_ref(),
        )
        .await;

        match result {
            Ok(manager) => {
                assert_eq!(manager.node_info.name, "test-node");
                assert_eq!(manager.node_info.address, "127.0.0.1");
                assert_eq!(manager.checkpoint_dir, checkpoint_dir);
            }
            Err(_) => {
                println!("Cluster not available for testing");
            }
        }
    }

    #[tokio::test]
    async fn test_stream_checkpoint_file() {
        let temp_dir = TempDir::new().unwrap();
        let checkpoint_path = temp_dir.path().join("test_checkpoint");

        // checkpoint test file
        let test_data = b"test checkpoint data";
        fs::write(&checkpoint_path, test_data).await.unwrap();

        let (tx, mut rx) = tokio::sync::mpsc::channel(4);

        // test streaming
        let checkpoint_path_str = checkpoint_path.to_str().unwrap();
        let result = ShiftpodManager::stream_checkpoint_file(checkpoint_path_str, tx).await;

        assert!(result.is_ok());

        // collect streamed data
        let mut collected_data = Vec::new();
        while let Some(response) = rx.recv().await {
            match response {
                Ok(pull_response) => {
                    collected_data.extend(pull_response.chunk);
                }
                Err(_) => break,
            }
        }

        assert_eq!(collected_data, test_data);
    }

    #[tokio::test]
    async fn test_stream_checkpoint_file_not_found() {
        let (tx, _rx) = tokio::sync::mpsc::channel(4);

        let result = ShiftpodManager::stream_checkpoint_file("/nonexistent/path", tx).await;
        assert!(result.is_err());
    }
}
