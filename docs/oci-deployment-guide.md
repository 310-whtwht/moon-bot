# OCI デプロイメントガイド

## 概要

このドキュメントでは、Moomoo トレーディングシステムを Oracle Cloud Infrastructure (OCI) にデプロイする完全な手順を説明します。

## 前提条件

- OCI アカウント（有効なクレジットカードが必要）
- OCI CLI がインストール済み
- Docker がインストール済み
- Git がインストール済み

## 1. OCI プロジェクトの初期設定

### 1.1 OCI CLI の設定

```bash
# OCI CLI のインストール（まだの場合）
bash -c "$(curl -L https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh)"

# OCI CLI の設定
oci setup config
```

設定時に以下の情報が必要です：
- OCI User OCID
- Tenancy OCID
- Region
- 認証方法（API Key または Interactive）

### 1.2 コンパートメントの作成

```bash
# コンパートメントの作成
oci iam compartment create \
  --compartment-id <TENANCY_OCID> \
  --name "moomoo-trading" \
  --description "Moomoo Trading System Infrastructure"
```

## 2. ネットワーク設定

### 2.1 VCN の作成

```bash
# VCN の作成
oci network vcn create \
  --compartment-id <COMPARTMENT_OCID> \
  --display-name "moomoo-vcn" \
  --cidr-blocks '["10.0.0.0/16"]' \
  --dns-label "moomootrading"

# サブネットの作成
oci network subnet create \
  --compartment-id <COMPARTMENT_OCID> \
  --vcn-id <VCN_OCID> \
  --display-name "moomoo-public-subnet" \
  --cidr-block "10.0.1.0/24" \
  --availability-domain <AVAILABILITY_DOMAIN> \
  --dns-label "public"
```

### 2.2 セキュリティリストの設定

```bash
# セキュリティリストの作成
oci network security-list create \
  --compartment-id <COMPARTMENT_OCID> \
  --vcn-id <VCN_OCID> \
  --display-name "moomoo-security-list" \
  --egress-security-rules '[{"destination": "0.0.0.0/0", "protocol": "all", "isStateless": false}]' \
  --ingress-security-rules '[{"source": "0.0.0.0/0", "protocol": "6", "tcpOptions": {"destinationPortRange": {"min": 80, "max": 80}}}, {"source": "0.0.0.0/0", "protocol": "6", "tcpOptions": {"destinationPortRange": {"min": 443, "max": 443}}}, {"source": "0.0.0.0/0", "protocol": "6", "tcpOptions": {"destinationPortRange": {"min": 22, "max": 22}}}]'
```

## 3. データベース設定

### 3.1 MySQL Database Service の作成

```bash
# MySQL Database System の作成
oci mysql db-system create \
  --compartment-id <COMPARTMENT_OCID> \
  --display-name "moomoo-mysql" \
  --description "MySQL Database for Moomoo Trading System" \
  --availability-domain <AVAILABILITY_DOMAIN> \
  --subnet-id <SUBNET_OCID> \
  --shape-name "MySQL.VM.Standard.E3.1.8GB" \
  --mysql-version "8.0.32" \
  --admin-username "admin" \
  --admin-password "SecurePassword123!" \
  --data-storage-size-in-gbs 50 \
  --hostname-label "moomoo-mysql" \
  --is-highly-available false \
  --configuration-id <MYSQL_CONFIGURATION_OCID>
```

### 3.2 Redis クラスターの作成

```bash
# Redis クラスターの作成（Redis Cloud Service または自前でインストール）
# この例では Compute Instance に Redis をインストールする方法を示します
```

## 4. コンピュートインスタンスの作成

### 4.1 アプリケーションサーバーの作成

```bash
# Compute Instance の作成
oci compute instance launch \
  --compartment-id <COMPARTMENT_OCID> \
  --display-name "moomoo-app-server" \
  --availability-domain <AVAILABILITY_DOMAIN> \
  --subnet-id <SUBNET_OCID> \
  --shape "VM.Standard.E3.Flex" \
  --shape-config '{"ocpus": 2, "memoryInGBs": 16}' \
  --image-id <UBUNTU_20_04_IMAGE_OCID> \
  --ssh-authorized-keys-file ~/.ssh/id_rsa.pub \
  --assign-public-ip true
```

### 4.2 セキュリティグループの設定

