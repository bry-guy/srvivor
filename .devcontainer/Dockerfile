ARG USERNAME=dev

FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

# Create the non-root user with UID/GID 1000 and a home directory
RUN useradd -m -u 2000 -s /bin/zsh $USERNAME

# Create workspaces directory and set permissions 
RUN mkdir -p /workspaces/srvivor && chown -R 2000:2000 /workspaces

# Switch to the user
USER $USERNAME
ENV HOME=/home/$USERNAME

# Install prerequisites including ca-certificates, curl, zsh, git, etc.
RUN apt-get update && apt-get install -y --no-install-recommends \
    sudo \
    unzip \
    curl \
    ca-certificates \
    zsh \
    git && \
    rm -rf /var/lib/apt/lists/* && \
	echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Ensure the local bin directory exists and update PATH so that it takes precedence
RUN mkdir -p ${HOME}/.local/bin
ENV PATH="${HOME}/.local/bin:${PATH}"

# Install mise as the non-root user
RUN curl -fsSL https://mise.run | sh

WORKDIR /workspaces/srvivor

# Copy your configuration file and install mise dependencies
COPY mise.toml ./
RUN mise trust
RUN mise install

CMD ["/bin/zsh"]

