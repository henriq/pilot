# DX

**Develop one microservice locally while it talks to the rest of your cluster.**

DX lets you work on a single service locally while it communicates with other services running in Kubernetes. Traffic routes to your machine automatically when your service is running, and falls back to the cluster when it's not. You get a built-in traffic inspector to see every request between services. Define your builds, deployments, and secrets in one configuration file.

## A Typical Workflow

You have 5 services in Kubernetes: `api`, `auth`, `users`, `payments`, and `notifications`.

```bash
# Deploy everything to your local Kubernetes cluster
dx install

# Start your api service locally (however you normally run it)
go run ./cmd/api  # or npm start, python app.py, etc.

# That's it. The auth, users, payments, and notifications services
# in Kubernetes now talk to your local api automatically.
```

Enable HTTP traffic inspection (optional):

```bash
dx install --intercept-http  # Password is printed after install
dx context info              # Shows the mitmweb URL
```

## How It Works

```
                          Kubernetes Cluster
┌───────────────────────────────────────────────────────────────────┐
│                                                                   │
│    ┌────────┐    ┌────────┐    ┌──────────┐    ┌────────┐         │
│    │  auth  │    │ users  │    │ payments │    │ notif. │         │
│    └───┬────┘    └───┬────┘    └────┬─────┘    └───┬────┘         │
│        │             │              │              │              │
│        └─────────────┴──────┬───────┴──────────────┘              │
│                             │                                     │
│                             ▼                                     │
│                    ┌─────────────────┐                            │
│                    │    dev-proxy    │                            │
│                    │                 │                            │
│                    │  • Health check │                            │
│                    │  • Route traffic│                            │
│                    │  • Capture HTTP │                            │
│                    └────────┬────────┘                            │
│                             │                                     │
└─────────────────────────────┼─────────────────────────────────────┘
                              │
                 Local healthy?
                    │       │
                   YES      NO
                    │       │
                    ▼       ▼
          ┌──────────────┐  ┌──────────────┐
          │ Your Machine │  │   Cluster    │
          │              │  │              │
          │  api :8080   │  │  api (pod)   │
          └──────────────┘  └──────────────┘
```

**How traffic flows:**

1. DX patches Kubernetes services to route through a dev-proxy
2. The proxy health-checks your local service
3. Healthy? Traffic goes to your machine. Down? Falls back to the cluster pod
4. With `--intercept-http`: all HTTP traffic is captured via mitmweb for inspection

**When the dev-proxy is rebuilt:**

1. It does not exist in the cluster yet
2. Your `localServices` configuration has changed (services added, removed, or modified)
3. The `--intercept-http` flag is set (always rebuilds to generate a fresh password)

Without `--intercept-http`, `dx install` and `dx update` skip the rebuild when the configuration is unchanged.

## Traffic Inspection

DX includes an optional traffic inspector (powered by mitmproxy) that captures every request between services: headers, bodies, timing, and more. Filter by service, path, or status code.

Enable it with the `--intercept-http` flag:

```bash
dx install --intercept-http
dx update --intercept-http
```

The mitmweb password is printed after each install. Run `dx context info` to see the inspector URL. Without `--intercept-http`, the dev-proxy at port 8001 shows a page explaining how to enable interception.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap henriq/dx
brew install dx
```

### Manual Installation

Download the latest release from the [releases page](https://github.com/henriq/dx/releases) and add it to your PATH.

## Prerequisites

- A local Kubernetes cluster (Docker Desktop, Rancher Desktop, minikube, kind)
- Docker client connected to the cluster's Docker daemon
- CLI tools: `kubectl`, `helm`, `git`, `docker`, `bash`

> **Note:** DX is designed for local development clusters where your Docker client can build images directly into the cluster. It is not intended for remote or production environments.

## Quick Start

```bash
# 1. Create a sample configuration
dx initialize

# 2. Edit ~/.dx-config.yaml to define your services
#    (see Configuration section below)

# 3. Build images and deploy everything
dx update

