package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

var (
	config Config
	once   sync.Once
)

// START CUSTOMIZING - Added Config for Backup Clearer  -----------------------------------------

type Config struct {
	LogLevel           string `json:"log_level"`
	LoggingTimestamp   bool   `json:"logging_timestamp"`
	Username           string
	Password           string
	DisableWeb         bool `json:"disable_web"`
	DisableMetrics     bool `json:"disable_metrics"`
	UnprotectedMetrics bool `json:"unprotected_metrics"`
	S3                 S3Config
	Services           map[string]ServiceConfig
	Foreground         bool
	BackupCleaner      BackupCleaner `json:"backup_cleaner"`
}

type BackupCleaner struct {
	Enabled            bool   `json:"enabled"`
	DefaultEndHours    int    `json:"default_end_hours"`
	DefaultStartHours  int    `json:"default_start_hours"`
	OutdatedEndHours   int    `json:"outdated_end_hours"`
	OutdatedStartHours int    `json:"outdated_start_hours"`
	Group              string `json:"group"`
	Type               string `json:"type"`
	Instance           string `json:"instance"`
	EsDateHourPattern  string `json:"es_datehour_pattern"`
}

// STOP CUSTOMIZING -----------------------------------------------------------------------------

type S3Config struct {
	DisableSSL    bool   `json:"disable_ssl"`
	ServiceLabel  string `json:"service_label"`
	ServiceName   string `json:"service_name"`
	BucketName    string `json:"bucket_name"`
	EncryptionKey string `json:"encryption_key"`
}

type ServiceConfig struct {
	Schedule  string
	Timeout   TimeoutDuration
	Retention struct {
		Days  int
		Files int
	}
	DisableColumnStatistics bool `json:"disable_column_statistics"`
}

type TimeoutDuration struct {
	time.Duration
}

func (td TimeoutDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(td.String())
}

func (td *TimeoutDuration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		td.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		td.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}

