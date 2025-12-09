package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port                string
	Environment         string
	ServiceName         string
	DatabaseURL         string
	RabbitMQURL         string
	EnableBroker        bool
	WarehouseServiceURL string
	JaegerEndpoint      string
	MaxRetries          int
}

func Load() *Config {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./services/order-service")
	viper.AddConfigPath("../../")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No .env file found, using environment variables and defaults")
		} else {
			log.Printf("Error reading config file: %v", err)
		}
	}

	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("MAX_RETRIES", 3)
	viper.SetDefault("JAEGER_ENDPOINT", "localhost:4318")

	databaseURL := viper.GetString("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = buildDatabaseURL()
	}

	return &Config{
		Port:                viper.GetString("PORT"),
		Environment:         viper.GetString("ENVIRONMENT"),
		ServiceName:         viper.GetString("SERVICE_NAME"),
		DatabaseURL:         databaseURL,
		RabbitMQURL:         viper.GetString("RABBITMQ_URL"),
		EnableBroker:        viper.GetBool("ENABLE_BROKER"),
		WarehouseServiceURL: viper.GetString("WAREHOUSE_SERVICE_URL"),
		JaegerEndpoint:      viper.GetString("JAEGER_ENDPOINT"),
		MaxRetries:          viper.GetInt("MAX_RETRIES"),
	}
}

func buildDatabaseURL() string {
	host := viper.GetString("DB_HOST")
	port := viper.GetString("DB_PORT")
	user := viper.GetString("DB_USER")
	password := viper.GetString("DB_PASSWORD")
	dbname := viper.GetString("DB_NAME")
	sslmode := viper.GetString("DB_SSLMODE")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
}
