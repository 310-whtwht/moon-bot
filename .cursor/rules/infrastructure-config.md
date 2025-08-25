# インフラ・デプロイ設定仕様

## 1. Kubernetes設定

### 1.1 Namespace設定
```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: trading-system
  labels:
    name: trading-system
    environment: production
```

### 1.2 ConfigMap設定
```yaml
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: trading-system-config
  namespace: trading-system
data:
  # データベース設定
  DB_HOST: "mysql-service"
  DB_PORT: "3306"
  DB_NAME: "trading_system"
  
  # Redis設定
  REDIS_HOST: "redis-service"
  REDIS_PORT: "6379"
  REDIS_DB: "0"
  
  # ブローカー設定
  BROKER_HOST: "localhost"
  BROKER_PORT: "11111"
  BROKER_TRD_ENV: "SIMULATE"
  
  # アプリケーション設定
  APP_ENV: "production"
  LOG_LEVEL: "info"
  API_PORT: "8080"
  
  # 監視設定
  METRICS_PORT: "9090"
  HEALTH_CHECK_PORT: "8081"
```

### 1.3 Secret設定
```yaml
# k8s/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: trading-system-secrets
  namespace: trading-system
type: Opaque
data:
  # データベース認証情報
  DB_USERNAME: <base64-encoded-username>
  DB_PASSWORD: <base64-encoded-password>
  
  # Redis認証情報
  REDIS_PASSWORD: <base64-encoded-password>
  
  # ブローカー認証情報
  BROKER_API_KEY: <base64-encoded-api-key>
  BROKER_SECRET: <base64-encoded-secret>
  
  # JWT秘密鍵
  JWT_SECRET: <base64-encoded-jwt-secret>
  
  # 暗号化鍵
  ENCRYPTION_KEY: <base64-encoded-encryption-key>
```

### 1.4 API Deployment
```yaml
# k8s/api-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trading-api
  namespace: trading-system
  labels:
    app: trading-api
    version: v1.0.0
spec:
  replicas: 3
  selector:
    matchLabels:
      app: trading-api
  template:
    metadata:
      labels:
        app: trading-api
        version: v1.0.0
    spec:
      containers:
      - name: api
        image: trading-system/api:v1.0.0
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        - containerPort: 8081
          name: health
        env:
        - name: DB_HOST
          valueFrom:
            configMapKeyRef:
              name: trading-system-config
              key: DB_HOST
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: DB_PASSWORD
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: REDIS_PASSWORD
        - name: BROKER_API_KEY
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: BROKER_API_KEY
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        volumeMounts:
        - name: logs
          mountPath: /app/logs
      volumes:
      - name: logs
        emptyDir: {}
```

### 1.5 Bot Deployment
```yaml
# k8s/bot-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trading-bot
  namespace: trading-system
  labels:
    app: trading-bot
    version: v1.0.0
spec:
  replicas: 2
  selector:
    matchLabels:
      app: trading-bot
  template:
    metadata:
      labels:
        app: trading-bot
        version: v1.0.0
    spec:
      containers:
      - name: bot
        image: trading-system/bot:v1.0.0
        ports:
        - containerPort: 9090
          name: metrics
        env:
        - name: DB_HOST
          valueFrom:
            configMapKeyRef:
              name: trading-system-config
              key: DB_HOST
        - name: REDIS_HOST
          valueFrom:
            configMapKeyRef:
              name: trading-system-config
              key: REDIS_HOST
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "200m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 9090
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
```

### 1.6 Web Deployment
```yaml
# k8s/web-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trading-web
  namespace: trading-system
  labels:
    app: trading-web
    version: v1.0.0
spec:
  replicas: 2
  selector:
    matchLabels:
      app: trading-web
  template:
    metadata:
      labels:
        app: trading-web
        version: v1.0.0
    spec:
      containers:
      - name: web
        image: trading-system/web:v1.0.0
        ports:
        - containerPort: 3000
          name: http
        env:
        - name: NEXT_PUBLIC_API_URL
          value: "https://api.trading-system.com"
        - name: NEXTAUTH_SECRET
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: JWT_SECRET
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "200m"
        livenessProbe:
          httpGet:
            path: /
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
```

