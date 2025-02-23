FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

# Install prerequisites including ca-certificates, curl, zsh, git, etc.
RUN apt-get update && apt-get install -y --no-install-recommends \
    unzip \
    curl \
    ca-certificates \
    zsh \
    git && \
    rm -rf /var/lib/apt/lists/*

# Create the non-root user "dev" with UID/GID 1000 and a home directory
RUN useradd -m -u 2000 -s /bin/zsh dev

# Create workspaces directory and set permissions for "dev"
RUN mkdir -p /workspaces/srvivor && chown -R dev:dev /workspaces

# Switch to the "dev" user
USER dev
ENV HOME=/home/dev

# Ensure the local bin directory exists and update PATH so that it takes precedence
RUN mkdir -p ${HOME}/.local/bin
ENV PATH="${HOME}/.local/bin:${PATH}"

# Install mise as the non-root user; it will now install to /home/dev/.local/bin
RUN curl -fsSL https://mise.run | sh

WORKDIR /workspaces/srvivor

# Copy your configuration file and install mise dependencies
COPY mise.toml ./
RUN mise install

CMD ["/bin/zsh"]

