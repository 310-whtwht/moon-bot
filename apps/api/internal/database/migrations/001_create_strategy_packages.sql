-- Create strategy_packages table
CREATE TABLE IF NOT EXISTS strategy_packages (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    author VARCHAR(255) NOT NULL,
    is_public BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_name (name),
    INDEX idx_author (author),
    INDEX idx_is_public (is_public)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create strategy_versions table
CREATE TABLE IF NOT EXISTS strategy_versions (
    id VARCHAR(36) PRIMARY KEY,
    package_id VARCHAR(36) NOT NULL,
    version VARCHAR(50) NOT NULL,
    code TEXT NOT NULL,
    description TEXT NULL,
    is_active BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_package_version (package_id, version),
    INDEX idx_package_id (package_id),
    INDEX idx_is_active (is_active),
    
    FOREIGN KEY (package_id) REFERENCES strategy_packages(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create strategy_params table
CREATE TABLE IF NOT EXISTS strategy_params (
    id VARCHAR(36) PRIMARY KEY,
    version_id VARCHAR(36) NOT NULL,
    param_name VARCHAR(100) NOT NULL,
    param_type ENUM('string', 'number', 'boolean', 'array', 'object') NOT NULL,
    default_value TEXT NULL,
    description TEXT NULL,
    is_required BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_version_param (version_id, param_name),
    INDEX idx_version_id (version_id),
    
    FOREIGN KEY (version_id) REFERENCES strategy_versions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;