# Image Copier

A tool to help developers in China pull Docker images from foreign registries that may be blocked or slow to access directly.

The tool works by leveraging GitHub Actions to pull the image on a server outside of China, then pushing it to a domestic registry where it can be pulled locally at high speed.

## Features

- Pull individual Docker images via GitHub Actions relay
- Batch processing of multiple images
- Command-line interface with subcommands
- Configurable settings via YAML file
- Concurrent processing for improved performance
- Detailed logging and progress tracking

## Installation

### Pre-requisites

Before installing and using Image Copier, ensure you have the following tools installed:

1. Docker - For pulling and managing images
2. Skopeo - For copying images between registries
3. Go (1.19+) - Only required if building from source

Install these dependencies with:

```bash
# On macOS with Homebrew
brew install docker skopeo

# On Ubuntu/Debian
sudo apt-get update
sudo apt-get install docker.io skopeo
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/your-github-username/image-copier.git
cd image-copier

# Build the binary
go build -o image-copier ./cmd/image-copier

# Optionally move to a location in your PATH
sudo mv image-copier /usr/local/bin/
```

## Configuration

Image Copier requires configuration to work properly. You need to provide credentials for both GitHub and your domestic container registry.

### Creating a Configuration File

Generate a sample configuration file:

```bash
./image-copier config init
```

This creates a `config.yaml` file in your current directory with placeholders for all required settings.

### Required Configuration Values

Edit the generated `config.yaml` file with your actual values:

| Setting | Description | Environment Variable |
|---------|-------------|---------------------|
| `github.owner` | Your GitHub username or organization | GITHUB_OWNER |
| `github.repo` | Repository name where this tool is hosted | GITHUB_REPO |
| `github.token` | Personal Access Token with workflow permissions | GITHUB_TOKEN |
| `registry.host` | Domestic registry hostname | REGISTRY_HOST |
| `registry.username` | Registry username | REGISTRY_USERNAME |
| `registry.password` | Registry password or token | REGISTRY_PASSWD |

You can also configure these values using environment variables instead of a config file.

### GitHub Personal Access Token

To generate a GitHub Personal Access Token:

1. Visit https://github.com/settings/tokens
2. Click "Generate new token"
3. Give the token a descriptive name
4. Select the `repo` scope and `workflow` scope
5. Click "Generate token"
6. Copy the token and use it in your configuration

## Usage

### Pull a Single Image

```bash
./image-copier pull nginx:latest
```

### Pull Multiple Images

#### From Command Line Arguments

```bash
./image-copier batch nginx:latest redis:7-alpine postgres:15
```

#### From a File

Create a text file with one image per line:

```txt
# images.txt
nginx:latest
redis:7-alpine
postgres:15
```

Then run:

```bash
./image-copier batch -f images.txt
```

### View Current Configuration

```bash
./image-copier config show
```

### Generate Sample Configuration

```bash
./image-copier config init
```

## How It Works

1. When you request to pull an image, the tool first checks if it already exists in your domestic registry
2. If not present, it triggers a GitHub Action workflow in your fork of this repository
3. The workflow uses Skopeo to copy the image from the foreign registry to your domestic registry
4. Once complete, the tool copies the image from your domestic registry to your local Docker daemon

## GitHub Actions Setup

Make sure your fork has the proper GitHub Actions workflow configured. The default workflow file is `.github/workflows/image-copier-v2.yaml`.

Ensure that your repository secrets contain:
- `DEST_CREDS`: Credentials for your domestic registry in the format `username:password`

## Troubleshooting

If you encounter issues:

1. Verify all configuration values are correct
2. Ensure your GitHub token has the necessary permissions
3. Check that your domestic registry credentials are valid
4. Confirm Docker and Skopeo are properly installed and accessible

For detailed logs, increase the log level in your configuration:

```yaml
log_level: debug
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.