### 1.7 Service設定
```yaml
# k8s/services.yaml
apiVersion: v1
kind: Service
metadata:
  name: trading-api-service
  namespace: trading-system
spec:
  selector:
    app: trading-api
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  name: trading-web-service
  namespace: trading-system
spec:
  selector:
    app: trading-web
  ports:
  - name: http
    port: 80
    targetPort: 3000
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  name: mysql-service
  namespace: trading-system
spec:
  selector:
    app: mysql
  ports:
  - name: mysql
    port: 3306
    targetPort: 3306
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  name: redis-service
  namespace: trading-system
spec:
  selector:
    app: redis
  ports:
  - name: redis
    port: 6379
    targetPort: 6379
  type: ClusterIP
```

### 1.8 Ingress設定
```yaml
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: trading-system-ingress
  namespace: trading-system
  annotations:
    kubernetes.io/ingress.class: "nginx"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "300"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "300"
spec:
  tls:
  - hosts:
    - api.trading-system.com
    - trading-system.com
    secretName: trading-system-tls
  rules:
  - host: api.trading-system.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: trading-api-service
            port:
              number: 80
  - host: trading-system.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: trading-web-service
            port:
              number: 80
```

## 2. データベース設定

### 2.1 MySQL Deployment
```yaml
# k8s/mysql-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mysql
  namespace: trading-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      containers:
      - name: mysql
        image: mysql:8.0
        ports:
        - containerPort: 3306
        env:
        - name: MYSQL_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: DB_PASSWORD
        - name: MYSQL_DATABASE
          value: "trading_system"
        - name: MYSQL_USER
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: DB_USERNAME
        - name: MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: DB_PASSWORD
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        volumeMounts:
        - name: mysql-data
          mountPath: /var/lib/mysql
        - name: mysql-config
          mountPath: /etc/mysql/conf.d
      volumes:
      - name: mysql-data
        persistentVolumeClaim:
          claimName: mysql-pvc
      - name: mysql-config
        configMap:
          name: mysql-config
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mysql-pvc
  namespace: trading-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi
  storageClassName: fast-ssd
```

### 2.2 MySQL ConfigMap
```yaml
# k8s/mysql-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mysql-config
  namespace: trading-system
data:
  my.cnf: |
    [mysqld]
    # 基本設定
    default-authentication-plugin=mysql_native_password
    character-set-server=utf8mb4
    collation-server=utf8mb4_unicode_ci
    
    # パフォーマンス設定
    innodb_buffer_pool_size=1G
    innodb_log_file_size=256M
    innodb_log_buffer_size=64M
    innodb_flush_log_at_trx_commit=2
    innodb_flush_method=O_DIRECT
    
    # 接続設定
    max_connections=200
    max_connect_errors=1000
    
    # クエリキャッシュ
    query_cache_type=1
    query_cache_size=128M
    
    # ログ設定
    slow_query_log=1
    slow_query_log_file=/var/log/mysql/slow.log
    long_query_time=2
    
    # タイムゾーン
    default-time-zone='+00:00'
```

### 2.3 Redis Deployment
```yaml
# k8s/redis-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: trading-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
        command:
        - redis-server
        - /etc/redis/redis.conf
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: trading-system-secrets
              key: REDIS_PASSWORD
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        volumeMounts:
        - name: redis-data
          mountPath: /data
        - name: redis-config
          mountPath: /etc/redis
      volumes:
      - name: redis-data
        persistentVolumeClaim:
          claimName: redis-pvc
      - name: redis-config
        configMap:
          name: redis-config
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-pvc
  namespace: trading-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: fast-ssd
```

### 2.4 Redis ConfigMap
```yaml
# k8s/redis-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: redis-config
  namespace: trading-system
data:
  redis.conf: |
    # 基本設定
    bind 0.0.0.0
    port 6379
    requirepass ${REDIS_PASSWORD}
    
    # メモリ設定
    maxmemory 800mb
    maxmemory-policy allkeys-lru
    
    # 永続化設定
    save 900 1
    save 300 10
    save 60 10000
    rdbcompression yes
    dbfilename dump.rdb
    dir /data
    
    # AOF設定
    appendonly yes
    appendfilename "appendonly.aof"
    appendfsync everysec
    
    # ログ設定
    loglevel notice
    logfile /data/redis.log
    
    # ネットワーク設定
    tcp-keepalive 300
    timeout 0
    
    # セキュリティ設定
    protected-mode yes
```

