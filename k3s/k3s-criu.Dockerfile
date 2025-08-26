FROM alpine:3.20 as build

ARG CC=gcc
ARG CRIU_VERSION=v4.1.1

# install alpine dependencies
RUN apk --update --no-cache add \
    $CC \
    bash \
    build-base \
    coreutils \
    procps \
    git \
    gnutls-dev \
    libaio-dev \
    libcap-dev \
    libnet-dev \
    libnl3-dev \
    nftables \
    nftables-dev \
    pkgconfig \
    protobuf-c-dev \
    protobuf-dev \
    py3-pip \
    py3-protobuf \
    python3 \
    sudo \
    libcap-utils \
    libdrm-dev \
    util-linux \
    util-linux-dev \
    tar

WORKDIR /workspace

# clone criu repo and checkout to selected version
RUN git clone https://github.com/checkpoint-restore/criu.git
WORKDIR /workspace/criu
RUN git checkout $CRIU_VERSION

# build criu
RUN make -j $(nproc) CC="$CC"
# copy criu libs dependencies
RUN mkdir criu-libs/ && \
    for l in $(ldd criu/criu | awk '{ print $3 }'); do cp $l criu-libs/; done

# extract tar binary and its dependencies
RUN cp $(which tar) /workspace/gnu-tar && \
    mkdir /workspace/gnu-tar-libs && \
    for l in $(ldd $(which tar) | awk '{ print $3 }' | grep -v '^$'); do cp $l /workspace/gnu-tar-libs/; done


# rancher k3s image
FROM rancher/k3s:v1.32.3-k3s1

# copy tar. needed because alpines uses busybox
COPY --from=build /workspace/gnu-tar /opt/bin/tar
COPY --from=build /workspace/gnu-tar-libs /lib/
ENV PATH="/opt/bin:$PATH"

# copy criu binaries and libs from build
COPY --from=build /workspace/criu/criu/criu /bin/
COPY --from=build /workspace/criu/criu-libs /lib/

# copy shiftpod shim and config files
COPY build/containerd-shim-shiftpod-v2 /usr/local/bin/
COPY k3s/config.toml.tmpl /var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl
COPY k3s/criu.conf /etc/criu/default.conf

ENTRYPOINT ["/bin/k3s"]
