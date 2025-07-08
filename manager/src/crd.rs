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

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json;

    #[test]
    fn test_migration_phase_serialization() {
        let phase = MigrationPhase::Pending;
        let json = serde_json::to_string(&phase).unwrap();
        assert_eq!(json, "\"Pending\"");

        let phase = MigrationPhase::Completed;
        let json = serde_json::to_string(&phase).unwrap();
        assert_eq!(json, "\"Completed\"");
    }

    #[test]
    fn test_migration_phase_deserialization() {
        let json = "\"Pending\"";
        let phase: MigrationPhase = serde_json::from_str(json).unwrap();
        assert!(matches!(phase, MigrationPhase::Pending));

        let json = "\"Failed\"";
        let phase: MigrationPhase = serde_json::from_str(json).unwrap();
        assert!(matches!(phase, MigrationPhase::Failed));
    }

    #[test]
    fn test_container_migration_container_creation() {
        let container = ContainerMigrationContainer {
            name: "test-container".to_string(),
            id: "container-123".to_string(),
            image_server: "192.168.1.100".to_string(),
            checkpoint_path: "/tmp/checkpoint".to_string(),
        };

        assert_eq!(container.name, "test-container");
        assert_eq!(container.id, "container-123");
        assert_eq!(container.image_server, "192.168.1.100");
        assert_eq!(container.checkpoint_path, "/tmp/checkpoint");
    }

    #[test]
    fn test_container_migration_status_serialization() {
        let status = ContainerMigrationStatus {
            phase: MigrationPhase::Migrating,
            message: Some("Migration in progress".to_string()),
        };

        let json = serde_json::to_string(&status).unwrap();
        assert!(json.contains("\"phase\":\"Migrating\""));
        assert!(json.contains("\"message\":\"Migration in progress\""));
    }

    #[test]
    fn test_container_migration_spec_creation() {
        let spec = ContainerMigrationSpec {
            pod_template_hash: "hash-123".to_string(),
            source_node: "node-1".to_string(),
            source_pod: "pod-1".to_string(),
            target_node: None,
            target_pod: None,
            containers: vec![ContainerMigrationContainer {
                name: "nginx".to_string(),
                id: "container-456".to_string(),
                image_server: "10.0.0.1".to_string(),
                checkpoint_path: "/var/lib/checkpoints/456".to_string(),
            }],
        };

        assert_eq!(spec.pod_template_hash, "hash-123");
        assert_eq!(spec.source_node, "node-1");
        assert_eq!(spec.source_pod, "pod-1");
        assert!(spec.target_node.is_none());
        assert!(spec.target_pod.is_none());
        assert_eq!(spec.containers.len(), 1);
        assert_eq!(spec.containers[0].name, "nginx");
    }

    #[test]
    fn test_container_migration_spec_with_target() {
        let mut spec = ContainerMigrationSpec {
            pod_template_hash: "hash-789".to_string(),
            source_node: "node-source".to_string(),
            source_pod: "pod-source".to_string(),
            target_node: Some("node-target".to_string()),
            target_pod: Some("pod-target".to_string()),
            containers: vec![],
        };

        assert_eq!(spec.target_node, Some("node-target".to_string()));
        assert_eq!(spec.target_pod, Some("pod-target".to_string()));

        // Test claiming migration
        spec.target_node = Some("new-target".to_string());
        assert_eq!(spec.target_node, Some("new-target".to_string()));
    }

    #[test]
    fn test_json_schema_generation() {
        use schemars::schema_for;

        let schema = schema_for!(ContainerMigrationSpec);
        assert!(schema.schema.object.is_some());

        let schema = schema_for!(MigrationPhase);
        assert!(schema.schema.subschemas.is_some());
    }
}
