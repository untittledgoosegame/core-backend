package config

import (
	"errors"
	"io/fs"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"

	"github.com/untittledgoosegame/core-backend/pkg/cache"
	"github.com/untittledgoosegame/core-backend/pkg/logger"
)

type Config struct {
	Env          string             `mapstructure:"env"`
	Log          logger.Config      `mapstructure:"log"`
	App          AppSettings        `mapstructure:"app"`
	Server       ServerConfig       `mapstructure:"server"`
	Mongo        MongoConfig        `mapstructure:"mongo"`
	Redis        cache.Config       `mapstructure:"redis"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	WebAuthn     WebAuthnConfig     `mapstructure:"webauthn"`
	Truecaller   TruecallerConfig   `mapstructure:"truecaller"`
	SMTP         SMTPConfig         `mapstructure:"smtp"`
	Security     SecurityConfig     `mapstructure:"security"`
	RateLimit    RateLimitConfig    `mapstructure:"rate_limit"`
	Mobile       MobileConfig       `mapstructure:"mobile"`
	OAuthService OAuthServiceConfig `mapstructure:"oauth_service"`
}

type AppSettings struct {
	Name string `mapstructure:"name"`
}

type ServerConfig struct {
	Host           string   `mapstructure:"host"`
	Port           string   `mapstructure:"port"`
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

type MongoConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	TTL    time.Duration `mapstructure:"ttl"`
}

type WebAuthnConfig struct {
	RPID      string   `mapstructure:"rp_id"`
	RPOrigins []string `mapstructure:"rp_origins"`
}

type TruecallerConfig struct {
	ClientID string `mapstructure:"client_id"`
	BaseURL  string `mapstructure:"base_url"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

type SecurityConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

type RateLimitConfig struct {
	GlobalPerMinute int `mapstructure:"global_per_minute"`
	AuthPerMinute   int `mapstructure:"auth_per_minute"`
}

type MobileConfig struct {
	AndroidPackage          string   `mapstructure:"android_package"`
	AndroidCertFingerprints []string `mapstructure:"android_cert_fingerprints"`
	IOSAppIDs               []string `mapstructure:"ios_app_ids"`
}

type OAuthServiceConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Secret  string `mapstructure:"secret"`
}

var AppConfig Config

const devJWTSecret = "dev-only-jwt-secret"

func (c Config) IsDev() bool { return strings.EqualFold(c.Env, "dev") }

func LoadConfig() error {
	switch err := gotenv.Load(); {
	case err == nil:
		log.Info("loaded settings from .env")
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.SetDefault("env", "production")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")
	viper.SetDefault("app.name", "Goose")
	viper.SetDefault("server.host", "127.0.0.1")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.trusted_proxies", []string{})
	viper.SetDefault("mongo.uri", "mongodb://root:root@localhost:27017")
	viper.SetDefault("mongo.database", "goose")
	viper.SetDefault("redis.url", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("jwt.secret", "")
	viper.SetDefault("jwt.ttl", "720h")
	viper.SetDefault("webauthn.rp_id", "localhost")
	viper.SetDefault("webauthn.rp_origins", []string{"http://localhost:3000"})
	viper.SetDefault("truecaller.client_id", "")
	viper.SetDefault("truecaller.base_url", "https://oauth-account-noneu.truecaller.com")
	viper.SetDefault("smtp.host", "")
	viper.SetDefault("smtp.port", "587")
	viper.SetDefault("smtp.username", "")
	viper.SetDefault("smtp.password", "")
	viper.SetDefault("smtp.from", "")
	viper.SetDefault("security.encryption_key", "")
	viper.SetDefault("rate_limit.global_per_minute", 120)
	viper.SetDefault("rate_limit.auth_per_minute", 10)
	viper.SetDefault("mobile.android_package", "")
	viper.SetDefault("mobile.android_cert_fingerprints", []string{})
	viper.SetDefault("mobile.ios_app_ids", []string{})
	viper.SetDefault("oauth_service.base_url", "")
	viper.SetDefault("oauth_service.secret", "")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var missing viper.ConfigFileNotFoundError
		if !errors.As(err, &missing) {
			return err
		}
	} else {
		log.Infof("loaded settings from %s", viper.ConfigFileUsed())
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		return err
	}

	if AppConfig.JWT.Secret == "" {
		if !AppConfig.IsDev() {
			return errors.New("jwt.secret (JWT_SECRET) is required outside dev")
		}
		AppConfig.JWT.Secret = devJWTSecret
	}
	if AppConfig.Security.EncryptionKey == "" && !AppConfig.IsDev() {
		return errors.New("security.encryption_key (SECURITY_ENCRYPTION_KEY) is required outside dev")
	}
	return nil
}