```bash
# セキュリティグループの作成
oci network security-group create \
  --compartment-id <COMPARTMENT_OCID> \
  --vcn-id <VCN_OCID> \
  --display-name "moomoo-app-sg"

# セキュリティグループルールの追加
oci network security-group add-security-rules \
  --security-group-id <SECURITY_GROUP_OCID> \
  --ingress-security-rules '[{"source": "0.0.0.0/0", "protocol": "6", "tcpOptions": {"destinationPortRange": {"min": 80, "max": 80}}}, {"source": "0.0.0.0/0", "protocol": "6", "tcpOptions": {"destinationPortRange": {"min": 443, "max": 443}}}, {"source": "0.0.0.0/0", "protocol": "6", "tcpOptions": {"destinationPortRange": {"min": 8081, "max": 8081}}}]'
```

## 5. アプリケーションのデプロイ

### 5.1 サーバーへの接続

```bash
# インスタンスのパブリックIPを取得
oci compute instance list-vnics \
  --instance-id <INSTANCE_OCID>

# SSH接続
ssh ubuntu@<PUBLIC_IP>
```

### 5.2 必要なソフトウェアのインストール

```bash
# システムの更新
sudo apt update && sudo apt upgrade -y

# Docker のインストール
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker ubuntu

# Docker Compose のインストール
sudo curl -L "https://github.com/docker/compose/releases/download/v2.20.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Git のインストール
sudo apt install git -y

# Go のインストール
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Node.js のインストール
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs
```

### 5.3 アプリケーションのクローンとセットアップ

```bash
# アプリケーションのクローン
git clone <YOUR_REPOSITORY_URL> moomoo-trading
cd moomoo-trading

# 環境変数ファイルの作成
cat > .env << EOF
# Database Configuration
DB_HOST=<MYSQL_HOST>
DB_PORT=3306
DB_USER=moomoo
DB_PASSWORD=SecurePassword123!
DB_NAME=moomoo_trading

# Redis Configuration
REDIS_HOST=<REDIS_HOST>
REDIS_PORT=6379
REDIS_PASSWORD=

# Moomoo Configuration
MOOMOO_HOST=<MOOMOO_HOST>
MOOMOO_USERNAME=<MOOMOO_USERNAME>
MOOMOO_PASSWORD=<MOOMOO_PASSWORD>
MOOMOO_APP_ID=<MOOMOO_APP_ID>
MOOMOO_APP_KEY=<MOOMOO_APP_KEY>

# Application Configuration
ENVIRONMENT=production
PORT=8081
EOF
```

### 5.4 データベースの初期化

```bash
# MySQL への接続
mysql -h <MYSQL_HOST> -u admin -p

# データベースとユーザーの作成
CREATE DATABASE moomoo_trading;
CREATE USER 'moomoo'@'%' IDENTIFIED BY 'SecurePassword123!';
GRANT ALL PRIVILEGES ON moomoo_trading.* TO 'moomoo'@'%';
FLUSH PRIVILEGES;
EXIT;

# スキーマの初期化
mysql -h <MYSQL_HOST> -u moomoo -p moomoo_trading < scripts/init.sql
```

### 5.5 アプリケーションのビルドとデプロイ

```bash
# API のビルド
cd apps/api
go mod download
go build -o main .

# Web アプリケーションのビルド
cd ../web
npm install
npm run build

# Docker Compose でのデプロイ
cd ../..
docker-compose -f docker-compose.prod.yml up -d
```

## 6. ロードバランサーの設定

### 6.1 Load Balancer の作成

```bash
# Load Balancer の作成
oci lb load-balancer create \
  --compartment-id <COMPARTMENT_OCID> \
  --display-name "moomoo-lb" \
  --shape-name "flexible" \
  --shape-details '{"minimumBandwidthInMbps": 10, "maximumBandwidthInMbps": 100}' \
  --subnet-ids '["<SUBNET_OCID>"]' \
  --is-private false

# Backend Set の作成
oci lb backend-set create \
  --load-balancer-id <LOAD_BALANCER_OCID> \
  --name "moomoo-backend-set" \
  --policy "ROUND_ROBIN" \
  --health-checker-protocol "HTTP" \
  --health-checker-port 8081 \
  --health-checker-url-path "/healthz"

# Backend の追加
oci lb backend create \
  --load-balancer-id <LOAD_BALANCER_OCID> \
  --backend-set-name "moomoo-backend-set" \
  --ip-address <INSTANCE_PRIVATE_IP> \
  --port 8081
```