func Get() *Config {
	once.Do(func() {
		// initialize
		config = Config{
			Services: make(map[string]ServiceConfig),
		}

		// first load config file, if it exists
		if _, err := os.Stat("config.json"); err == nil {
			data, err := ioutil.ReadFile("config.json")
			if err != nil {
				log.Println("could not load 'config.json'")
				log.Fatalln(err.Error())
			}
			if err := json.Unmarshal(data, &config); err != nil {
				log.Println("could not parse 'config.json'")
				log.Fatalln(err.Error())
			}
		}

		// now load & overwrite with env provided config, if it exists
		env := os.Getenv("BACKMAN_CONFIG")
		if len(env) > 0 {
			envConfig := Config{}
			if err := json.Unmarshal([]byte(env), &envConfig); err != nil {
				log.Println("could not parse environment variable 'BACKMAN_CONFIG'")
				log.Fatalln(err.Error())
			}

			// merge config values
			if len(envConfig.LogLevel) > 0 {
				config.LogLevel = envConfig.LogLevel
			}
			if envConfig.LoggingTimestamp {
				config.LoggingTimestamp = envConfig.LoggingTimestamp
			}
			if len(envConfig.Username) > 0 {
				config.Username = envConfig.Username
			}
			if len(envConfig.Password) > 0 {
				config.Password = envConfig.Password
			}
			if envConfig.DisableWeb {
				config.DisableWeb = envConfig.DisableWeb
			}
			if envConfig.DisableMetrics {
				config.DisableMetrics = envConfig.DisableMetrics
			}
			if envConfig.UnprotectedMetrics {
				config.UnprotectedMetrics = envConfig.UnprotectedMetrics
			}
			if envConfig.S3.DisableSSL {
				config.S3.DisableSSL = envConfig.S3.DisableSSL
			}
			if len(envConfig.S3.ServiceLabel) > 0 {
				config.S3.ServiceLabel = envConfig.S3.ServiceLabel
			}
			if len(envConfig.S3.ServiceName) > 0 {
				config.S3.ServiceName = envConfig.S3.ServiceName
			}
			if len(envConfig.S3.BucketName) > 0 {
				config.S3.BucketName = envConfig.S3.BucketName
			}
			if len(envConfig.S3.EncryptionKey) > 0 {
				config.S3.EncryptionKey = envConfig.S3.EncryptionKey
			}
			for serviceName, serviceConfig := range envConfig.Services {
				mergedServiceConfig := config.Services[serviceName]
				if len(serviceConfig.Schedule) > 0 {
					mergedServiceConfig.Schedule = serviceConfig.Schedule
				}
				if serviceConfig.Timeout.Seconds() > 1 {
					mergedServiceConfig.Timeout = serviceConfig.Timeout
				}
				if serviceConfig.Retention.Days > 0 {
					mergedServiceConfig.Retention.Days = serviceConfig.Retention.Days
				}
				if serviceConfig.Retention.Files > 0 {
					mergedServiceConfig.Retention.Files = serviceConfig.Retention.Files
				}
				if serviceConfig.DisableColumnStatistics {
					mergedServiceConfig.DisableColumnStatistics = serviceConfig.DisableColumnStatistics
				}
				config.Services[serviceName] = mergedServiceConfig
			}

			// START CUSTOMIZING - Added Config for Backup Clearer  -----------------------------------------
			if envConfig.BackupCleaner.Enabled {
				config.BackupCleaner.Enabled = envConfig.BackupCleaner.Enabled
			}
			if envConfig.BackupCleaner.DefaultStartHours > 0 {
				config.BackupCleaner.DefaultStartHours = envConfig.BackupCleaner.DefaultStartHours
			}
			if envConfig.BackupCleaner.DefaultEndHours > 0 {
				config.BackupCleaner.DefaultEndHours = envConfig.BackupCleaner.DefaultEndHours
			}
			if envConfig.BackupCleaner.OutdatedStartHours > 0 {
				config.BackupCleaner.OutdatedStartHours = envConfig.BackupCleaner.OutdatedStartHours
			}
			if envConfig.BackupCleaner.OutdatedEndHours > 0 {
				config.BackupCleaner.OutdatedEndHours = envConfig.BackupCleaner.OutdatedEndHours
			}
			if len(envConfig.BackupCleaner.Group) > 0 {
				config.BackupCleaner.Group = envConfig.BackupCleaner.Group
			}
			if len(envConfig.BackupCleaner.Type) > 0 {
				config.BackupCleaner.Type = envConfig.BackupCleaner.Type
			}
			if len(envConfig.BackupCleaner.Instance) > 0 {
				config.BackupCleaner.Instance = envConfig.BackupCleaner.Instance
			}
			if len(envConfig.BackupCleaner.EsDateHourPattern) > 0 {
				config.BackupCleaner.EsDateHourPattern = envConfig.BackupCleaner.EsDateHourPattern
			}
			// STOP CUSTOMIZING -----------------------------------------------------------------------------
		}

		// ensure we have default values
		if len(config.LogLevel) == 0 {
			config.LogLevel = "info"
		}
		if len(config.S3.ServiceLabel) == 0 {
			config.S3.ServiceLabel = "dynstrg"
		}

		// START CUSTOMIZING - Added Config for Backup Clearer  -----------------------------------------
		if len(config.BackupCleaner.Group) == 0 {
			config.BackupCleaner.Enabled = false
		}
		if len(config.BackupCleaner.Type) == 0 {
			config.BackupCleaner.Enabled = false
		}
		if len(config.BackupCleaner.Instance) == 0 {
			config.BackupCleaner.Enabled = false
		}
		// STOP CUSTOMIZING -----------------------------------------------------------------------------

		// use username & password from env if defined
		if len(os.Getenv("BACKMAN_USERNAME")) > 0 {
			config.Username = os.Getenv("BACKMAN_USERNAME")
		}
		if len(os.Getenv("BACKMAN_PASSWORD")) > 0 {
			config.Password = os.Getenv("BACKMAN_PASSWORD")
		}

		// use s3 encryption key from env if defined
		if len(os.Getenv("BACKMAN_ENCRYPTION_KEY")) > 0 {
			config.S3.EncryptionKey = os.Getenv("BACKMAN_ENCRYPTION_KEY")
		}
	})
	return &config
}
