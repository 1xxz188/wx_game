package mongoop

import (
	"context"
	"errors"
	"time"

	"github.com/donnie4w/go-logger/logger"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Conf struct {
	Url           string `mapstructure:"url"`
	ConnTimeout   string `mapstructure:"conntimeout"`
	AuthMechanism string `mapstructure:"authmechanism"` //"SCRAM-SHA-256",
	User          string `mapstructure:"user"`
	Password      string `mapstructure:"passwd"`
	MaxPoolSize   uint64 `mapstructure:"maxpoolsize"`
	IsAuthSource  bool   `mapstructure:"isauthsource"`
	Database      string `mapstructure:"dbbase"`
	FlushDbSec    int32  `mapstructure:"flushdbsec"`
}

type MongoClient struct {
	C   *mongo.Client
	Cfg Conf
}

func NewMongoClient(cfg Conf) (*MongoClient, error) {
	conTm, err := time.ParseDuration(cfg.ConnTimeout)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), conTm)
	defer cancel()

	op := options.Client()

	if cfg.User != "" && cfg.Password != "" {
		awsCredential := options.Credential{
			AuthMechanism: cfg.AuthMechanism,
			Username:      cfg.User,
			Password:      cfg.Password,
		}
		if cfg.IsAuthSource {
			awsCredential.AuthSource = cfg.Database
		}
		op.SetAuth(awsCredential)
	}
	op.SetMaxPoolSize(cfg.MaxPoolSize) //default=100
	//op.SetMaxConnecting(2) default=2
	//op.SetMinPoolSize(0) default=0
	c, err := mongo.Connect(ctx, op.ApplyURI(cfg.Url))
	if err != nil {
		return nil, err
	}

	err = c.Ping(ctx, readpref.Primary())
	if err != nil {
		_ = c.Disconnect(ctx)
		return nil, err
	}

	client := &MongoClient{
		C:   c,
		Cfg: cfg,
	}
	return client, nil
}

func NewMongoClientFromConfig() (*MongoClient, error) {
	var mongoCfg Conf
	err := viper.UnmarshalKey("Mongo", &mongoCfg)
	if err != nil {
		return nil, err
	}

	if len(mongoCfg.Url) <= 0 {
		return nil, errors.New("mongo url empty")
	}

	if len(mongoCfg.ConnTimeout) <= 0 {
		return nil, errors.New("mongo ConnTimeout empty")
	}

	if len(mongoCfg.AuthMechanism) <= 0 {
		return nil, errors.New("mongo AuthMechanism empty")
	}

	if len(mongoCfg.Database) <= 0 {
		return nil, errors.New("mongo Database empty")
	}

	//连接mongo
	mongoClient, err := NewMongoClient(mongoCfg)
	if err != nil {
		return nil, err
	}
	logger.Infof("mongoConn ok url[%s] Username[%s] ConnTimeout[%s] MaxPoolSize[%d] Database[%s] isAuthSource[%v]", mongoCfg.Url, mongoCfg.User, mongoCfg.ConnTimeout, mongoCfg.MaxPoolSize, mongoCfg.Database, mongoCfg.IsAuthSource)
	return mongoClient, nil
}

func (mc *MongoClient) DisConnect(ctx context.Context) error {
	return mc.C.Disconnect(ctx)
}
