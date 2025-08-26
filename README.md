# shiftpod

Shiftpod is a distributed container checkpoint/restore system that enables seamless pod migration between Kubernetes nodes. It consists of a Go containerd shim that intercepts container lifecycle events and a Rust manager service that coordinates checkpoints across nodes using Kubernetes Custom Resource Definitions (CRDs).

## Installation

### Prerequisites

Install `devenv` to build this project:

```sh
devenv shell
```

## Execute

- Create K3D cluster with `task k3d`
- Build shim image and criu files with `task k3d:build`
- Build counter example with `task counter:build`
- Open terminal on server node to check logs with `task k3d:terminal`
- `cd /tmp/shiftpod` and `tail -f shim.log`;
- On another terminal, `task counter:delete`
