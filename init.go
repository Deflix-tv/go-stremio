package stremio

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	// Configure logging (except for level, which we only know from the config which is obtained later).
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}
