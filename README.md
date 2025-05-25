# shiftpod

## Install

Install `devenv` to build this project.

```sh
devenv shell
task build
```

## k3s

1. Change `/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl` inserting the content of config.toml.tmpl.
2. Move `containerd-shim-shiftpod-v2` binary to `/usr/local/bin/`;
3. Apply `kubectl apply -f runtimeclass.yaml`;
4. Restart k3s `sudo systemctl restart k3s`;
5. Apply `kubectl apply -f testpod.yaml`;
6. Check `kubectl get pods` or `sudo journalctl -u k3s -f`.
