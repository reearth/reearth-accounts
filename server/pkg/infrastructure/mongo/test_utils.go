package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Connect(t *testing.T) func(*testing.T) *mongo.Database {
	t.Helper()

	c, _ := mongo.Connect(
		context.Background(),
		options.Client().
			ApplyURI("mongodb://localhost").
			SetConnectTimeout(time.Second*10),
	)

	return func(t *testing.T) *mongo.Database {
		t.Helper()

		databaseName := "test" + "_" + uuid.NewString()
		t.Cleanup(func() {
			_ = c.Database(databaseName).Drop(context.Background())
		})
		return c.Database(databaseName)
	}
}
