// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machineundertaker

import (
	"github.com/juju/errors"
	"gopkg.in/juju/worker.v1"
	"gopkg.in/juju/worker.v1/dependency"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/machineundertaker"
	"github.com/juju/juju/api/watcher"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/worker/common"
)

// ManifoldConfig defines the machine undertaker's configuration and
// dependencies.
type ManifoldConfig struct {
	APICallerName string
	EnvironName   string

	NewWorker                    func(Facade, environs.Environ, common.CredentialAPI) (worker.Worker, error)
	NewCredentialValidatorFacade func(base.APICaller) (common.CredentialAPI, error)
}

// Manifold returns a dependency.Manifold that runs a machine
// undertaker.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{config.APICallerName, config.EnvironName},
		Start: func(context dependency.Context) (worker.Worker, error) {
			var apiCaller base.APICaller
			if err := context.Get(config.APICallerName, &apiCaller); err != nil {
				return nil, errors.Trace(err)
			}
			var environ environs.Environ
			if err := context.Get(config.EnvironName, &environ); err != nil {
				return nil, errors.Trace(err)
			}
			api, err := machineundertaker.NewAPI(apiCaller, watcher.NewNotifyWatcher)
			if err != nil {
				return nil, errors.Trace(err)
			}

			credentialAPI, err := config.NewCredentialValidatorFacade(apiCaller)
			if err != nil {
				return nil, errors.Trace(err)
			}

			w, err := config.NewWorker(api, environ, credentialAPI)
			if err != nil {
				return nil, errors.Trace(err)
			}
			return w, nil
		},
	}
}
