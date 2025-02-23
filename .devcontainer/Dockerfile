FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

# Install prerequisites as root
RUN apt-get update && apt-get install -y --no-install-recommends \
    unzip \
    curl \
    ca-certificates \
    zsh \
    git \
	make \
    sudo && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user 'dev' (with UID 1000 or other, if needed)
ARG USERNAME=dev
ARG USER_UID=1000
ARG USER_GID=$USER_UID
RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m -s /bin/zsh $USERNAME \
    && echo "$USERNAME ALL=(root) NOPASSWD:ALL" > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Create workspace directory and set permissions
RUN mkdir -p /workspaces/srvivor && chown -R $USER_UID:$USER_GID /workspaces

# Switch to the non-root user
USER $USERNAME
ENV HOME=/home/$USERNAME
WORKDIR /workspaces/srvivor

# Ensure the local bin directory exists and update PATH
RUN mkdir -p ${HOME}/.local/bin
ENV PATH="${HOME}/.local/bin:${PATH}"

# Install mise as the non-root user
RUN curl -fsSL https://mise.run | sh

# Copy configuration file with proper ownership
COPY .devcontainer/.default-go-packages $HOME
COPY mise.toml .

# Install mise dependencies
RUN mise trust -y
RUN mise install -y

CMD ["/bin/zsh"]

