package managers

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

type DeploySmoketestManager struct {
	kbClient     KBClient
	deployMgr    *DeployManager
	smoketestMgr *SmoketestManager
}

// NewDeploySmoketestManager will run a deploy followed by smoketests. Will print logs and errors to the screen
func NewDeploySmoketestManager(kbClient KBClient) *DeploySmoketestManager {
	return &DeploySmoketestManager{
		kbClient:     kbClient,
		deployMgr:    NewDeployManager(kbClient),
		smoketestMgr: NewSmoketestManager(kbClient),
	}
}

func (dsm *DeploySmoketestManager) DeploySmoketest(ctx context.Context, installRef InstallReference, showLogs bool, timeout time.Duration) error {
	// Run deploy
	deployOpts := DeployOpts{
		Action:  ActionApplyOutputs,
		Timeout: timeout,
	}
	err := dsm.deployMgr.Deploy(ctx, installRef, deployOpts, showLogs)
	if err != nil {
		return errors.Wrapf(err, "couldn't execute deploy for '%s'", installRef.Name)
	}

	// Run smoketests
	err = dsm.smoketestMgr.Smoketest(ctx, installRef, showLogs, timeout)
	if err != nil {
		return errors.Wrapf(err, "couldn't run smoketest for %q", installRef.Name)
	}

	return nil
}
