# セキュリティ実装仕様

## 1. 認証・認可システム

### 1.1 JWT認証実装
```go
// internal/auth/jwt.go
package auth

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "time"
    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"
)

type JWTClaims struct {
    UserID   uint   `json:"user_id"`
    Email    string `json:"email"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}

type JWTService struct {
    secretKey []byte
    issuer    string
    audience  string
}

func NewJWTService(secretKey string) *JWTService {
    return &JWTService{
        secretKey: []byte(secretKey),
        issuer:    "trading-system",
        audience:  "trading-system-users",
    }
}

func (j *JWTService) GenerateToken(userID uint, email, role string) (string, error) {
    now := time.Now()
    claims := JWTClaims{
        UserID: userID,
        Email:  email,
        Role:   role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(now),
            NotBefore: jwt.NewNumericDate(now),
            Issuer:    j.issuer,
            Audience:  []string{j.audience},
            ID:        generateTokenID(),
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(j.secretKey)
}

func (j *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return j.secretKey, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
        return claims, nil
    }
    
    return nil, fmt.Errorf("invalid token")
}

func (j *JWTService) RefreshToken(tokenString string) (string, error) {
    claims, err := j.ValidateToken(tokenString)
    if err != nil {
        return "", err
    }
    
    // 新しいトークンを生成
    return j.GenerateToken(claims.UserID, claims.Email, claims.Role)
}

func generateTokenID() string {
    b := make([]byte, 16)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}
```

### 1.2 パスワードハッシュ化
```go
// internal/auth/password.go
package auth

import (
    "golang.org/x/crypto/bcrypt"
)

type PasswordService struct {
    cost int
}

func NewPasswordService() *PasswordService {
    return &PasswordService{
        cost: bcrypt.DefaultCost,
    }
}

func (p *PasswordService) HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), p.cost)
    return string(bytes), err
}

func (p *PasswordService) CheckPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

func (p *PasswordService) ValidatePasswordStrength(password string) error {
    if len(password) < 8 {
        return fmt.Errorf("password must be at least 8 characters long")
    }
    
    var (
        hasUpper   bool
        hasLower   bool
        hasNumber  bool
        hasSpecial bool
    )
    
    for _, char := range password {
        switch {
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsNumber(char):
            hasNumber = true
        case unicode.IsPunct(char) || unicode.IsSymbol(char):
            hasSpecial = true
        }
    }
    
    if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
        return fmt.Errorf("password must contain uppercase, lowercase, number, and special character")
    }
    
    return nil
}
```

### 1.3 TOTP（2FA）実装
```go
// internal/auth/totp.go
package auth

import (
    "crypto/rand"
    "encoding/base32"
    "fmt"
    "time"
    "github.com/pquerna/otp/totp"
    "github.com/pquerna/otp"
)

type TOTPService struct {
    issuer string
}

func NewTOTPService(issuer string) *TOTPService {
    return &TOTPService{
        issuer: issuer,
    }
}

func (t *TOTPService) GenerateSecret(email string) (*otp.Key, error) {
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      t.issuer,
        AccountName: email,
    })
    if err != nil {
        return nil, err
    }
    
    return key, nil
}

func (t *TOTPService) ValidateCode(secret, code string) bool {
    return totp.Validate(code, secret)
}

func (t *TOTPService) GenerateCode(secret string) (string, error) {
    return totp.GenerateCode(secret, time.Now())
}

func (t *TOTPService) GenerateBackupCodes() ([]string, error) {
    codes := make([]string, 10)
    for i := 0; i < 10; i++ {
        b := make([]byte, 4)
        _, err := rand.Read(b)
        if err != nil {
            return nil, err
        }
        codes[i] = fmt.Sprintf("%08d", int(b[0])<<24|int(b[1])<<16|int(b[2])<<8|int(b[3]))
    }
    return codes, nil
}
```

### 1.4 認証ミドルウェア
```go
// internal/middleware/auth.go
package middleware

