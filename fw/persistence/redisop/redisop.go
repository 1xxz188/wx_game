package redisop

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisClientConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"passwd"`
}

func NewRedieClient(config RedisClientConfig) (*redis.Client, error) {
	if len(config.Addr) <= 0 {
		return nil, errors.New("addr empty")
	}

	if len(config.Password) <= 0 {
		return nil, errors.New("passwd empty")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       0,
	})

	if client == nil {
		return nil, errors.New("client is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, errors.Join(errors.New("ping failed: "), err)
	}

	return client, nil
}
