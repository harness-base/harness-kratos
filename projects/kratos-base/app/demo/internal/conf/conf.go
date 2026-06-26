// Package conf defines runtime configuration types for the demo service.
// Structure mirrors configs/runtime.yaml so yaml.Unmarshal works directly.
// IMPORTANT: duration fields are stored as strings (e.g. "300s") because
// Kratos config.Scan uses json.Unmarshal internally, which cannot parse
// duration strings into time.Duration. SelectPool handles the string→Duration
// conversion. All struct fields carry both yaml and json tags so that both
// direct yaml.Unmarshal and Kratos config.Scan work correctly.
package conf

import (
	"fmt"
	"time"

	"github.com/z-mate/kratos-base/pkg/mq/rabbitmq"
	"github.com/z-mate/kratos-base/pkg/mq/rocketmq"
	"github.com/z-mate/kratos-base/pkg/pgxpool"
	"github.com/z-mate/kratos-base/pkg/redisx"
)

// Runtime is the top-level runtime configuration structure.
// Tags carry both yaml (for direct yaml.Unmarshal in tests/bootstrap) and
// json (for Kratos config.Scan, which routes through json.Unmarshal internally).
type Runtime struct {
	Server ServerConfig `yaml:"server" json:"server"`
	Data   DataConfig   `yaml:"data"   json:"data"`
	Log    LogConfig    `yaml:"log"    json:"log"`
	Trace  TraceConfig  `yaml:"trace"  json:"trace"`
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	GRPC struct {
		Addr string `yaml:"addr" json:"addr"`
	} `yaml:"grpc" json:"grpc"`
	HTTP struct {
		Addr string `yaml:"addr" json:"addr"`
	} `yaml:"http" json:"http"`
}

// RedisConfig holds Redis connection configuration for the data layer.
// Duration fields are stored as strings (e.g. "5s") and parsed by SelectRedis,
// because Kratos config.Scan routes through json.Unmarshal which cannot decode
// "5s"-style strings into time.Duration directly.
type RedisConfig struct {
	Addrs        []string `yaml:"addrs"          json:"addrs"`
	Username     string   `yaml:"username"       json:"username"`
	Password     string   `yaml:"password"       json:"password"`
	DB           int      `yaml:"db"             json:"db"`
	PoolSize     int      `yaml:"pool_size"      json:"pool_size"`
	MaxRetries   int      `yaml:"max_retries"    json:"max_retries"`
	DialTimeout  string   `yaml:"dial_timeout"   json:"dial_timeout"`
	ReadTimeout  string   `yaml:"read_timeout"   json:"read_timeout"`
	WriteTimeout string   `yaml:"write_timeout"  json:"write_timeout"`
	EnableTLS    bool     `yaml:"enable_tls"     json:"enable_tls"`
}

// RabbitmqConfig holds RabbitMQ connection configuration for the data layer.
// DialTimeout is stored as a string (e.g. "5s") and parsed by SelectMQ,
// because Kratos config.Scan routes through json.Unmarshal.
type RabbitmqConfig struct {
	URL         string `yaml:"url"          json:"url"`
	DialTimeout string `yaml:"dial_timeout" json:"dial_timeout"`
}

// RocketmqConfig holds RocketMQ connection configuration for the data layer.
// RequestTimeout and AwaitDuration are stored as strings and parsed by SelectMQ.
type RocketmqConfig struct {
	Endpoint       string `yaml:"endpoint"        json:"endpoint"`
	AccessKey      string `yaml:"access_key"      json:"access_key"`
	SecretKey      string `yaml:"secret_key"      json:"secret_key"`
	ConsumerGroup  string `yaml:"consumer_group"  json:"consumer_group"`
	AwaitDuration  string `yaml:"await_duration"  json:"await_duration"`
	RequestTimeout string `yaml:"request_timeout" json:"request_timeout"`
	// EnableTLS controls whether the gRPC connection to proxy uses TLS.
	// Set to false for sandbox/dev environments running a plain-text proxy.
	EnableTLS bool `yaml:"enable_tls" json:"enable_tls"`
}