## 3. 監視設定

### 3.1 Prometheus設定
```yaml
# monitoring/prometheus-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    
    rule_files:
      - "alerts/*.yml"
    
    scrape_configs:
      - job_name: 'kubernetes-pods'
        kubernetes_sd_configs:
          - role: pod
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
            action: keep
            regex: true
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
            action: replace
            target_label: __metrics_path__
            regex: (.+)
          - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
            action: replace
            regex: ([^:]+)(?::\d+)?;(\d+)
            replacement: $1:$2
            target_label: __address__
          - action: labelmap
            regex: __meta_kubernetes_pod_label_(.+)
          - source_labels: [__meta_kubernetes_namespace]
            action: replace
            target_label: kubernetes_namespace
          - source_labels: [__meta_kubernetes_pod_name]
            action: replace
            target_label: kubernetes_pod_name
    
      - job_name: 'trading-api'
        static_configs:
          - targets: ['trading-api-service:9090']
        metrics_path: /metrics
        scrape_interval: 30s
    
      - job_name: 'trading-bot'
        static_configs:
          - targets: ['trading-bot:9090']
        metrics_path: /metrics
        scrape_interval: 30s
```

### 3.2 Grafana設定
```yaml
# monitoring/grafana-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-datasources
  namespace: monitoring
data:
  datasources.yaml: |
    apiVersion: 1
    datasources:
      - name: Prometheus
        type: prometheus
        url: http://prometheus-service:9090
        access: proxy
        isDefault: true
      - name: MySQL
        type: mysql
        url: mysql-service:3306
        database: trading_system
        user: ${DB_USERNAME}
        secureJsonData:
          password: ${DB_PASSWORD}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards
  namespace: monitoring
data:
  trading-dashboard.json: |
    {
      "dashboard": {
        "title": "Trading System Dashboard",
        "panels": [
          {
            "title": "API Response Time",
            "type": "graph",
            "targets": [
              {
                "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
                "legendFormat": "95th Percentile"
              }
            ]
          },
          {
            "title": "Order Count",
            "type": "stat",
            "targets": [
              {
                "expr": "rate(trading_orders_total[5m])",
                "legendFormat": "Orders/sec"
              }
            ]
          },
          {
            "title": "Active Strategies",
            "type": "stat",
            "targets": [
              {
                "expr": "trading_active_strategies",
                "legendFormat": "Active"
              }
            ]
          },
          {
            "title": "PnL",
            "type": "graph",
            "targets": [
              {
                "expr": "trading_pnl_total",
                "legendFormat": "Total PnL"
              }
            ]
          }
        ]
      }
    }
```

### 3.3 アラート設定
```yaml
# monitoring/alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: trading-alerts
  namespace: monitoring
spec:
  groups:
  - name: trading
    rules:
    - alert: HighErrorRate
      expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
      for: 2m
      labels:
        severity: warning
      annotations:
        summary: "High error rate detected"
        description: "Error rate is {{ $value | humanizePercentage }}"
    
    - alert: SlowResponseTime
      expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1.0
      for: 3m
      labels:
        severity: warning
      annotations:
        summary: "Slow response time detected"
        description: "95th percentile response time is {{ $value }}s"
    
    - alert: DatabaseConnectionFailed
      expr: up{job="mysql"} == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "Database connection failed"
        description: "MySQL database is down"
    
    - alert: RedisConnectionFailed
      expr: up{job="redis"} == 0
      for: 1m
      labels:
        severity: critical
      annotations:
        summary: "Redis connection failed"
        description: "Redis is down"
    
    - alert: HighMemoryUsage
      expr: (container_memory_usage_bytes{container="api"} / container_spec_memory_limit_bytes{container="api"}) > 0.8
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "High memory usage"
        description: "Memory usage is {{ $value | humanizePercentage }}"
    
    - alert: HighCPUUsage
      expr: (rate(container_cpu_usage_seconds_total{container="api"}[5m]) / container_spec_cpu_quota{container="api"}) > 0.8
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "High CPU usage"
        description: "CPU usage is {{ $value | humanizePercentage }}"
```