import (
    "context"
    "net/http"
    "strings"
    "github.com/gin-gonic/gin"
    "github.com/trading-system/internal/auth"
)

type AuthMiddleware struct {
    jwtService *auth.JWTService
}

func NewAuthMiddleware(jwtService *auth.JWTService) *AuthMiddleware {
    return &AuthMiddleware{
        jwtService: jwtService,
    }
}

func (a *AuthMiddleware) Authenticate() gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
            c.Abort()
            return
        }
        
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        if tokenString == authHeader {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
            c.Abort()
            return
        }
        
        claims, err := a.jwtService.ValidateToken(tokenString)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
            c.Abort()
            return
        }
        
        // コンテキストにユーザー情報を設定
        ctx := context.WithValue(c.Request.Context(), "user", claims)
        c.Request = c.Request.WithContext(ctx)
        
        c.Next()
    }
}

func (a *AuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user, exists := c.Get("user")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            c.Abort()
            return
        }
        
        claims := user.(*auth.JWTClaims)
        for _, role := range roles {
            if claims.Role == role {
                c.Next()
                return
            }
        }
        
        c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
        c.Abort()
    }
}

func (a *AuthMiddleware) RateLimit(limit int, window time.Duration) gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Every(window), limit)
    
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

## 2. データ暗号化

### 2.1 AES-GCM暗号化
```go
// internal/security/encryption.go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
)

type EncryptionService struct {
    key []byte
}

func NewEncryptionService(key string) (*EncryptionService, error) {
    if len(key) != 32 {
        return nil, fmt.Errorf("encryption key must be 32 bytes")
    }
    
    return &EncryptionService{
        key: []byte(key),
    }, nil
}

func (e *EncryptionService) Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *EncryptionService) Decrypt(encryptedText string) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
    if err != nil {
        return "", err
    }
    
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", fmt.Errorf("ciphertext too short")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }
    
    return string(plaintext), nil
}

func (e *EncryptionService) EncryptSensitiveData(data map[string]interface{}) (map[string]interface{}, error) {
    encrypted := make(map[string]interface{})
    
    for key, value := range data {
        if isSensitiveField(key) {
            if str, ok := value.(string); ok {
                encryptedValue, err := e.Encrypt(str)
                if err != nil {
                    return nil, err
                }
                encrypted[key] = encryptedValue
            } else {
                encrypted[key] = value
            }
        } else {
            encrypted[key] = value
        }
    }
    
    return encrypted, nil
}

func isSensitiveField(fieldName string) bool {
    sensitiveFields := []string{
        "api_key", "secret_key", "password", "token", "private_key",
        "credit_card", "ssn", "phone", "email",
    }
    
    for _, field := range sensitiveFields {
        if strings.Contains(strings.ToLower(fieldName), field) {
            return true
        }
    }
    return false
}
```