// MQConfig holds message-queue configuration for the data layer.
// Kind selects the broker backend: "rabbitmq", "rocketmq", or "" (disabled).
type MQConfig struct {
	// Kind selects the broker backend: "rabbitmq" | "rocketmq" | "" (not enabled).
	Kind string `yaml:"kind"     json:"kind"`
	// Topic is the business topic/queue name. Defaults to "demo.events".
	Topic    string         `yaml:"topic"    json:"topic"`
	Rabbitmq RabbitmqConfig `yaml:"rabbitmq" json:"rabbitmq"`
	Rocketmq RocketmqConfig `yaml:"rocketmq" json:"rocketmq"`
}

// DataConfig holds data-layer configuration.
// Duration fields are stored as strings (e.g. "300s", "5m") and parsed by
// SelectPool, because Kratos config.Scan routes through json.Unmarshal which
// cannot decode "300s"-style strings into time.Duration directly.
type DataConfig struct {
	Database struct {
		DSN             string `yaml:"dsn"               json:"dsn"`
		MaxOpen         int    `yaml:"max_open"          json:"max_open"`
		MaxIdle         int    `yaml:"max_idle"          json:"max_idle"`
		ConnMaxLifetime string `yaml:"conn_max_lifetime"  json:"conn_max_lifetime"`
		ConnMaxIdleTime string `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
		ConnectTimeout  string `yaml:"connect_timeout"   json:"connect_timeout"`
	} `yaml:"database" json:"database"`
	Redis RedisConfig `yaml:"redis" json:"redis"`
	MQ    MQConfig    `yaml:"mq"    json:"mq"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string `yaml:"level" json:"level"`
}

// TraceConfig holds tracing configuration.
type TraceConfig struct {
	Endpoint    string  `yaml:"endpoint"     json:"endpoint"`
	SampleRatio float64 `yaml:"sample_ratio" json:"sample_ratio"`
}

// Validate performs basic validation on the Runtime config.
// Returns an error if required fields are missing or invalid.
func Validate(r Runtime) error {
	if r.Server.GRPC.Addr == "" {
		return fmt.Errorf("conf: server.grpc.addr must not be empty")
	}
	if r.Server.HTTP.Addr == "" {
		return fmt.Errorf("conf: server.http.addr must not be empty")
	}
	switch r.Data.MQ.Kind {
	case "", "rabbitmq", "rocketmq":
		// valid
	default:
		return fmt.Errorf("conf: data.mq.kind %q is invalid; must be \"rabbitmq\", \"rocketmq\", or \"\" (disabled)", r.Data.MQ.Kind)
	}
	return nil
}

// parseDuration parses a duration string (e.g. "300s", "5m", "0").
// Returns 0 for empty strings (uses driver default).
func parseDuration(s string) (time.Duration, error) {
	if s == "" || s == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("conf: parse duration %q: %w", s, err)
	}
	return d, nil
}

// SelectPool extracts a pgxpool.PoolConfig from a Runtime config value.
// cfg must be a conf.Runtime; returns an error if the type assertion fails.
// Duration fields are stored as strings in the config and parsed here.
func SelectPool(cfg any) (pgxpool.PoolConfig, error) {
	r, ok := cfg.(Runtime)
	if !ok {
		return pgxpool.PoolConfig{}, fmt.Errorf("conf: SelectPool: expected conf.Runtime, got %T", cfg)
	}
	db := r.Data.Database

	connMaxLifetime, err := parseDuration(db.ConnMaxLifetime)
	if err != nil {
		return pgxpool.PoolConfig{}, err
	}
	connMaxIdleTime, err := parseDuration(db.ConnMaxIdleTime)
	if err != nil {
		return pgxpool.PoolConfig{}, err
	}
	connectTimeout, err := parseDuration(db.ConnectTimeout)
	if err != nil {
		return pgxpool.PoolConfig{}, err
	}

	return pgxpool.PoolConfig{
		DSN:             db.DSN,
		MaxOpen:         db.MaxOpen,
		MaxIdle:         db.MaxIdle,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
		ConnectTimeout:  connectTimeout,
	}, nil
}