## 4. ログ管理

### 4.1 Fluentd設定
```yaml
# logging/fluentd-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluentd-config
  namespace: logging
data:
  fluent.conf: |
    <source>
      @type tail
      path /var/log/containers/*.log
      pos_file /var/log/fluentd-containers.log.pos
      tag kubernetes.*
      read_from_head true
      <parse>
        @type json
        time_format %Y-%m-%dT%H:%M:%S.%NZ
      </parse>
    </source>
    
    <filter kubernetes.**>
      @type kubernetes_metadata
      @id filter_kube_metadata
    </filter>
    
    <filter kubernetes.**>
      @type record_transformer
      enable_ruby true
      <record>
        @timestamp ${time.strftime('%Y-%m-%dT%H:%M:%S.%NZ')}
        level ${record['level'] || 'info'}
        message ${record['message'] || record['log']}
      </record>
    </filter>
    
    <match kubernetes.**>
      @type elasticsearch
      host elasticsearch-service
      port 9200
      logstash_format true
      logstash_prefix k8s
      <buffer>
        @type file
        path /var/log/fluentd-buffers/kubernetes.system.buffer
        flush_mode interval
        retry_type exponential_backoff
        flush_interval 5s
        retry_forever false
        retry_max_interval 30
        chunk_limit_size 2M
        queue_limit_length 8
        overflow_action block
      </buffer>
    </match>
```

### 4.2 Elasticsearch設定
```yaml
# logging/elasticsearch-deployment.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: elasticsearch
  namespace: logging
spec:
  serviceName: elasticsearch-service
  replicas: 3
  selector:
    matchLabels:
      app: elasticsearch
  template:
    metadata:
      labels:
        app: elasticsearch
    spec:
      containers:
      - name: elasticsearch
        image: docker.elastic.co/elasticsearch/elasticsearch:8.8.0
        ports:
        - containerPort: 9200
          name: http
        - containerPort: 9300
          name: transport
        env:
        - name: cluster.name
          value: "trading-cluster"
        - name: node.name
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: discovery.seed_hosts
          value: "elasticsearch-0.elasticsearch-service,elasticsearch-1.elasticsearch-service,elasticsearch-2.elasticsearch-service"
        - name: cluster.initial_master_nodes
          value: "elasticsearch-0,elasticsearch-1,elasticsearch-2"
        - name: ES_JAVA_OPTS
          value: "-Xms1g -Xmx1g"
        resources:
          requests:
            memory: "2Gi"
            cpu: "500m"
          limits:
            memory: "4Gi"
            cpu: "1000m"
        volumeMounts:
        - name: elasticsearch-data
          mountPath: /usr/share/elasticsearch/data
      volumes:
      - name: elasticsearch-data
        persistentVolumeClaim:
          claimName: elasticsearch-pvc
```

## 5. バックアップ設定

### 5.1 データベースバックアップ
```yaml
# backup/mysql-backup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: mysql-backup
  namespace: trading-system
spec:
  schedule: "0 2 * * *"  # 毎日午前2時
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: mysql:8.0
            command:
            - /bin/bash
            - -c
            - |
              mysqldump -h mysql-service -u $DB_USERNAME -p$DB_PASSWORD \
                --single-transaction --routines --triggers \
                trading_system > /backup/full_$(date +%Y%m%d_%H%M%S).sql
            env:
            - name: DB_USERNAME
              valueFrom:
                secretKeyRef:
                  name: trading-system-secrets
                  key: DB_USERNAME
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: trading-system-secrets
                  key: DB_PASSWORD
            volumeMounts:
            - name: backup-volume
              mountPath: /backup
          volumes:
          - name: backup-volume
            persistentVolumeClaim:
              claimName: backup-pvc
          restartPolicy: OnFailure
```