### 2.2 OCI Vault統合
```go
// internal/security/vault.go
package security

import (
    "context"
    "fmt"
    "github.com/oracle/oci-go-sdk/v65/common"
    "github.com/oracle/oci-go-sdk/v65/vault"
)

type VaultService struct {
    client    vault.VaultsClient
    vaultID   string
    compartmentID string
}

func NewVaultService(configProvider common.ConfigurationProvider, vaultID, compartmentID string) (*VaultService, error) {
    client, err := vault.NewVaultsClientWithConfigurationProvider(configProvider)
    if err != nil {
        return nil, err
    }
    
    return &VaultService{
        client:         client,
        vaultID:        vaultID,
        compartmentID:  compartmentID,
    }, nil
}

func (v *VaultService) CreateSecret(secretName, secretContent string) (string, error) {
    request := vault.CreateSecretRequest{
        CreateSecretDetails: vault.CreateSecretDetails{
            CompartmentId: &v.compartmentID,
            VaultId:       &v.vaultID,
            KeyId:         &v.keyID,
            SecretName:    &secretName,
            SecretContent: vault.Base64SecretContentDetails{
                Content: &secretContent,
            },
        },
    }
    
    response, err := v.client.CreateSecret(context.Background(), request)
    if err != nil {
        return "", err
    }
    
    return *response.Secret.Id, nil
}

func (v *VaultService) GetSecret(secretID string) (string, error) {
    request := vault.GetSecretBundleRequest{
        SecretId: &secretID,
    }
    
    response, err := v.client.GetSecretBundle(context.Background(), request)
    if err != nil {
        return "", err
    }
    
    if response.SecretBundle.SecretBundleContent == nil {
        return "", fmt.Errorf("secret content is nil")
    }
    
    content := response.SecretBundle.SecretBundleContent.(vault.Base64SecretBundleContentDetails)
    return *content.Content, nil
}

func (v *VaultService) UpdateSecret(secretID, secretContent string) error {
    request := vault.UpdateSecretRequest{
        SecretId: &secretID,
        UpdateSecretDetails: vault.UpdateSecretDetails{
            SecretContent: vault.Base64SecretContentDetails{
                Content: &secretContent,
            },
        },
    }
    
    _, err := v.client.UpdateSecret(context.Background(), request)
    return err
}

func (v *VaultService) DeleteSecret(secretID string) error {
    request := vault.ScheduleSecretDeletionRequest{
        SecretId: &secretID,
        ScheduleSecretDeletionDetails: vault.ScheduleSecretDeletionDetails{
            TimeOfDeletion: &common.SDKTime{Time: time.Now().Add(24 * time.Hour)},
        },
    }
    
    _, err := v.client.ScheduleSecretDeletion(context.Background(), request)
    return err
}
```

## 3. 入力検証・サニタイゼーション

### 3.1 バリデーション実装
```go
// internal/validation/validator.go
package validation

import (
    "regexp"
    "strings"
    "unicode"
    "github.com/go-playground/validator/v10"
)

type Validator struct {
    validate *validator.Validate
}

func NewValidator() *Validator {
    v := validator.New()
    
    // カスタムバリデーション関数を登録
    v.RegisterValidation("symbol", validateSymbol)
    v.RegisterValidation("timeframe", validateTimeframe)
    v.RegisterValidation("order_type", validateOrderType)
    v.RegisterValidation("side", validateSide)
    v.RegisterValidation("safe_string", validateSafeString)
    
    return &Validator{
        validate: v,
    }
}

func (v *Validator) ValidateStruct(s interface{}) error {
    return v.validate.Struct(s)
}

func (v *Validator) ValidateVar(field interface{}, tag string) error {
    return v.validate.Var(field, tag)
}

// カスタムバリデーション関数
func validateSymbol(fl validator.FieldLevel) bool {
    symbol := fl.Field().String()
    
    // 銘柄コードの形式チェック
    if len(symbol) < 1 || len(symbol) > 10 {
        return false
    }
    
    // 英数字とドットのみ許可
    matched, _ := regexp.MatchString(`^[A-Z0-9.]+$`, symbol)
    return matched
}

func validateTimeframe(fl validator.FieldLevel) bool {
    timeframe := fl.Field().String()
    validTimeframes := []string{"1m", "5m", "15m", "1d"}
    
    for _, valid := range validTimeframes {
        if timeframe == valid {
            return true
        }
    }
    return false
}

func validateOrderType(fl validator.FieldLevel) bool {
    orderType := fl.Field().String()
    validTypes := []string{"market", "limit", "stop", "stop_limit", "trailing"}
    
    for _, valid := range validTypes {
        if orderType == valid {
            return true
        }
    }
    return false
}

func validateSide(fl validator.FieldLevel) bool {
    side := fl.Field().String()
    return side == "buy" || side == "sell"
}

func validateSafeString(fl validator.FieldLevel) bool {
    str := fl.Field().String()
    
    // 危険な文字列をチェック
    dangerousPatterns := []string{
        `<script`, `javascript:`, `onload=`, `onerror=`,
        `union select`, `drop table`, `insert into`,
        `../`, `..\\`, `%00`, `%0d`, `%0a`,
    }
    
    lowerStr := strings.ToLower(str)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(lowerStr, pattern) {
            return false
        }
    }
    
    return true
}
```

