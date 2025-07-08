use kube::CustomResource;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(CustomResource, Debug, Clone, Deserialize, Serialize, JsonSchema)]
#[kube(
    group = "shiftpod.io",
    version = "v1",
    kind = "ContainerMigration",
    plural = "containermigrations",
    status = "ContainerMigrationStatus",
    namespaced
)]
pub struct ContainerMigrationSpec {
    pub pod_template_hash: String,
    pub source_node: String,
    pub source_pod: String,
    pub target_node: Option<String>,
    pub target_pod: Option<String>,
    pub containers: Vec<ContainerMigrationContainer>,
}

#[derive(Debug, Clone, Deserialize, Serialize, JsonSchema)]
pub struct ContainerMigrationContainer {
    pub name: String,
    pub id: String,
    pub image_server: String,
    pub checkpoint_path: String,
}

#[derive(Debug, Clone, Deserialize, Serialize, JsonSchema)]
pub struct ContainerMigrationStatus {
    pub phase: MigrationPhase,
    pub message: Option<String>,
}

#[derive(Debug, Clone, Deserialize, Serialize, JsonSchema)]
pub enum MigrationPhase {
    Pending,
    Claimed,
    Migrating,
    Completed,
    Failed,
}
