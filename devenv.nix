{ pkgs, lib, config, inputs, ... }:

{
  # https://devenv.sh/basics/
  env.PROJECT = "shiftpod";

  # https://devenv.sh/packages/
  packages = [
    pkgs.git
    pkgs.go-task
    pkgs.protobuf
    pkgs.protoc-gen-go
    pkgs.protoc-gen-go-grpc
    pkgs.delve
    pkgs.golangci-lint
    pkgs.gopls
    pkgs.kubectl
  ];

  # https://devenv.sh/languages/
  languages.go.enable = true;
  languages.go.enableHardeningWorkaround = true;
  languages.rust.enable = true;

  # https://devenv.sh/processes/
  # processes.cargo-watch.exec = "cargo-watch";

  # https://devenv.sh/services/
  # services.postgres.enable = true;

  # https://devenv.sh/scripts/
  scripts.hello.exec = ''
    echo Development environment loaded for $PROJECT
  '';

  enterShell = ''
    export SHELL=$(which zsh)
    exec $SHELL -i
    hello
    go version
    cargo --version
  '';

  # https://devenv.sh/tasks/
  # tasks = {
  #   "myproj:setup".exec = "mytool build";
  #   "devenv:enterShell".after = [ "myproj:setup" ];
  # };

  # https://devenv.sh/tests/
  enterTest = ''
    echo "Running tests"
    git --version | grep --color=auto "${pkgs.git.version}"
  '';

  # https://devenv.sh/git-hooks/
  git-hooks.hooks.govet = {
    enable = true;
    pass_filenames = false;
  };

  scripts.pre-commit.exec = ''
    echo "Running go fmt..."
    go fmt ./...

    echo "Running golangci-lint..."
    golangci-lint run
  '';

  # See full reference at https://devenv.sh/reference/options/
}