### 3.2 SQLインジェクション対策
```go
// internal/database/sql_injection.go
package database

import (
    "database/sql"
    "fmt"
    "strings"
)

type SQLInjectionProtection struct{}

func NewSQLInjectionProtection() *SQLInjectionProtection {
    return &SQLInjectionProtection{}
}

func (s *SQLInjectionProtection) SanitizeInput(input string) string {
    // 危険な文字をエスケープ
    dangerousChars := map[string]string{
        "'": "''",
        "\"": "\"\"",
        ";": "",
        "--": "",
        "/*": "",
        "*/": "",
        "xp_": "",
        "sp_": "",
    }
    
    sanitized := input
    for dangerous, replacement := range dangerousChars {
        sanitized = strings.ReplaceAll(sanitized, dangerous, replacement)
    }
    
    return sanitized
}

func (s *SQLInjectionProtection) ValidateTableName(tableName string) error {
    // 許可されたテーブル名のみ
    allowedTables := []string{
        "strategies", "orders", "trades", "market_data",
        "audit_logs", "users", "system_configs",
    }
    
    for _, allowed := range allowedTables {
        if tableName == allowed {
            return nil
        }
    }
    
    return fmt.Errorf("invalid table name: %s", tableName)
}

func (s *SQLInjectionProtection) ValidateColumnName(columnName string) error {
    // 許可されたカラム名のみ
    allowedColumns := map[string][]string{
        "strategies": {"id", "name", "description", "category", "is_active"},
        "orders":     {"id", "symbol", "side", "order_type", "quantity", "status"},
        "trades":     {"id", "symbol", "side", "quantity", "open_price", "close_price"},
    }
    
    // テーブル名とカラム名の組み合わせをチェック
    for table, columns := range allowedColumns {
        for _, column := range columns {
            if columnName == column {
                return nil
            }
        }
    }
    
    return fmt.Errorf("invalid column name: %s", columnName)
}

func (s *SQLInjectionProtection) BuildSafeQuery(baseQuery string, params map[string]interface{}) (string, []interface{}, error) {
    // パラメータ化クエリを使用
    var args []interface{}
    var placeholders []string
    
    for key, value := range params {
        // キーの検証
        if strings.Contains(key, " ") || strings.Contains(key, ";") {
            return "", nil, fmt.Errorf("invalid parameter key: %s", key)
        }
        
        args = append(args, value)
        placeholders = append(placeholders, "?")
    }
    
    // プレースホルダーで置換
    query := baseQuery
    for i, placeholder := range placeholders {
        query = strings.Replace(query, "?", placeholder, 1)
    }
    
    return query, args, nil
}
```

### 3.3 XSS対策
```go
// internal/security/xss.go
package security

import (
    "html"
    "regexp"
    "strings"
)

type XSSProtection struct{}

func NewXSSProtection() *XSSProtection {
    return &XSSProtection{}
}

func (x *XSSProtection) SanitizeHTML(input string) string {
    // HTMLエンティティをエスケープ
    sanitized := html.EscapeString(input)
    
    // 危険なタグを削除
    dangerousTags := []string{
        "script", "iframe", "object", "embed", "form",
        "input", "textarea", "select", "button",
    }
    
    for _, tag := range dangerousTags {
        pattern := regexp.MustCompile(fmt.Sprintf(`(?i)<%s[^>]*>.*?</%s>`, tag, tag))
        sanitized = pattern.ReplaceAllString(sanitized, "")
    }
    
    // 危険な属性を削除
    dangerousAttrs := []string{
        "onload", "onerror", "onclick", "onmouseover",
        "javascript:", "vbscript:", "data:",
    }
    
    for _, attr := range dangerousAttrs {
        pattern := regexp.MustCompile(fmt.Sprintf(`(?i)%s\s*=\s*["'][^"']*["']`, attr))
        sanitized = pattern.ReplaceAllString(sanitized, "")
    }
    
    return sanitized
}

func (x *XSSProtection) ValidateURL(url string) bool {
    // 許可されたプロトコルのみ
    allowedProtocols := []string{"http://", "https://", "mailto:"}
    
    url = strings.ToLower(url)
    for _, protocol := range allowedProtocols {
        if strings.HasPrefix(url, protocol) {
            return true
        }
    }
    
    return false
}

func (x *XSSProtection) SanitizeCSS(css string) string {
    // 危険なCSSプロパティを削除
    dangerousCSS := []string{
        "expression(", "url(javascript:", "behavior:",
        "background-image: url(javascript:",
    }
    
    sanitized := css
    for _, dangerous := range dangerousCSS {
        sanitized = strings.ReplaceAll(sanitized, dangerous, "")
    }
    
    return sanitized
}
```

