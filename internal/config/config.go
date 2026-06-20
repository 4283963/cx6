package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Redis  RedisConfig  `mapstructure:"redis"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Rank   RankConfig   `mapstructure:"rank"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type RedisConfig struct {
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	Password      string `mapstructure:"password"`
	DB            int    `mapstructure:"db"`
	PoolSize      int    `mapstructure:"pool_size"`
	MinIdleConns  int    `mapstructure:"min_idle_conns"`
	MaxRetries    int    `mapstructure:"max_retries"`
}

type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	Database        string `mapstructure:"database"`
	Charset         string `mapstructure:"charset"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

type RankConfig struct {
	LeaderboardKey string         `mapstructure:"leaderboard_key"`
	AntiCheat      AntiCheatConfig `mapstructure:"anti_cheat"`
	TopN           int            `mapstructure:"top_n"`
}

type AntiCheatConfig struct {
	MaxScorePerGame    int64 `mapstructure:"max_score_per_game"`
	MinScorePerGame    int64 `mapstructure:"min_score_per_game"`
	MaxUploadPerMinute int   `mapstructure:"max_upload_per_minute"`
	WindowSeconds      int   `mapstructure:"window_seconds"`
}

var AppConfig *Config

func Load(path string) error {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config file failed: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("unmarshal config failed: %w", err)
	}

	AppConfig = cfg
	log.Printf("config loaded successfully from %s", path)
	return nil
}

func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

func (m *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		m.User, m.Password, m.Host, m.Port, m.Database, m.Charset)
}
