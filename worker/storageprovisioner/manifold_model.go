// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package storageprovisioner

import (
	"github.com/juju/clock"
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"
	"gopkg.in/juju/worker.v1"
	"gopkg.in/juju/worker.v1/dependency"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/storageprovisioner"
	"github.com/juju/juju/storage"
	"github.com/juju/juju/worker/common"
)

// ModelManifoldConfig defines a storage provisioner's configuration and dependencies.
type ModelManifoldConfig struct {
	APICallerName       string
	ClockName           string
	StorageRegistryName string

	Model                        names.ModelTag
	StorageDir                   string
	NewCredentialValidatorFacade func(base.APICaller) (common.CredentialAPI, error)
	NewWorker                    func(config Config) (worker.Worker, error)
}

// ModelManifold returns a dependency.Manifold that runs a storage provisioner.
func ModelManifold(config ModelManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{config.APICallerName, config.ClockName, config.StorageRegistryName},
		Start: func(context dependency.Context) (worker.Worker, error) {

			var clock clock.Clock
			if err := context.Get(config.ClockName, &clock); err != nil {
				return nil, errors.Trace(err)
			}
			var apiCaller base.APICaller
			if err := context.Get(config.APICallerName, &apiCaller); err != nil {
				return nil, errors.Trace(err)
			}
			var registry storage.ProviderRegistry
			if err := context.Get(config.StorageRegistryName, &registry); err != nil {
				return nil, errors.Trace(err)
			}

			api, err := storageprovisioner.NewState(apiCaller)
			if err != nil {
				return nil, errors.Trace(err)
			}

			credentialAPI, err := config.NewCredentialValidatorFacade(apiCaller)
			if err != nil {
				return nil, errors.Trace(err)
			}
			w, err := config.NewWorker(Config{
				Model:            config.Model,
				Scope:            config.Model,
				StorageDir:       config.StorageDir,
				Applications:     api,
				Volumes:          api,
				Filesystems:      api,
				Life:             api,
				Registry:         registry,
				Machines:         api,
				Status:           api,
				Clock:            clock,
				CloudCallContext: common.NewCloudCallContext(credentialAPI, nil),
			})
			if err != nil {
				return nil, errors.Trace(err)
			}
			return w, nil
		},
	}
}