## 4. CSRF対策

### 4.1 CSRFトークン実装
```go
// internal/security/csrf.go
package security

import (
    "crypto/rand"
    "encoding/base64"
    "time"
    "github.com/gin-gonic/gin"
)

type CSRFProtection struct {
    tokenLength int
    expiry      time.Duration
}

func NewCSRFProtection() *CSRFProtection {
    return &CSRFProtection{
        tokenLength: 32,
        expiry:      24 * time.Hour,
    }
}

func (c *CSRFProtection) GenerateToken() (string, error) {
    b := make([]byte, c.tokenLength)
    _, err := rand.Read(b)
    if err != nil {
        return "", err
    }
    
    return base64.URLEncoding.EncodeToString(b), nil
}

func (c *CSRFProtection) ValidateToken(token string, sessionToken string) bool {
    if token == "" || sessionToken == "" {
        return false
    }
    
    return token == sessionToken
}

func (c *CSRFProtection) CSRFMiddleware() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        if ctx.Request.Method == "GET" {
            // GETリクエストではトークンを生成
            token, err := c.GenerateToken()
            if err != nil {
                ctx.JSON(500, gin.H{"error": "Failed to generate CSRF token"})
                ctx.Abort()
                return
            }
            
            ctx.Set("csrf_token", token)
            ctx.Header("X-CSRF-Token", token)
        } else {
            // POST/PUT/DELETEリクエストではトークンを検証
            token := ctx.GetHeader("X-CSRF-Token")
            sessionToken := ctx.GetString("csrf_token")
            
            if !c.ValidateToken(token, sessionToken) {
                ctx.JSON(403, gin.H{"error": "Invalid CSRF token"})
                ctx.Abort()
                return
            }
        }
        
        ctx.Next()
    }
}
```

## 5. セキュリティヘッダー

### 5.1 セキュリティヘッダーミドルウェア
```go
// internal/middleware/security_headers.go
package middleware

import (
    "github.com/gin-gonic/gin"
)

func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        // XSS Protection
        c.Header("X-XSS-Protection", "1; mode=block")
        
        // Content Type Options
        c.Header("X-Content-Type-Options", "nosniff")
        
        // Frame Options
        c.Header("X-Frame-Options", "DENY")
        
        // Content Security Policy
        c.Header("Content-Security-Policy", 
            "default-src 'self'; "+
            "script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data: https:; "+
            "font-src 'self' https:; "+
            "connect-src 'self' https:; "+
            "frame-ancestors 'none';")
        
        // Referrer Policy
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        
        // Permissions Policy
        c.Header("Permissions-Policy", 
            "geolocation=(), microphone=(), camera=(), payment=()")
        
        // HSTS (HTTPS Strict Transport Security)
        c.Header("Strict-Transport-Security", 
            "max-age=31536000; includeSubDomains; preload")
        
        c.Next()
    }
}
```

## 6. 監査ログ

