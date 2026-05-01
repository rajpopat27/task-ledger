# task-ledger Development Container

This devcontainer configuration provides a development environment for task-ledger with:

- Go 1.25.8 development environment
- tl CLI built and installed from source
- All dependencies pre-installed

## Quick Start

### GitHub Codespaces

1. Click the "Code" button on GitHub
2. Select "Create codespace on main"
3. Wait for the container to build (~2-3 minutes)
4. The environment will be ready with tl installed and configured

### VS Code Remote Containers

1. Install the [Remote - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension
2. Open the task-ledger repository in VS Code
3. Click "Reopen in Container" when prompted (or use Command Palette: "Remote-Containers: Reopen in Container")
4. Wait for the container to build

## What Gets Installed

The `setup.sh` script automatically:

1. Builds tl from source (`go build ./cmd/tl`)
2. Installs tl to `/usr/local/bin/tl`
3. Runs `tl init --quiet` (non-interactive initialization)
4. Downloads Go module dependencies

## Verification

After the container starts, verify everything works:

```bash
# Check tl is installed
tl --version

# Check for ready tasks
tl ready

# View project counts
tl count --by-status
```

## Git Configuration

Your local `.gitconfig` is mounted into the container so your git identity is preserved. If you need to configure git:

```bash
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"
```

## Troubleshooting

**tl command not found:**
- The setup script should install tl automatically
- Manually run: `bash .devcontainer/setup.sh`

**Container fails to build:**
- Check the container logs for specific errors
- Ensure Docker/Podman is running and has sufficient resources
- Try rebuilding: Command Palette → "Remote-Containers: Rebuild Container"
