package managers

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type PublishManager struct {
}

func NewPublishManager() *PublishManager {
	return &PublishManager{}
}

// Publish publishes a bundle to the correct key based on the bundle's version. In addition, it writes a json metadata file
// for looking up the latest bundle version
func (pm *PublishManager) Publish(ctx context.Context, source Source, namespace string, filenames []string) error {
	for _, filename := range filenames {
		bundleFile, err := NewBundleFromFile(filename)
		if err != nil {
			return errors.Wrapf(err, "couldn't open bundle '%s'", filename)
		}

		err = source.Put(bundleFile)
		if err != nil {
			return errors.Wrapf(err, "couldn't publish bundle '%s'", filename)
		}
		log.WithFields(log.Fields{"filename": filename, "bundle": bundleFile.Name, "version": bundleFile.Version, "size": bundleFile.Size}).Info("Published bundle")

		err = bundleFile.Close()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("couldn't close bundle")
		}
	}
	return nil
}
