# Shiftpod Manager

The Shiftpod Manager is a component that handles container migration orchestration in Kubernetes clusters. It manages Custom Resource Definitions (CRDs) for container migrations and facilitates checkpoint transfers between nodes.

## Overview

The manager runs as a DaemonSet on each node and provides:

- **gRPC API**: node-to-node communication (port 9090);
- **Unix Socket**: communication with shims;
- **CRD management**: `ContainerMigration` resources;

## Architecture

```
┌─────────────────┐    ┌─────────────────┐
│   Node A        │    │   Node B        │
│                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │ Shim        │ │    │ │ Shim        │ │
│ │             │ │    │ │             │ │
│ └──────┬──────┘ │    │ └──────┬──────┘ │
│        │ Unix   │    │        │ Unix   │
│        │ Socket │    │        │ Socket │
│        │        │    │        │        │
│ ┌──────▼──────┐ │    │ ┌──────▼──────┐ │
│ │ Manager     │◄┼────┼─> Manager     │ |
│ │             │ │gRPC│ |             │ │
│ └─────────────┘ │    │ └─────────────┘ │
└─────────────────┘    └─────────────────┘
         │                       │
         └───────────────────────|
                                 │
                    ┌─────────────▼────────────┐
                    │    Kubernetes API        │
                    │  (ContainerMigration     │
                    │       CRDs)              │
                    └──────────────────────────┘
```

## Usage

coming soon...
