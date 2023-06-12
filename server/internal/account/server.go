package account

import "github.com/reearth/reearthx/log"

func StartServer() error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	log.Infof("config: %#v", config)

	// TODO

	return nil
}
