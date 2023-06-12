package main

import (
	"github.com/reearth/reearth-account/internal/account"
	"github.com/reearth/reearthx/log"
)

func main() {
	if err := account.StartServer(); err != nil {
		log.Fatal(err)
	}
}
