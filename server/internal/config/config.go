package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Host string `mapstructure:"host"`
	Port int `mapstructure:"port"`
	DbHost string `mapstructure:"db_host"`
	DbPort int `mapstructure:"db_port"`
	DbUser string `mapstructure:"db_user"`
	DbPassword string `mapstructure:"db_password"`
	DbName string `mapstructure:"db_name"`
	DbSSLMode string `mapstructure:"db_sslmode"`
	SFHD string `mapstructure:"static_file_host_dir"`
}

var GlobalConfig Config

func InitConfig() {
	viper.SetConfigName("config")
	
	viper.SetConfigType("yaml")
	
	viper.AddConfigPath("internal/config")
	
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
}