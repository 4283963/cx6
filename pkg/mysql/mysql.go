package mysql

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cx6/internal/config"
)

var DB *gorm.DB

func Init(cfg *config.MySQLConfig) error {
	var err error
	DB, err = gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("mysql connect failed: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB failed: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("mysql ping failed: %w", err)
	}

	log.Printf("mysql connected successfully at %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	return nil
}

func AutoMigrate(models ...interface{}) error {
	if DB == nil {
		return fmt.Errorf("mysql not initialized")
	}
	return DB.AutoMigrate(models...)
}

func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
