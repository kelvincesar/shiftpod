# shiftpod

## Install

Install `devenv` to build this project.

```sh
devenv shell
task build
```

## Execute

- Create K3D cluster with `task k3d`
- Build shim image and criu files with `task k3d:build`
- Build counter example with `task counter:build`
- Open terminal on server node to check logs with `task k3d:terminal`
- `cd /tmp/shiftpod` and `tail -f shim.log`;
- On another terminal, `task counter:delete`
