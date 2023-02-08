package managers

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type CopyManager struct {
}

func NewCopyManager() *CopyManager {
	return &CopyManager{}
}

// Copy copies the required version of a bundle from one source to other
func (pm *CopyManager) Copy(ctx context.Context, fromSource, destinationSource Source, namespace string, bundleRefs []BundleRef) error {
	for _, bundleRef := range bundleRefs {

		bundleFile, err := fromSource.Get(bundleRef)
		if err != nil {
			return errors.Wrapf(err, "couldn't get bundle '%s'", bundleRef.Name)
		}
		log.WithFields(log.Fields{"filename": bundleFile.Filename(), "bundle": bundleRef.Name, "version": bundleFile.Version, "size": bundleFile.Size}).Info("Downloaded bundle")

		err = destinationSource.Put(bundleFile)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("couldn't copy bundle to destination source")
		}
		log.WithFields(log.Fields{"filename": bundleFile.Filename(), "bundle": bundleRef.Name, "version": bundleFile.Version, "size": bundleFile.Size}).Info("Downloaded bundle")

		err = bundleFile.Close()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("couldn't close bundle")
		}
	}
	return nil
}
