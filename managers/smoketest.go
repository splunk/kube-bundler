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
	"io"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type SmoketestManager struct {
	kbClient  KBClient
	deployMgr *DeployManager
}

func NewSmoketestManager(kbClient KBClient) *SmoketestManager {
	return &SmoketestManager{
		kbClient:  kbClient,
		deployMgr: NewDeployManager(kbClient),
	}
}

func (sm *SmoketestManager) Smoketest(ctx context.Context, installRef InstallReference, showLogs bool, timeout time.Duration) error {
	deployOpts := DeployOpts{
		Action:  ActionSmoketest,
		Timeout: timeout,
	}
	err := sm.deployMgr.Deploy(ctx, installRef, deployOpts, showLogs)
	if err != nil {
		return errors.Wrapf(err, "smoketest failed for '%s'", installRef.Name)
	}

	log.WithFields(log.Fields{"install": installRef.Name, "namespace": installRef.Namespace}).Info("Smoketest complete")
	return nil
}

func (sm *SmoketestManager) GetLogs(ctx context.Context, installRef InstallReference) (io.ReadCloser, error) {
	return sm.deployMgr.GetLogs(ctx, installRef)
}
