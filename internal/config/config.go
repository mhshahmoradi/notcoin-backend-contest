package config

import (
    "fmt"
    "os"
    "strconv"
    "time"

    "github.com/joho/godotenv"
)

type Config struct {
    ServerPort int

    DBDriver         string
    DBDataSourceName string
    PostgresURL      string
    MigrationsDir    string

    RedisAddr     string
    RedisPassword string
    RedisDB       int
    RedisURL      string

    SaleCycleInterval time.Duration
    SaleDuration      time.Duration
    CodeTTLExpiry     time.Duration

    ItemsPerSale         int
    MaxItemsPerUser      int
}

func LoadConfig() (*Config, error) {
    if err := godotenv.Load(); err != nil {
        fmt.Printf("Warning: Could not load .env file from")
}

    config := &Config{}

    if port := os.Getenv("PORT"); port != "" {
        if p, err := strconv.Atoi(port); err == nil {
            config.ServerPort = p
        }
    }
    if config.ServerPort == 0 {
        config.ServerPort = 8032
    }

    config.DBDriver = "postgres"
    
    dbHost := getEnvOrDefault("NOTBACK_DB_HOST", "localhost")
    dbPort := getEnvOrDefault("NOTBACK_DB_PORT", "5432")
    dbName := getEnvOrDefault("NOTBACK_DB_DATABASE", "notDB")
    dbUser := getEnvOrDefault("NOTBACK_DB_USERNAME", "root")
    dbPassword := getEnvOrDefault("NOTBACK_DB_PASSWORD", "1234")
    
    config.DBDataSourceName = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", 
        dbUser, dbPassword, dbHost, dbPort, dbName)
    config.PostgresURL = config.DBDataSourceName
    config.MigrationsDir = getEnvOrDefault("MIGRATIONS_DIR", "migrations")

    redisHost := getEnvOrDefault("NOTBACK_REDIS_HOST", "localhost")
    redisPort := getEnvOrDefault("NOTBACK_REDIS_PORT", "6379")
    config.RedisAddr = fmt.Sprintf("%s:%s", redisHost, redisPort)
    config.RedisPassword = os.Getenv("NOTBACK_REDIS_PASSWORD")
    config.RedisURL = fmt.Sprintf("redis://%s", config.RedisAddr)

	config.SaleCycleInterval = time.Hour
	config.SaleDuration = time.Hour
	config.CodeTTLExpiry = 5 * time.Minute
    config.ItemsPerSale = 10000
    config.MaxItemsPerUser = 10

    return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