// MQSelection is the result of SelectMQ; carries the resolved broker configs
// and the effective topic name.
type MQSelection struct {
	Kind   string
	Rabbit rabbitmq.Config
	Rocket rocketmq.Config
	Topic  string
}

// SelectMQ extracts broker configuration from a Runtime config value.
// cfg must be a conf.Runtime; returns an error if the type assertion fails or
// duration fields are malformed.
// When Kind is "" the returned MQSelection has Kind=="" and both broker configs
// are zero-valued — callers should treat this as "MQ not enabled".
func SelectMQ(cfg any) (MQSelection, error) {
	r, ok := cfg.(Runtime)
	if !ok {
		return MQSelection{}, fmt.Errorf("conf: SelectMQ: expected conf.Runtime, got %T", cfg)
	}
	mq := r.Data.MQ

	topic := mq.Topic
	if topic == "" {
		topic = "demo.events"
	}

	sel := MQSelection{Kind: mq.Kind, Topic: topic}

	switch mq.Kind {
	case "":
		// Not enabled; return zero-valued configs.
		return sel, nil
	case "rabbitmq":
		dt, err := parseDuration(mq.Rabbitmq.DialTimeout)
		if err != nil {
			return MQSelection{}, fmt.Errorf("conf: SelectMQ rabbitmq.dial_timeout: %w", err)
		}
		sel.Rabbit = rabbitmq.Config{
			URL:         mq.Rabbitmq.URL,
			DialTimeout: dt,
		}
	case "rocketmq":
		await, err := parseDuration(mq.Rocketmq.AwaitDuration)
		if err != nil {
			return MQSelection{}, fmt.Errorf("conf: SelectMQ rocketmq.await_duration: %w", err)
		}
		reqTimeout, err := parseDuration(mq.Rocketmq.RequestTimeout)
		if err != nil {
			return MQSelection{}, fmt.Errorf("conf: SelectMQ rocketmq.request_timeout: %w", err)
		}
		sel.Rocket = rocketmq.Config{
			Endpoint:       mq.Rocketmq.Endpoint,
			AccessKey:      mq.Rocketmq.AccessKey,
			SecretKey:      mq.Rocketmq.SecretKey,
			ConsumerGroup:  mq.Rocketmq.ConsumerGroup,
			AwaitDuration:  await,
			RequestTimeout: reqTimeout,
			EnableTLS:      mq.Rocketmq.EnableTLS,
		}
	}
	return sel, nil
}

// SelectMQFlat is the flattened form of SelectMQ that satisfies data.New's
// selectMQ parameter signature:
//
//	func(cfg any) (kind string, rabbit rabbitmq.Config, rocket rocketmq.Config, topic string, err error)
//
// It is provided separately so wire can match the exact function type without
// needing a closure.
func SelectMQFlat(cfg any) (string, rabbitmq.Config, rocketmq.Config, string, error) {
	sel, err := SelectMQ(cfg)
	return sel.Kind, sel.Rabbit, sel.Rocket, sel.Topic, err
}

// SelectRedis extracts a redisx.Config from a Runtime config value.
// cfg must be a conf.Runtime; returns an error if the type assertion fails.
// Duration fields are stored as strings in the config and parsed here.
func SelectRedis(cfg any) (redisx.Config, error) {
	r, ok := cfg.(Runtime)
	if !ok {
		return redisx.Config{}, fmt.Errorf("conf: SelectRedis: expected conf.Runtime, got %T", cfg)
	}
	rc := r.Data.Redis

	dialTimeout, err := parseDuration(rc.DialTimeout)
	if err != nil {
		return redisx.Config{}, err
	}
	readTimeout, err := parseDuration(rc.ReadTimeout)
	if err != nil {
		return redisx.Config{}, err
	}
	writeTimeout, err := parseDuration(rc.WriteTimeout)
	if err != nil {
		return redisx.Config{}, err
	}

	return redisx.Config{
		Addrs:        rc.Addrs,
		Username:     rc.Username,
		Password:     rc.Password,
		DB:           rc.DB,
		PoolSize:     rc.PoolSize,
		MaxRetries:   rc.MaxRetries,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		EnableTLS:    rc.EnableTLS,
	}, nil
}