### 6.1 監査ログ実装
```go
// internal/audit/logger.go
package audit

import (
    "context"
    "encoding/json"
    "time"
    "github.com/trading-system/internal/database"
)

type AuditEvent struct {
    ID          uint      `json:"id"`
    UserID      uint      `json:"user_id"`
    Action      string    `json:"action"`
    Resource    string    `json:"resource"`
    ResourceID  string    `json:"resource_id"`
    Details     string    `json:"details"`
    IPAddress   string    `json:"ip_address"`
    UserAgent   string    `json:"user_agent"`
    Timestamp   time.Time `json:"timestamp"`
    Success     bool      `json:"success"`
}

type AuditLogger struct {
    db *database.DB
}

func NewAuditLogger(db *database.DB) *AuditLogger {
    return &AuditLogger{
        db: db,
    }
}

func (a *AuditLogger) LogEvent(ctx context.Context, event *AuditEvent) error {
    query := `
        INSERT INTO audit_logs 
        (user_id, action, resource, resource_id, details, ip_address, user_agent, timestamp, success)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
    
    _, err := a.db.ExecContext(ctx, query,
        event.UserID, event.Action, event.Resource, event.ResourceID,
        event.Details, event.IPAddress, event.UserAgent, event.Timestamp, event.Success)
    
    return err
}

func (a *AuditLogger) LogSecurityEvent(ctx context.Context, userID uint, action, details string, success bool) error {
    event := &AuditEvent{
        UserID:    userID,
        Action:    action,
        Resource:  "security",
        Details:   details,
        Timestamp: time.Now(),
        Success:   success,
    }
    
    return a.LogEvent(ctx, event)
}

func (a *AuditLogger) LogDataAccess(ctx context.Context, userID uint, resource, resourceID string, details map[string]interface{}) error {
    detailsJSON, _ := json.Marshal(details)
    
    event := &AuditEvent{
        UserID:     userID,
        Action:     "access",
        Resource:   resource,
        ResourceID: resourceID,
        Details:    string(detailsJSON),
        Timestamp:  time.Now(),
        Success:    true,
    }
    
    return a.LogEvent(ctx, event)
}

func (a *AuditLogger) LogDataModification(ctx context.Context, userID uint, action, resource, resourceID string, oldData, newData map[string]interface{}) error {
    details := map[string]interface{}{
        "old_data": oldData,
        "new_data": newData,
    }
    
    detailsJSON, _ := json.Marshal(details)
    
    event := &AuditEvent{
        UserID:     userID,
        Action:     action,
        Resource:   resource,
        ResourceID: resourceID,
        Details:    string(detailsJSON),
        Timestamp:  time.Now(),
        Success:    true,
    }
    
    return a.LogEvent(ctx, event)
}
```

## 7. セキュリティテスト

### 7.1 セキュリティテスト実装
```go
// tests/security/security_test.go
package security

import (
    "testing"
    "net/http"
    "net/http/httptest"
    "strings"
    "github.com/stretchr/testify/assert"
)

func TestSQLInjectionProtection(t *testing.T) {
    t.Run("SQLインジェクション攻撃", func(t *testing.T) {
        payloads := []string{
            "'; DROP TABLE strategies; --",
            "' OR '1'='1",
            "'; INSERT INTO users VALUES ('hacker', 'password'); --",
            "admin'--",
        }
        
        for _, payload := range payloads {
            req := httptest.NewRequest("POST", "/api/v1/strategies", 
                strings.NewReader(`{"name":"`+payload+`"}`))
            req.Header.Set("Content-Type", "application/json")
            
            w := httptest.NewRecorder()
            // アプリケーションのハンドラーを呼び出し
            
            // バリデーションエラーになることを確認
            assert.Equal(t, http.StatusBadRequest, w.Code)
        }
    })
}

