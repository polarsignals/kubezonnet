# Test Workload for Flow Log Verification

Simple configurable client-server workload to test cross-zone traffic monitoring.

## Configuration

### Server (via environment variables)
- `MODE=server` - Run in server mode
- `PORT=8888` - Port to listen on (default: 8888)

### Client (via environment variables)
- `MODE=client` - Run in client mode
- `SERVER=test-server:8888` - Server address (default: test-server:8888)
- `CHUNK_SIZE_MB=1` - Size of each chunk to send in MB (default: 1)
- `INTERVAL_MS=100` - Interval between sends in milliseconds (default: 100)

## Quick Start

```bash
# Build the image
cd test
docker build -t ghcr.io/polarsignals/kubezonnet-test-workload:latest .

# Push to registry (requires authentication)
docker push ghcr.io/polarsignals/kubezonnet-test-workload:latest

# OR for local testing with kind/minikube
kind load docker-image ghcr.io/polarsignals/kubezonnet-test-workload:latest
# OR for minikube
# minikube image load ghcr.io/polarsignals/kubezonnet-test-workload:latest

# Deploy (pods will automatically be placed in different zones)
kubectl apply -f deploy.yaml

# Verify pods are in different zones
kubectl get pods -n test-traffic -o wide

# Watch logs
kubectl logs -n test-traffic -l app=test-server -f
kubectl logs -n test-traffic -l app=test-client -f

# Check kubezonnet server logs for flow data
kubectl logs -n kubezonnet -l app=kubezonnet-server

# Check metrics on kubezonnet server
kubectl port-forward -n kubezonnet svc/kubezonnet-server 8080:8080
curl localhost:8080/metrics | grep pod_cross_zone
```

## Expected Behavior

With default settings (1MB chunk every 100ms):
- **Client → Server**: ~10 MB/s
- **Server → Client**: Minimal (~20 bytes/s for ACKs)

The pods use podAntiAffinity to ensure they are scheduled in different availability zones, which will trigger cross-zone traffic monitoring.

## Adjust Traffic Pattern

Edit the client deployment to change behavior:

```yaml
# Send 5MB every 500ms = 10 MB/s
- name: CHUNK_SIZE_MB
  value: "5"
- name: INTERVAL_MS
  value: "500"
```

## Clean up

```bash
kubectl delete namespace test-traffic
```
