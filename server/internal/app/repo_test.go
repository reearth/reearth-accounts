package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMongoTransactionsAvailable(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{name: "srv scheme", uri: "mongodb+srv://cluster0.example.mongodb.net/db", want: true},
		{name: "comma-separated hosts", uri: "mongodb://host1:27017,host2:27017/db", want: true},
		{name: "single host with replicaSet param", uri: "mongodb://mongo:27017/db?replicaSet=rs0", want: true},
		{name: "single host, no replicaSet param", uri: "mongodb://localhost:27017/db", want: false},
		{name: "unparseable", uri: "://not a uri", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mongoTransactionsAvailable(tt.uri))
		})
	}
}

func TestRedactMongoURI(t *testing.T) {
	got := redactMongoURI("mongodb://user:pass@localhost:27017/db?replicaSet=rs0")
	assert.NotContains(t, got, "user")
	assert.NotContains(t, got, "pass")
	assert.Contains(t, got, "localhost:27017")

	assert.Equal(t, "(unparseable)", redactMongoURI("://not a uri"))
}