### 6.2 リスナーの設定

```bash
# HTTP リスナーの作成
oci lb listener create \
  --load-balancer-id <LOAD_BALANCER_OCID> \
  --name "moomoo-http-listener" \
  --default-backend-set-name "moomoo-backend-set" \
  --port 80 \
  --protocol "HTTP"

# HTTPS リスナーの作成（SSL証明書が必要）
oci lb listener create \
  --load-balancer-id <LOAD_BALANCER_OCID> \
  --name "moomoo-https-listener" \
  --default-backend-set-name "moomoo-backend-set" \
  --port 443 \
  --protocol "HTTP" \
  --ssl-configuration '{"certificateName": "<CERTIFICATE_NAME>", "verifyPeerCertificate": false}'
```

## 7. 監視とログ設定

### 7.1 Cloud Monitoring の設定

```bash
# メトリクス名前空間の作成
oci monitoring namespace create \
  --compartment-id <COMPARTMENT_OCID> \
  --name "moomoo_trading"

# カスタムメトリクスの作成
oci monitoring metric create \
  --compartment-id <COMPARTMENT_OCID> \
  --namespace "moomoo_trading" \
  --name "api_requests_total" \
  --display-name "API Requests Total" \
  --description "Total number of API requests"
```

### 7.2 ログ設定

```bash
# ロググループの作成
oci logging log-group create \
  --compartment-id <COMPARTMENT_OCID> \
  --display-name "moomoo-logs"

# ログの作成
oci logging log create \
  --log-group-id <LOG_GROUP_OCID> \
  --display-name "moomoo-app-logs" \
  --log-type "CUSTOM"
```

## 8. セキュリティ設定

### 8.1 Vault の設定

```bash
# Vault の作成
oci kms vault create \
  --compartment-id <COMPARTMENT_OCID> \
  --display-name "moomoo-vault" \
  --vault-type "DEFAULT"

# マスターキーの作成
oci kms key create \
  --compartment-id <COMPARTMENT_OCID> \
  --display-name "moomoo-master-key" \
  --key-shape '{"algorithm": "AES", "length": 32}' \
  --protection-mode "SOFTWARE"
```

### 8.2 セキュリティポリシーの設定

```bash
# セキュリティポリシーの作成
oci iam policy create \
  --compartment-id <COMPARTMENT_OCID> \
  --name "moomoo-security-policy" \
  --description "Security policy for Moomoo Trading System" \
  --statements '["allow group Administrators to manage all-resources in compartment moomoo-trading", "allow group Developers to read all-resources in compartment moomoo-trading"]'
```

## 9. バックアップとディザスタリカバリ

### 9.1 自動バックアップの設定

```bash
# MySQL の自動バックアップ設定
oci mysql db-system update \
  --db-system-id <DB_SYSTEM_OCID> \
  --backup-policy '{"isEnabled": true, "windowStartTime": "02:00", "retentionInDays": 7, "freeformTags": {"backup-type": "automated"}}'
```

### 9.2 Object Storage でのバックアップ

```bash
# バケットの作成
oci os bucket create \
  --compartment-id <COMPARTMENT_OCID> \
  --name "moomoo-backups" \
  --public-access-type "NoPublicAccess"

# バックアップスクリプトの作成
cat > backup.sh << 'EOF'
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
mysqldump -h <MYSQL_HOST> -u moomoo -p moomoo_trading > backup_$DATE.sql
gzip backup_$DATE.sql
oci os object put --bucket-name moomoo-backups --file backup_$DATE.sql.gz
rm backup_$DATE.sql.gz
EOF

chmod +x backup.sh
```

## 10. CI/CD パイプラインの設定

### 10.1 GitHub Actions の設定

```yaml
# .github/workflows/deploy.yml
name: Deploy to OCI

on:
  push:
    branches: [ main ]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    
    - name: Setup OCI CLI
      uses: oracle-actions/setup-oci-cli@v1
      with:
        auth-type: 'api_key'
        config-file: ${{ secrets.OCI_CONFIG_FILE }}
    
    - name: Deploy to OCI
      run: |
        # デプロイスクリプトの実行
        ./scripts/deploy.sh
```

## 11. 運用監視

### 11.1 ヘルスチェックの設定

