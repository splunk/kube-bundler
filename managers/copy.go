/*
   Copyright 2023 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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
