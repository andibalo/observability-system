package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port           string
	Environment    string
	ServiceName    string
	JaegerEndpoint string
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

	return &Config{
		Port:           viper.GetString("PORT"),
		Environment:    viper.GetString("ENVIRONMENT"),
		ServiceName:    viper.GetString("SERVICE_NAME"),
		JaegerEndpoint: viper.GetString("JAEGER_ENDPOINT"),
	}
}
