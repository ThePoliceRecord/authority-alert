ARG VARIANT="noble"
FROM buildpack-deps:${VARIANT}-curl

LABEL dev.containers.features="common"

ARG VARIANT
RUN if [ "$VARIANT" = "noble" ]; then \
        if id "ubuntu" &>/dev/null; then \
            echo "Deleting user 'ubuntu'  for $VARIANT" && userdel -f -r ubuntu || echo "Failed to delete ubuntu user for $VARIANT"; \  
        else \
            echo "User 'ubuntu' does not exist for $VARIANT"; \ 
        fi; \
    fi

RUN curl -fsSL https://deb.nodesource.com/setup_18.x | bash -

# Install Go 1.25.5 for supervisor build (supports RISC-V cross-compilation)
RUN curl -fsSL https://go.dev/dl/go1.25.5.linux-amd64.tar.gz | tar -C /usr/local -xzf - && \
    ln -sf /usr/local/go/bin/go /usr/local/bin/go && \
    ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
    build-essential \
    ninja-build \
    automake \
    autoconf \
    libtool \
    wget \
    curl \
    git \
    libssl-dev \
    bc \
    squashfs-tools \
    android-sdk-libsparse-utils \
    jq \
    cmake \
    tclsh \
    scons \
    parallel \
    ssh-client \
    tree \
    python3-dev \
    python3-pip \
    device-tree-compiler \
    libssl-dev \
    cpio \
    squashfs-tools \
    fakeroot \
    flex \
    bison \
    mtools \
    gcc \
    g++ \
    libbz2-dev \
    lib32c-dev \
    lib32stdc++6 \
    libpcre3-dev \
    unzip \
    ca-certificates \
    lsb-release \
    curl \
    gnupg \
    sudo \
    ca-certificates \
    nodejs \
    rsync \
    python-is-python3 \
    python3-jinja2 \
    python3-yaml \
    pkg-config \
    xxd \
    zip \
    dosfstools \
    fdisk \
 && ln -sf /usr/bin/python3 /usr/local/bin/python || true