# 4. View your environment and monitoring URLs
dx context info
```

## Commands

### Build and Deploy

| Command | What it does |
|---------|--------------|
| `dx update [services...]` | Build images and deploy (most common) |
| `dx build [services...]` | Build Docker images only |
| `dx install [services...]` | Deploy to Kubernetes only |
| `dx uninstall [services...]` | Remove services from Kubernetes |

All commands support:
- **No arguments**: operates on the default profile
- **Service names**: operates on specific services (`dx update api auth`)
- **`-p, --profile`**: targets a profile (`dx install -p infra`)

### Manage Contexts

Contexts let you maintain separate configurations for different projects or environments:

```bash
dx context list              # Show all contexts
dx context set my-project    # Switch context
dx context info              # Show current status and URLs
dx context print             # Output context as JSON
```

### Manage Secrets

Secrets are encrypted with AES-GCM. Encryption keys are stored in your system keyring (macOS Keychain, Windows Credential Manager, or Linux Secret Service).

```bash
dx secret set DB_PASSWORD             # Set (prompts for value securely)
dx secret get DB_PASSWORD             # Retrieve
dx secret list                        # List all
dx secret delete DB_PASSWORD          # Remove
dx secret configure                   # Configure missing secrets interactively
dx secret configure --check           # Validate all secrets are configured
```

#### Discovering Required Secrets

DX can scan your configuration for secret references and prompt you for any missing values:

```bash
dx secret configure
```

This scans `{{.Secrets.KEY}}` references in:
- Scripts (`scripts` section)
- Helm arguments (`services[].helmArgs`)
- Docker build arguments (`services[].dockerImages[].buildArgs`)

Existing secrets are preserved. Press Enter to skip a secret during prompts.

Use `--check` to validate without prompting:

```bash
dx secret configure --check
# Exit code 0: all secrets configured
# Exit code 1: missing secrets (lists them)
```

### Utilities

```bash
dx run <script>       # Run a custom script defined in config
dx gen-env-key        # Generate cluster verification key
dx version            # Show version
```

## Configuration

DX uses `~/.dx-config.yaml`. Run `dx initialize` to create a starter file.

### Minimal Example

```yaml
contexts:
  - name: my-app
    services:
      - name: api
        helmRepoPath: /path/to/charts
        helmChartRelativePath: charts/api
        helmBranch: main
        dockerImages:
          - name: api:latest
            dockerfilePath: Dockerfile
            buildContextRelativePath: .
            gitRepoPath: /path/to/api
            gitRef: main
        profiles:
          - default

    localServices:
      - name: api
        localPort: 8080
        kubernetesPort: 80
        healthCheckPath: /health
        selector:               # Must match the K8s service you want to intercept
          app: api
```

### Services

Services define what DX builds and deploys:

```yaml
services:
  - name: api
    # Helm chart location
    helmRepoPath: /path/to/helm/repo
    helmChartRelativePath: charts/api
    helmBranch: main
    helmArgs:
      - --set=image.tag=latest
      - --set=replicas=1

    # Docker images to build
    dockerImages:
      - name: api:latest
        dockerfilePath: Dockerfile
        buildContextRelativePath: .
        gitRepoPath: /path/to/source
        gitRef: main
        buildArgs:
          - GO_VERSION=1.21

    # Images to pull (not build)
    remoteImages:
      - postgres:15
      - redis:7

    # Profile membership
    profiles:
      - default
      - backend
```

### Local Services

Local services define traffic routing to your machine:

```yaml
localServices:
  - name: api
    localPort: 8080           # Your local server port
    kubernetesPort: 80        # The K8s service port
    healthCheckPath: /health  # Health check endpoint
    selector:                 # Must match the K8s service selector
      app: api
```

When your local service responds to health checks on `localPort`, cluster traffic routes to your machine. Stop your local service, and traffic automatically falls back to Kubernetes.

### Profiles

Group services for targeted operations:

```yaml
services:
  - name: postgres
    profiles: [infra]
  - name: redis
    profiles: [infra]
  - name: api
    profiles: [default, backend]
  - name: worker
    profiles: [default, backend]
  - name: frontend
    profiles: [default]
```

```bash
dx install -p infra      # Deploy postgres and redis
dx update -p backend     # Build and deploy api and worker
dx update                # Default profile (api, worker, frontend)
dx uninstall -p all      # Remove everything
```

### Secrets in Helm Values

Reference encrypted secrets in your Helm arguments:

```yaml
helmArgs:
  - --set=database.password={{.Secrets.DB_PASSWORD}}
  - --set=api.key={{.Secrets.API_KEY}}
