# ns-sentry

A Kubernetes controller that automatically secures and hardens new namespaces.

## What it does

When you create a new namespace, ns-sentry automatically:

- **Sets resource limits** - Prevents workloads from consuming unlimited CPU and memory
- **Implements network policies** - Denies all incoming and outgoing traffic by default, allowing only DNS queries to reach kube-system

This ensures every namespace starts with sensible defaults for resource management and network isolation, without requiring manual setup.

## Getting Started

### Prerequisites
- Kubernetes 1.20+
- kubectl

### Installation

```bash
kubectl apply -f https://github.com/yourusername/go-k8-tools/releases/download/v0.1.0/install.yaml
```

### Example

Create a namespace and ns-sentry will automatically harden it:

```bash
kubectl create namespace my-app
# ns-sentry automatically creates:
# - ResourceQuota: limits CPU and memory usage
# - NetworkPolicy: default-deny-all traffic (except DNS)
```

## License

Licensed under the Apache License, Version 2.0.
See [LICENSE](LICENSE) for details.