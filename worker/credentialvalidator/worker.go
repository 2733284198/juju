// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package credentialvalidator

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/juju/worker.v1"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/watcher"
	"github.com/juju/juju/worker/catacomb"
)

var logger = loggo.GetLogger("juju.api.credentialvalidator")

// ErrValidityChanged indicates that a Worker has bounced because its
// credential validity has changed: either a valid credential became invalid
// or invalid credential became valid.
var ErrValidityChanged = errors.New("cloud credential validity has changed")

// ErrModelMissingCredentialRequired indicates that a Worker has been bounced
// since the model does not have a cloud credential set but is on a cloud
// that requires authentication.
var ErrModelMissingCredentialRequired = errors.New("model has no credential set but is on the cloud that requires it")

// Facade exposes functionality required by a Worker to access and watch
// a cloud credential that a model uses.
type Facade interface {

	// ModelCredential gets model's cloud credential.
	// Models that are on the clouds that do not require auth will return
	// false to signify that credential was not set.
	ModelCredential() (base.StoredCredential, bool, error)

	// WatchCredential gets cloud credential watcher.
	WatchCredential(string) (watcher.NotifyWatcher, error)
}

// Config holds the dependencies and configuration for a Worker.
type Config struct {
	Facade Facade
}

// Validate returns an error if the config cannot be expected to
// drive a functional Worker.
func (config Config) Validate() error {
	if config.Facade == nil {
		return errors.NotValidf("nil Facade")
	}
	return nil
}

// NewWorker returns a Worker that tracks the validity of the Model's cloud
// credential, as exposed by the Facade.
func NewWorker(config Config) (worker.Worker, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}

	mc, err := modelCredential(config.Facade)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// It is possible that this model is on a cloud that does not
	// necessarily require authentication. Consequently, a model
	// is created without a set credential. However, later on,
	// a credential may be added to a model. This worker will need to
	// be restarted as there will be no credential watcher set to react to
	// changes.
	// TODO (anastasiamac 2018-05-30) when model-credential relationship no
	// longer resides on model itself but has a dedicated document in mongo,
	// we can have a watcher that will support the above corner case.

	v := &validator{
		validatorFacade: config.Facade,
		credential:      mc,
	}

	plan := catacomb.Plan{
		Site: &v.catacomb,
		Work: v.loop,
	}

	if mc.CloudCredential != "" {
		var err error
		v.watcher, err = config.Facade.WatchCredential(mc.CloudCredential)
		if err != nil {
			return nil, errors.Trace(err)
		}
		// The watcher needs to be added to the worker's catacomb plan
		// here in order to be controlled by this worker's lifecycle events:
		// for example, to be destroyed when this worker is destroyed, etc.
		// We also add the watcher to the Plan.Init collection to ensure that
		// the worker's Plan.Work method is executed after the watcher
		// is initialised and watcher's changes collection obtains the changes.
		// Watchers that are added using catacomb.Add method
		// miss out on a first call of Worker's Plan.Work method and can, thus,
		// be missing out on an initial change.
		plan.Init = []worker.Worker{v.watcher}
	}

	if err := catacomb.Invoke(plan); err != nil {
		return nil, errors.Trace(err)
	}
	return v, nil
}

type validator struct {
	catacomb catacomb.Catacomb

	credential base.StoredCredential

	// watcher may sometimes be nil when there is no credential to watch.
	watcher watcher.NotifyWatcher

	validatorFacade Facade
}

// Kill is part of the worker.Worker interface.
func (v *validator) Kill() {
	v.catacomb.Kill(nil)
}

// Wait is part of the worker.Worker interface.
func (v *validator) Wait() error {
	return v.catacomb.Wait()
}

// Check is part of the util.Flag interface.
func (v *validator) Check() bool {
	return v.credential.Valid
}

func (v *validator) loop() error {
	var watcherChanges watcher.NotifyChannel
	if v.watcher != nil {
		watcherChanges = v.watcher.Changes()
	}

	for {
		select {
		case <-v.catacomb.Dying():
			return v.catacomb.ErrDying()
		case _, ok := <-watcherChanges:
			if !ok {
				return v.catacomb.ErrDying()
			}
			updatedCredential, err := modelCredential(v.validatorFacade)
			if err != nil {
				return errors.Trace(err)
			}
			// TODO (anastasiamac 2018-05-31) model's reference to cloud credential
			// is immutable at this stage. Planned worked caters for situations
			// where model credential is changed.
			// Once this is implemented, this worker will need to be changed too.
			if v.credential.Valid != updatedCredential.Valid {
				return ErrValidityChanged
			}
		}
	}
}

func modelCredential(v Facade) (base.StoredCredential, error) {
	mc, exists, err := v.ModelCredential()
	if err != nil {
		return base.StoredCredential{}, errors.Trace(err)
	}
	if !exists && !mc.Valid {
		logger.Warningf("model credential is not set for the model but the cloud requires it")
		// In this situation, where a model credential is not set and the model
		// is on the cloud that requires a credential, we want the watcher to restart
		// in hopes a new valid credential has been set on the model.
		return base.StoredCredential{}, ErrModelMissingCredentialRequired
	}
	return mc, nil
}
