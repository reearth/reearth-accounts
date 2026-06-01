package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveDBDriver(t *testing.T) {
	assert.Equal(t, "mongo", (&Config{DB: "mongodb://localhost"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "mongodb+srv://x"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "postgres://u:p@h/db"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "postgresql://u:p@h/db"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "mongodb://x", DBDriver: "postgres"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "postgres://x", DBDriver: "mongo"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "mongodb://x", DBDriver: "postgresql"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "postgres://x", DBDriver: "mongodb"}).ResolveDBDriver())
	assert.Equal(t, "postgres", (&Config{DB: "mongodb://x", DBDriver: "Postgres"}).ResolveDBDriver())
	// unknown override falls through to scheme inference
	assert.Equal(t, "postgres", (&Config{DB: "postgres://x", DBDriver: "sqlite"}).ResolveDBDriver())
	assert.Equal(t, "mongo", (&Config{DB: "mongodb://x", DBDriver: "sqlite"}).ResolveDBDriver())
}
