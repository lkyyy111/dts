package db

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dsn string) error {
	var err error

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	if err := AutoMigrateAll(); err != nil {
		log.Printf("Failed to run database migrations: %v", err)
		return err
	}

	log.Printf("Database connected successfully!")
	return nil
}
