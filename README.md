# Pilot

Pilot automates builds, deployments, and local development workflows for Kubernetes-hosted services. Services are declaratively configured in YAML — Pilot handles Docker builds, Helm deployments, traffic routing, TLS certificates, encrypted secrets, and HTTP interception. Teams can share a base configuration so that every developer gets a consistent environment.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap henriq/pilot
brew install pilot
```

### Manual Installation

Download the latest release from the [releases page](https://github.com/henriq/pilot/releases) and add it to your PATH.

## Prerequisites

- A local Kubernetes cluster (Docker Desktop, Rancher Desktop, minikube, kind)
- Docker client connected to the cluster's Docker daemon
- CLI tools: `kubectl`, `helm`, `git`, `docker`, `bash`

> **Note:** Pilot is designed for local development clusters where your Docker client can build images directly into the cluster. It is not intended for remote or production environments.

## Quick Start

```bash
# 1. Create a sample configuration
pilot initialize

# 2. Edit ~/.pilot-config.yaml to define your services
#    (see Configuration section below)

# 3. Build images and deploy everything
pilot update

# 4. View your environment and monitoring URLs
pilot context info
```

## Commands

### Build and Deploy

| Command                         | What it does                          |
|---------------------------------|---------------------------------------|
| `pilot update [services...]`    | Build images and deploy (most common) |
| `pilot build [services...]`     | Build Docker images only              |
| `pilot install [services...]`   | Deploy to Kubernetes only             |
| `pilot uninstall [services...]` | Remove services from Kubernetes       |

All commands support:
- **No arguments**: operates on the default profile
- **Service names**: operates on specific services (`pilot update api auth`)
- **`-p, --profile`**: targets a profile (`pilot install -p infra`)

### Manage Contexts

Contexts let you maintain separate configurations for different projects or environments:

```bash
pilot context list              # Show all contexts
pilot context set my-project    # Switch context
pilot context info              # Show current status and URLs
pilot context print             # Output context as JSON
```

### Manage Secrets

Secrets are encrypted with AES-GCM. Encryption keys are stored in your system keyring (macOS Keychain, Windows Credential Manager, or Linux Secret Service).

```bash
pilot secret set DB_PASSWORD             # Set (prompts for value securely)
pilot secret get DB_PASSWORD             # Retrieve
pilot secret list                        # List all
pilot secret delete DB_PASSWORD          # Remove
pilot secret configure                   # Configure missing secrets interactively
pilot secret configure --check           # Validate all secrets are configured
```

#### Discovering Required Secrets

Pilot can scan your configuration for secret references and prompt you for any missing values:

```bash
pilot secret configure
```

This scans `{{.Secrets.KEY}}` references in:
- Scripts (`scripts` section)
- Helm arguments (`services[].helmArgs`)
- Docker build arguments (`services[].dockerImages[].buildArgs`)

Existing secrets are preserved. Press Enter to skip a secret during prompts.

Use `--check` to validate without prompting:

```bash
pilot secret configure --check
# Exit code 0: all secrets configured
# Exit code 1: missing secrets (lists them)
```

### Manage Certificates

Pilot can automatically issue TLS certificates for your services using a private certificate authority (CA) managed per context. Certificates are provisioned as Kubernetes secrets before services are installed.

```bash
pilot ca status                  # Show CA status and expiry
pilot ca print                   # Print CA certificate in PEM format
pilot ca print > ca.crt          # Save CA certificate to a file
pilot ca delete                  # Delete CA (new one created on next pilot install)
pilot ca delete --yes            # Skip confirmation prompt
```

The CA is created automatically on the first `pilot install`. Leaf certificates have a 30-day validity period and are always freshly issued during `pilot install`.

To trust the certificates locally, extract the CA certificate and add it to your system's trust store:

```bash
pilot ca print > ca.crt
```

### Utilities

```bash
pilot run <script>       # Run a custom script defined in config
pilot gen-env-key        # Generate cluster verification key
pilot version            # Show version
```

### Traffic Interception

Pilot includes optional HTTP traffic interception powered by [mitmproxy](https://mitmproxy.org/). When enabled, requests between services are captured — headers, bodies, timing — and displayed in a web UI where you can filter, inspect, and replay them.

```bash
# Enable traffic interception during install
pilot install --intercept-http

# Or during update
pilot update --intercept-http
```

After install, Pilot prints the mitmweb URL and a randomly generated password:

```
-> mitmweb: https://dev-proxy.my-app.localhost
   -> password: a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

Run `pilot context info` at any time to see the mitmweb and HAProxy stats URLs. Without `--intercept-http`, the dev-proxy routes traffic directly and the mitmweb URL shows a page explaining how to enable interception.

### Local Services

Local services let you route cluster traffic to a service running on your machine. Pilot deploys a dev-proxy into Kubernetes that health-checks your local service every 5 seconds. When it responds, traffic flows to your machine. When it stops, traffic automatically falls back to the Kubernetes pod — no manual intervention needed.

```bash
# 1. Deploy services (creates the dev-proxy)
pilot install

# 2. Start your service locally
go run ./cmd/api   # or npm start, python app.py, etc.

# 3. Cluster traffic now routes to your machine
# 4. Stop your service — traffic falls back to the cluster automatically
```

Run `pilot context info` to see ingress URLs and health check status for each local service.


## Configuration