func TestXSSProtection(t *testing.T) {
    t.Run("XSS攻撃", func(t *testing.T) {
        payloads := []string{
            "<script>alert('xss')</script>",
            "javascript:alert('xss')",
            "<img src=x onerror=alert('xss')>",
            "<iframe src=javascript:alert('xss')></iframe>",
        }
        
        for _, payload := range payloads {
            req := httptest.NewRequest("POST", "/api/v1/strategies", 
                strings.NewReader(`{"description":"`+payload+`"}`))
            req.Header.Set("Content-Type", "application/json")
            
            w := httptest.NewRecorder()
            // アプリケーションのハンドラーを呼び出し
            
            // エスケープされていることを確認
            assert.NotContains(t, w.Body.String(), "<script>")
            assert.NotContains(t, w.Body.String(), "javascript:")
        }
    })
}

func TestCSRFProtection(t *testing.T) {
    t.Run("CSRFトークン検証", func(t *testing.T) {
        // CSRFトークンなしでリクエスト
        req := httptest.NewRequest("POST", "/api/v1/strategies", nil)
        w := httptest.NewRecorder()
        
        // アプリケーションのハンドラーを呼び出し
        
        // 403エラーになることを確認
        assert.Equal(t, http.StatusForbidden, w.Code)
    })
}

func TestAuthentication(t *testing.T) {
    t.Run("無効なJWTトークン", func(t *testing.T) {
        req := httptest.NewRequest("GET", "/api/v1/strategies", nil)
        req.Header.Set("Authorization", "Bearer invalid-token")
        
        w := httptest.NewRecorder()
        // アプリケーションのハンドラーを呼び出し
        
        // 401エラーになることを確認
        assert.Equal(t, http.StatusUnauthorized, w.Code)
    })
}
```

## 8. セキュリティ設定

### 8.1 セキュリティ設定ファイル
```yaml
# config/security.yaml
security:
  jwt:
    secret_key: "${JWT_SECRET_KEY}"
    expiration_hours: 24
    refresh_expiration_hours: 168  # 7 days
  
  encryption:
    key: "${ENCRYPTION_KEY}"
    algorithm: "AES-256-GCM"
  
  password:
    min_length: 8
    require_uppercase: true
    require_lowercase: true
    require_numbers: true
    require_special: true
    max_age_days: 90
  
  totp:
    issuer: "Trading System"
    algorithm: "SHA1"
    digits: 6
    period: 30
  
  rate_limiting:
    requests_per_minute: 100
    burst_size: 20
  
  session:
    timeout_minutes: 30
    max_sessions_per_user: 5
  
  audit:
    enabled: true
    retention_days: 365
    sensitive_fields:
      - "password"
      - "api_key"
      - "secret_key"
      - "token"
  
  headers:
    hsts_max_age: 31536000
    csp_policy: "default-src 'self'"
    x_frame_options: "DENY"
    x_content_type_options: "nosniff"
    x_xss_protection: "1; mode=block"
  
  vault:
    enabled: true
    provider: "oci"
    vault_id: "${OCI_VAULT_ID}"
    compartment_id: "${OCI_COMPARTMENT_ID}"
    key_id: "${OCI_KEY_ID}"
```

### 8.2 環境変数設定
```bash
# .env.example
# JWT設定
JWT_SECRET_KEY=your-super-secret-jwt-key-32-bytes-long
JWT_EXPIRATION_HOURS=24

# 暗号化設定
ENCRYPTION_KEY=your-32-byte-encryption-key-here

# OCI Vault設定
OCI_VAULT_ID=ocid1.vault.oc1.region.vault-id
OCI_COMPARTMENT_ID=ocid1.compartment.oc1.compartment-id
OCI_KEY_ID=ocid1.key.oc1.region.key-id
OCI_USER_ID=ocid1.user.oc1.user-id
OCI_TENANCY_ID=ocid1.tenancy.oc1.tenancy-id
OCI_FINGERPRINT=your-api-key-fingerprint
OCI_PRIVATE_KEY_PATH=/path/to/private-key.pem

# セキュリティ設定
PASSWORD_MIN_LENGTH=8
RATE_LIMIT_REQUESTS_PER_MINUTE=100
SESSION_TIMEOUT_MINUTES=30
AUDIT_RETENTION_DAYS=365
```