```

```bash
dx secret set DB_PASSWORD
dx secret set API_KEY
dx install api
```

### Custom Scripts

Define reusable commands for your workflow:

```yaml
scripts:
  reset-db: |
    kubectl delete pvc -l app=postgres
    dx uninstall postgres
    dx install postgres
  logs: kubectl logs -f deployment/api
  shell: kubectl exec -it deployment/api -- /bin/sh
```

```bash
dx run reset-db
dx run logs
```

Scripts run in Bash and require `bash` to be available in your PATH.

### Dockerfile Override

Test Dockerfile changes without modifying your repository:

```yaml
dockerImages:
  - name: api:latest
    dockerfileOverride: |
      FROM golang:1.21-alpine
      WORKDIR /app
      COPY . .
      RUN go build -o server .
      CMD ["./server"]
    buildContextRelativePath: .
    gitRepoPath: /path/to/source
    gitRef: main
```

### Configuration Sharing

Teams can share base configurations and override specific values:

```yaml
contexts:
  - name: my-context
    import: /shared/team-config.yaml
    services:
      - name: api
        dockerImages:
          - name: api
            gitRef: my-feature-branch  # Override the branch
```

## Common Workflows

### Iterative Development

```bash
# Make changes to your api service, then rebuild and redeploy
dx update api

# Check the traffic inspector if something isn't working
dx context info
```

### Working on Multiple Services

```bash
# Deploy infrastructure first
dx install -p infra

# Work on backend services
dx update -p backend

# Start both api and worker locally
./start-api.sh &
./start-worker.sh &

# Both receive cluster traffic now
```

### Switching Projects

```bash
dx context list
# my-app
# other-project
# legacy-system

dx context set other-project
dx update
```

## Shell Completion

Enable tab completion for commands, service names, and profiles:

```bash
# Bash
source <(dx completion bash)

# Zsh
source <(dx completion zsh)

# Fish
dx completion fish | source

# PowerShell
dx completion powershell | Out-String | Invoke-Expression
```

## File Storage

DX stores data in `~/.dx/`:
- Cloned repositories for Helm charts and Docker builds
- Encrypted secrets (per context)
- Generated dev-proxy configuration

Configuration lives at `~/.dx-config.yaml`.

## Uninstall

```bash
# Remove Kubernetes resources
dx uninstall -p all

# Delete local data
rm -rf ~/.dx ~/.dx-config.yaml

# Remove the binary (if installed manually)
rm $(which dx)

# Or via Homebrew
brew uninstall dx
```

## Troubleshooting

**Traffic not routing to my local service**
- Verify your service responds to health checks: `curl http://localhost:8080/health`
- Check that `localPort` matches where your service is running
- Ensure the `selector` matches your Kubernetes service

**Cannot connect to Kubernetes**
- Verify `kubectl` can reach your cluster: `kubectl get nodes`
- Ensure your Docker client connects to the cluster's Docker daemon

**Build failures**
- Check that the `gitRepoPath` exists and contains the expected files
- Verify `dockerfilePath` or `dockerfileOverride` is correct

**SSH authentication errors (password-protected keys)**

If you're using SSH URLs for git repositories (e.g., `git@github.com:user/repo.git`) and your SSH key has a passphrase, DX will fail with an "SSH authentication failed" error. This happens because DX runs git commands non-interactively.

To fix this, add your SSH key to the ssh-agent before running DX:

```bash
# Start ssh-agent if not already running
eval "$(ssh-agent -s)"

# Add your SSH key (will prompt for passphrase once)
ssh-add ~/.ssh/id_rsa

# Now DX commands will work
dx build
dx update
```

On macOS, you can add the key permanently to the keychain:
```bash
ssh-add --apple-use-keychain ~/.ssh/id_rsa
```

On Linux, you can configure your shell profile to start ssh-agent automatically.

**Need to debug further?**
- Check dev-proxy logs: `kubectl logs -l app=dev-proxy`
- Check service logs: `kubectl logs -l app=<service-name>`
