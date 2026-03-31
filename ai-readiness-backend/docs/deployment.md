# Deployment Guide

## Local development (Docker Compose)

```bash
# Start MongoDB + API
make docker-up

# With Mongo Express UI at :8081
make docker-up-debug

# Tail logs
make docker-logs

# Stop
make docker-down
```

---

## Production: Kubernetes

### Prerequisites
- kubectl + cluster access
- MongoDB Atlas (or self-hosted with replica set)
- Container registry (GitHub Container Registry / Docker Hub / ECR)

### 1. Create namespace & secrets

```bash
kubectl create namespace ai-readiness

kubectl create secret generic ai-readiness-secrets \
  --namespace ai-readiness \
  --from-literal=MONGO_URI="mongodb://localhost:27017"
```

### 2. Apply manifests

```bash
kubectl apply -f k8s/
```

---

## Kubernetes manifests

### k8s/configmap.yaml

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ai-readiness-config
  namespace: ai-readiness
data:
  PORT: "8080"
  ENV: "production"
  MONGO_DB: "ai_readiness"
  CORS_ORIGINS: "https://app.your-domain.com"
  RATE_LIMIT_RPM: "120"
  PDF_TMP_DIR: "/tmp/ai-readiness-pdfs"
  LOG_LEVEL: "info"
  QUESTION_BANK_PATH: "/app/question-bank-v1.json"
```

### k8s/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-readiness-api
  namespace: ai-readiness
  labels:
    app: ai-readiness-api
    version: "1.0.0"
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ai-readiness-api
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: ai-readiness-api
    spec:
      terminationGracePeriodSeconds: 30
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
      containers:
        - name: api
          image: ghcr.io/yourorg/ai-readiness-backend:latest
          imagePullPolicy: Always
          ports:
            - containerPort: 8080
              name: http
          envFrom:
            - configMapRef:
                name: ai-readiness-config
          env:
            - name: MONGO_URI
              valueFrom:
                secretKeyRef:
                  name: ai-readiness-secrets
                  key: MONGO_URI
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "256Mi"
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 30
            failureThreshold: 3
          volumeMounts:
            - name: pdf-tmp
              mountPath: /tmp/ai-readiness-pdfs
      volumes:
        - name: pdf-tmp
          emptyDir: {}
```

### k8s/service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  name: ai-readiness-api
  namespace: ai-readiness
spec:
  selector:
    app: ai-readiness-api
  ports:
    - port: 80
      targetPort: 8080
      name: http
  type: ClusterIP
```

### k8s/ingress.yaml (nginx)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ai-readiness-ingress
  namespace: ai-readiness
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - api.your-domain.com
      secretName: ai-readiness-tls
  rules:
    - host: api.your-domain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ai-readiness-api
                port:
                  name: http
```

### k8s/hpa.yaml (autoscaling)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ai-readiness-api-hpa
  namespace: ai-readiness
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ai-readiness-api
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

---

## Environment variable reference

| Variable              | Required | Default                  | Description                              |
|-----------------------|----------|--------------------------|------------------------------------------|
| `MONGO_URI`           | ✅       | —                        | MongoDB connection string                |
| `MONGO_DB`            |          | `ai_readiness`           | Database name                            |
| `PORT`                |          | `8080`                   | HTTP listen port                         |
| `ENV`                 |          | `development`            | `development` or `production`            |
| `CORS_ORIGINS`        |          | `http://localhost:3000`  | Comma-separated allowed origins          |
| `RATE_LIMIT_RPM`      |          | `60`                     | Requests per minute per IP               |
| `PDF_TMP_DIR`         |          | `/tmp/ai-readiness-pdfs` | Directory for generated PDFs             |
| `LOG_LEVEL`           |          | `info`                   | `debug` / `info` / `warn` / `error`      |
| `QUESTION_BANK_PATH`  |          | `question-bank-v1.json`  | Path to question bank JSON               |

---

## Running database migrations

```bash
# Local
MONGO_URI=mongodb://localhost:27017 go run scripts/migrate.go

# With seed data
MONGO_URI=mongodb://localhost:27017 SEED=true go run scripts/migrate.go

# Against Atlas
MONGO_URI="mongodb+srv://..." go run scripts/migrate.go
```

---

## Observability

The server emits structured JSON logs (zap) on every request including:
- `method`, `path`, `status`, `latency`, `request_id`, `remote_addr`

Recommended stack: **Loki + Grafana** or **Datadog** log shipping.

For metrics, add `go-chi/middleware.Prometheus` and scrape with **Prometheus + Grafana**.

---

## MongoDB Atlas recommended settings

- **Tier**: M10+ for production (M0 free tier works for development)
- **Region**: Same as your API deployment
- **Backups**: Enable continuous backups
- **Network access**: Whitelist your Kubernetes node IPs or use VPC peering
- **Auth**: Use SCRAM-SHA-256 with a dedicated DB user scoped to `ai_readiness` db only
