package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port           string
	Environment    string
	ServiceName    string
	JaegerEndpoint string
	DatabaseURL    string
	RabbitMQURL    string
	EnableBroker   bool
	MaxRetries     int
}

func Load() *Config {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./services/warehouse-service")
	viper.AddConfigPath("../../")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No .env file found, using environment variables and defaults")
		} else {
			log.Printf("Error reading config file: %v", err)
		}
	}

	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8002")
	viper.SetDefault("SERVICE_NAME", "warehouse-service")
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("JAEGER_ENDPOINT", "localhost:4318")
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_NAME", "warehouse_db")
	viper.SetDefault("DB_USER", "postgres")
	viper.SetDefault("DB_PASSWORD", "postgres")
	viper.SetDefault("DB_SSLMODE", "disable")
	viper.SetDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	viper.SetDefault("ENABLE_BROKER", false)
	viper.SetDefault("MAX_RETRIES", 3)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		viper.GetString("DB_USER"),
		viper.GetString("DB_PASSWORD"),
		viper.GetString("DB_HOST"),
		viper.GetString("DB_PORT"),
		viper.GetString("DB_NAME"),
		viper.GetString("DB_SSLMODE"),
	)

	return &Config{
		Port:           viper.GetString("PORT"),
		Environment:    viper.GetString("ENVIRONMENT"),
		ServiceName:    viper.GetString("SERVICE_NAME"),
		JaegerEndpoint: viper.GetString("JAEGER_ENDPOINT"),
		DatabaseURL:    dbURL,
		RabbitMQURL:    viper.GetString("RABBITMQ_URL"),
		EnableBroker:   viper.GetBool("ENABLE_BROKER"),
		MaxRetries:     viper.GetInt("MAX_RETRIES"),
	}
}
