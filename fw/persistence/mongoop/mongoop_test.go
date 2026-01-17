package mongoop

import (
	"context"
	"testing"
	"time"

	"github.com/donnie4w/go-logger/logger"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// skipIfShort 在 -short 模式下跳过需要 MongoDB 连接的测试
func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过：需要 MongoDB 连接（使用 -short 模式）")
	}
}

func TestNewClient(t *testing.T) {
	skipIfShort(t)
	//mongodb://xxz:123456@localhost:26001/?authMechanism=SCRAM-SHA-256&authSource=myAuthDB
	client, err := NewMongoClient(Conf{
		Url:           "mongodb://localhost:26001",
		ConnTimeout:   "3s",
		User:          "xxz",
		Password:      "123456",
		AuthMechanism: "SCRAM-SHA-256",
		Database:      "myAuthDB",
	})
	require.NoError(t, err)
	require.NoError(t, client.DisConnect(nil))
}
func TestConnectPool(t *testing.T) {
	skipIfShort(t)
	client, err := NewMongoClient(Conf{
		Url:         "mongodb://localhost:26001",
		ConnTimeout: "3s",
	})
	require.NoError(t, err)

	require.NoError(t, client.DisConnect(nil))
}
func TestUpdateOne(t *testing.T) {
	skipIfShort(t)
	client, err := NewMongoClient(Conf{
		Url: "mongodb://localhost:26001",
		//Url:         "mongodb://10.0.2.10:26001",
		ConnTimeout: "3s",
	})
	require.NoError(t, err)

	defer func() {
		require.NoError(t, client.DisConnect(nil))
	}()

	collection := client.C.Database("xxz_map_100008").Collection("test")
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	ctx := context.Background()

	opts := options.Update().SetUpsert(true)
	filter := bson.D{{Key: "_id", Value: "map.hero.role_10002"}} //var id primitive.ObjectID
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "want.role.level", Value: 99}}}}
	//result, err := collection.UpdateByID(ctx, id, bson.D{{"$set", bson.D{{"item_map.100.value", 1500}}}})
	result, err := collection.UpdateOne(ctx, filter, update, opts)
	require.NoError(t, err)

	if result.MatchedCount != 0 {
		logger.Info("matched and replaced an existing document")
		return
	}
	if result.UpsertedCount != 0 {
		logger.Infof("inserted a new document with ID %v", result.UpsertedID)
	}
}

func TestUpdateMany(t *testing.T) {
	skipIfShort(t)
	client, err := NewMongoClient(Conf{
		Url:         "mongodb://localhost:26001",
		ConnTimeout: "3s",
	})
	require.NoError(t, err)

	defer func() {
		require.NoError(t, client.DisConnect(nil))
	}()

	collection := client.C.Database("xproject").Collection("test")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Update().SetUpsert(true)
	type MongoStruct struct {
		ObjectID string      `bson:"_id"`
		Want     interface{} `bson:"want"`
	}
	rs := &MongoStruct{
		ObjectID: "role_10001",
		Want:     "you1",
	}
	filter := bson.D{{Key: "_id", Value: "role_10001"}}
	result, err := collection.UpdateMany(ctx, filter, bson.D{{Key: "$set", Value: rs}}, opts)
	require.NoError(t, err)

	if result.MatchedCount != 0 {
		logger.Info("matched and replaced an existing document")
		return
	}
	if result.UpsertedCount != 0 {
		logger.Infof("inserted a new document with ID %v", result.UpsertedID)
	}
}
func TestFindOne(t *testing.T) {
	skipIfShort(t)
	client, err := NewMongoClient(Conf{
		Url:         "mongodb://localhost:26001",
		ConnTimeout: "3s",
	})
	require.NoError(t, err)

	defer func() {
		require.NoError(t, client.DisConnect(nil))
	}()

	collection := client.C.Database("xproject").Collection("role")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := "Role_102"
	res := collection.FindOne(ctx, bson.D{{Key: "_id", Value: id}})
	if res.Err() != nil {
		if res.Err() == mongo.ErrNoDocuments {
			t.Log("not find  key")
			return
		}
		t.Fatal(res.Err())
	}
}

func TestNewClient2(t *testing.T) {
	skipIfShort(t)
	//mongodb://xxz:123456@localhost:26001/?authMechanism=SCRAM-SHA-256&authSource=myAuthDB
	client, err := NewMongoClient(Conf{
		Url:           "mongodb://44.224.185.95:10000",
		ConnTimeout:   "3s",
		User:          "admin",
		Password:      "jj1Ajda.hfdja88JJ_dppI",
		AuthMechanism: "SCRAM-SHA-256",
		//AuthSource:    "rel_xproject",
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, client.DisConnect(nil))
	}()

	collection := client.C.Database("xproject").Collection("test")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Update().SetUpsert(true)
	type MongoStruct struct {
		ObjectID string      `bson:"_id"`
		Want     interface{} `bson:"want"`
	}
	rs := &MongoStruct{
		ObjectID: "role_10001",
		Want:     "you1",
	}
	filter := bson.D{{Key: "_id", Value: "role_10001"}}
	result, err := collection.UpdateMany(ctx, filter, bson.D{{Key: "$set", Value: rs}}, opts)
	require.NoError(t, err)

	if result.MatchedCount != 0 {
		logger.Info("matched and replaced an existing document")
		return
	}
	if result.UpsertedCount != 0 {
		logger.Infof("inserted a new document with ID %v", result.UpsertedID)
	}
}