### 5.2 オブジェクトストレージバックアップ
```yaml
# backup/object-storage-backup.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: object-storage-backup
  namespace: trading-system
spec:
  schedule: "0 3 * * *"  # 毎日午前3時
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: trading-system/backup:v1.0.0
            command:
            - /app/backup.sh
            env:
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: object-storage-secrets
                  key: access-key
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: object-storage-secrets
                  key: secret-key
            - name: S3_BUCKET
              value: "trading-system-backup"
            volumeMounts:
            - name: backup-volume
              mountPath: /backup
          volumes:
          - name: backup-volume
            persistentVolumeClaim:
              claimName: backup-pvc
          restartPolicy: OnFailure
```

## 6. セキュリティ設定

### 6.1 NetworkPolicy
```yaml
# security/network-policies.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: trading-api-network-policy
  namespace: trading-system
spec:
  podSelector:
    matchLabels:
      app: trading-api
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: trading-system
    ports:
    - protocol: TCP
      port: 8080
    - protocol: TCP
      port: 9090
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: trading-system
    ports:
    - protocol: TCP
      port: 3306
    - protocol: TCP
      port: 6379
  - to: []
    ports:
    - protocol: TCP
      port: 53
    - protocol: UDP
      port: 53
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: database-network-policy
  namespace: trading-system
spec:
  podSelector:
    matchLabels:
      app: mysql
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: trading-system
    ports:
    - protocol: TCP
      port: 3306
```

### 6.2 PodSecurityPolicy
```yaml
# security/pod-security-policy.yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: trading-system-psp
spec:
  privileged: false
  allowPrivilegeEscalation: false
  requiredDropCapabilities:
  - ALL
  volumes:
  - 'configMap'
  - 'emptyDir'
  - 'projected'
  - 'secret'
  - 'downwardAPI'
  - 'persistentVolumeClaim'
  hostNetwork: false
  hostIPC: false
  hostPID: false
  runAsUser:
    rule: 'MustRunAsNonRoot'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'MustRunAs'
    ranges:
    - min: 1
      max: 65535
  fsGroup:
    rule: 'MustRunAs'
    ranges:
    - min: 1
      max: 65535
  readOnlyRootFilesystem: true
```

## 7. CI/CD設定

### 7.1 GitHub Actions
```yaml
# .github/workflows/deploy.yml
name: Deploy to Kubernetes

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Run tests
      run: |
        go test ./...
        go vet ./...
    
    - name: Set up Node.js
      uses: actions/setup-node@v3
      with:
        node-version: '18'
    
    - name: Install dependencies
      run: npm ci
    
    - name: Run tests
      run: npm test
    
    - name: Build
      run: npm run build

  build-and-push:
    needs: test
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
    - uses: actions/checkout@v3
    
    - name: Log in to Container Registry
      uses: docker/login-action@v2
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
    
    - name: Build and push API image
      uses: docker/build-push-action@v4
      with:
        context: ./apps/api
        push: true
        tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}/api:${{ github.sha }}
    
    - name: Build and push Bot image
      uses: docker/build-push-action@v4
      with:
        context: ./apps/bot
        push: true
        tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}/bot:${{ github.sha }}
    
    - name: Build and push Web image
      uses: docker/build-push-action@v4
      with:
        context: ./apps/web
        push: true
        tags: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}/web:${{ github.sha }}

  deploy:
    needs: build-and-push
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up kubectl
      uses: azure/setup-kubectl@v3
      with:
        version: 'latest'
    
    - name: Configure kubectl
      run: |
        echo "${{ secrets.KUBE_CONFIG }}" | base64 -d > kubeconfig
        export KUBECONFIG=kubeconfig
    
    - name: Deploy to Kubernetes
      run: |
        kubectl set image deployment/trading-api api=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}/api:${{ github.sha }} -n trading-system
        kubectl set image deployment/trading-bot bot=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}/bot:${{ github.sha }} -n trading-system
        kubectl set image deployment/trading-web web=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}/web:${{ github.sha }} -n trading-system
        kubectl rollout status deployment/trading-api -n trading-system
        kubectl rollout status deployment/trading-bot -n trading-system
        kubectl rollout status deployment/trading-web -n trading-system
```