```bash
# ヘルスチェックエンドポイントの確認
curl -f http://<LOAD_BALANCER_IP>/healthz

# 自動ヘルスチェックスクリプト
cat > health_check.sh << 'EOF'
#!/bin/bash
if ! curl -f http://localhost:8081/healthz; then
  echo "Health check failed at $(date)" >> /var/log/health_check.log
  # アラート通知の送信
  curl -X POST <SLACK_WEBHOOK_URL> \
    -H 'Content-type: application/json' \
    -d '{"text": "Moomoo Trading System health check failed"}'
fi
EOF

# cron ジョブの追加
echo "*/5 * * * * /home/ubuntu/health_check.sh" | crontab -
```

### 11.2 ログローテーション

```bash
# logrotate の設定
sudo cat > /etc/logrotate.d/moomoo << EOF
/home/ubuntu/moomoo-trading/logs/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 ubuntu ubuntu
}
EOF
```

## 12. トラブルシューティング

### 12.1 よくある問題と解決方法

1. **接続エラー**
   ```bash
   # ファイアウォールの確認
   sudo ufw status
   sudo ufw allow 8081
   ```

2. **データベース接続エラー**
   ```bash
   # MySQL の状態確認
   sudo systemctl status mysql
   sudo mysql -u root -p -e "SHOW PROCESSLIST;"
   ```

3. **メモリ不足**
   ```bash
   # メモリ使用量の確認
   free -h
   # スワップの追加
   sudo fallocate -l 2G /swapfile
   sudo chmod 600 /swapfile
   sudo mkswap /swapfile
   sudo swapon /swapfile
   ```

### 12.2 ログの確認

```bash
# アプリケーションログの確認
docker-compose logs -f api
docker-compose logs -f web

# システムログの確認
sudo journalctl -u docker.service -f
sudo tail -f /var/log/syslog
```

## 13. コスト最適化

### 13.1 リソースの最適化

```bash
# インスタンスサイズの調整
oci compute instance update \
  --instance-id <INSTANCE_OCID> \
  --shape "VM.Standard.E3.Flex" \
  --shape-config '{"ocpus": 1, "memoryInGBs": 8}'

# 自動スケーリングの設定
oci compute-instance-pool create \
  --compartment-id <COMPARTMENT_OCID> \
  --instance-configuration-id <INSTANCE_CONFIG_OCID> \
  --size 1 \
  --display-name "moomoo-pool"
```

### 13.2 コスト監視

```bash
# コスト分析の設定
oci usage-api request create \
  --tenant-id <TENANCY_OCID> \
  --time-usage-started 2024-01-01T00:00:00Z \
  --time-usage-ended 2024-01-31T23:59:59Z \
  --granularity "DAILY" \
  --query-type "COST"
```

## 14. セキュリティ監査

### 14.1 セキュリティスキャン

```bash
# 脆弱性スキャンの実行
sudo apt install lynis -y
sudo lynis audit system

# ポートスキャン
nmap -sS -sV -O <INSTANCE_IP>
```

### 14.2 セキュリティログの監視

```bash
# ファイアウォールログの監視
sudo tail -f /var/log/ufw.log

# SSH ログイン試行の監視
sudo tail -f /var/log/auth.log | grep sshd
```

## 15. パフォーマンスチューニング

### 15.1 データベースの最適化

```sql
-- MySQL の設定最適化
SET GLOBAL innodb_buffer_pool_size = 1073741824; -- 1GB
SET GLOBAL max_connections = 200;
SET GLOBAL query_cache_size = 67108864; -- 64MB
```

### 15.2 アプリケーションの最適化

```bash
# Go アプリケーションの最適化
export GOMAXPROCS=4
export GOGC=100

# Node.js アプリケーションの最適化
export NODE_ENV=production
export NODE_OPTIONS="--max-old-space-size=4096"
```

## まとめ

このガイドに従って、Moomoo トレーディングシステムを OCI に完全にデプロイできます。各ステップを順番に実行し、必要に応じて設定を調整してください。

### 次のステップ

1. 本番環境でのテスト実行
2. パフォーマンス監視の設定
3. アラート通知の設定
4. 定期的なバックアップの確認
5. セキュリティ監査の実施

### サポート

問題が発生した場合は、以下を確認してください：
- OCI ドキュメント: https://docs.oracle.com/en-us/iaas/
- OCI サポート: https://www.oracle.com/support/
- プロジェクトの GitHub Issues