Pilot uses `~/.pilot-config.yaml`. Run `pilot initialize` to create a starter file.

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

Services define what Pilot builds and deploys:

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

### Certificates

Services can declare TLS certificates that Pilot provisions as Kubernetes secrets:

```yaml
services:
  - name: api
    # ... helm and docker config ...
    certificates:
      # Standard TLS secret (kubernetes.io/tls)
      - type: server
        dnsNames:
          - api.localhost
          - "*.api.localhost"
        k8sSecret:
          name: api-tls
          type: tls

      # Opaque secret with custom key names
      - type: client
        dnsNames:
          - api-client.localhost
        k8sSecret:
          name: api-client-tls
          type: opaque
          keys:
            privateKey: client.key
            cert: client.crt
            ca: ca.crt
```

- **`type`**: `server` (ExtKeyUsageServerAuth) or `client` (ExtKeyUsageClientAuth)
- **`dnsNames`**: Subject Alternative Names for the certificate. Supports wildcards (e.g., `*.api.localhost`). Must use a reserved TLD: `.localhost`, `.test`, `.example`, `.invalid`, `.local`, `.internal`, or `.home.arpa`
- **`k8sSecret.type`**: `tls` uses standard keys (`tls.crt`, `tls.key`, `ca.crt`); `opaque` uses custom key names defined in `keys`

Certificates are automatically issued during `pilot install` and renewed when they have less than 14 days of validity remaining.

### Local Services

Configure which services route cluster traffic to your machine (see [Local Services](#local-services) above for how routing works):

```yaml
localServices:
  - name: api
    localPort: 8080           # Your local server port
    kubernetesPort: 80        # The K8s service port
    healthCheckPath: /health  # Health check endpoint
    selector:                 # Must match the K8s service selector
      app: api
```

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
pilot install -p infra      # Deploy postgres and redis
pilot update -p backend     # Build and deploy api and worker
pilot update                # Default profile (api, worker, frontend)
pilot uninstall -p all      # Remove everything
```

### Secrets in Helm Values

Reference encrypted secrets in your Helm arguments:

```yaml
helmArgs:
  - --set=database.password={{.Secrets.DB_PASSWORD}}
  - --set=api.key={{.Secrets.API_KEY}}
```

```bash
pilot secret set DB_PASSWORD
pilot secret set API_KEY
pilot install api
```

### Custom Scripts

Define reusable commands for your workflow:

```yaml
scripts:
  reset-db: |
    kubectl delete pvc -l app=postgres
    pilot uninstall postgres
    pilot install postgres
  logs: kubectl logs -f deployment/api
  shell: kubectl exec -it deployment/api -- /bin/sh
```

```bash
pilot run reset-db
pilot run logs
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

When `import` is set, services are matched by name and merged with the following rules:

| Field                                        | Merge strategy                                                         |
|----------------------------------------------|------------------------------------------------------------------------|
| Scalar fields (`gitRef`, `helmBranch`, etc.) | Local value overrides base (if non-empty)                              |
| `dockerImages`                               | Merged by name; individual image fields are overlaid                   |
| `remoteImages`                               | Appended to base list                                                  |
| `certificates`                               | Merged by `k8sSecret.name`; individual certificate fields are overlaid |

## Common Workflows

### Iterative Development

```bash
# Make changes to your api service, then rebuild and redeploy
pilot update api

# Check intercepted traffic if something isn't working
pilot context info
```

### Working on Multiple Services

```bash
# Deploy infrastructure first
pilot install -p infra

# Work on backend services
pilot update -p backend

# Start both api and worker locally
./start-api.sh &
./start-worker.sh &

# Both receive cluster traffic now
```

### Switching Projects

```bash
pilot context list
# my-app
# other-project
# legacy-system

pilot context set other-project
pilot update
```

## Shell Completion

Enable tab completion for commands, service names, and profiles:

```bash
# Bash
source <(pilot completion bash)

# Zsh
source <(pilot completion zsh)

# Fish
pilot completion fish | source

# PowerShell
pilot completion powershell | Out-String | Invoke-Expression
```

## File Storage

Pilot stores data in `~/.pilot/`:
- Cloned repositories for Helm charts and Docker builds
- Encrypted secrets (per context)
- Certificate authority files (per context)
- Generated dev-proxy configuration

Configuration lives at `~/.pilot-config.yaml`.

## Uninstall

```bash
# Remove Kubernetes resources
pilot uninstall -p all

# Delete local data
rm -rf ~/.pilot ~/.pilot-config.yaml

# Remove the binary (if installed manually)
rm $(which pilot)

# Or via Homebrew
brew uninstall pilot
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

If you're using SSH URLs for git repositories (e.g., `git@github.com:user/repo.git`) and your SSH key has a passphrase, Pilot will fail with an "SSH authentication failed" error. This happens because Pilot runs git commands non-interactively.

To fix this, add your SSH key to the ssh-agent before running Pilot:

```bash
# Start ssh-agent if not already running
eval "$(ssh-agent -s)"

# Add your SSH key (will prompt for passphrase once)
ssh-add ~/.ssh/id_rsa

# Now Pilot commands will work
pilot build
pilot update
```

On macOS, you can add the key permanently to the keychain:
```bash
ssh-add --apple-use-keychain ~/.ssh/id_rsa
```

On Linux, you can configure your shell profile to start ssh-agent automatically.
