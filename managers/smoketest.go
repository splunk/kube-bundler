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
