// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperatorprovisioner

import (
	"github.com/juju/errors"
	"github.com/juju/version"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/common"
	apiwatcher "github.com/juju/juju/api/watcher"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/watcher"
)

// Client allows access to the CAAS operator provisioner API endpoint.
type Client struct {
	*common.APIAddresser
	facade base.FacadeCaller
}

// NewClient returns a client used to access the CAAS Operator Provisioner API.
func NewClient(caller base.APICaller) *Client {
	facadeCaller := base.NewFacadeCaller(caller, "CAASOperatorProvisioner")
	return &Client{
		facade:       facadeCaller,
		APIAddresser: common.NewAPIAddresser(facadeCaller),
	}
}

// WatchApplications returns a StringsWatcher that notifies of
// changes to the lifecycles of CAAS applications in the current model.
func (c *Client) WatchApplications() (watcher.StringsWatcher, error) {
	var result params.StringsWatchResult
	if err := c.facade.FacadeCall("WatchApplications", nil, &result); err != nil {
		return nil, err
	}
	if err := result.Error; err != nil {
		return nil, result.Error
	}
	w := apiwatcher.NewStringsWatcher(c.facade.RawAPICaller(), result)
	return w, nil
}

// ApplicationPassword holds parameters for setting
// an application password.
type ApplicationPassword struct {
	Name     string
	Password string
}

// SetPasswords sets API passwords for the specified applications.
func (c *Client) SetPasswords(appPasswords []ApplicationPassword) (params.ErrorResults, error) {
	var result params.ErrorResults
	args := params.EntityPasswords{Changes: make([]params.EntityPassword, len(appPasswords))}
	for i, p := range appPasswords {
		args.Changes[i] = params.EntityPassword{
			Tag: names.NewApplicationTag(p.Name).String(), Password: p.Password,
		}
	}
	err := c.facade.FacadeCall("SetPasswords", args, &result)
	if err != nil {
		return params.ErrorResults{}, err
	}
	if len(result.Results) != len(args.Changes) {
		return params.ErrorResults{}, errors.Errorf("expected %d result(s), got %d", len(args.Changes), len(result.Results))
	}
	return result, nil
}

// maybeNotFound returns an error satisfying errors.IsNotFound
// if the supplied error has a CodeNotFound error.
func maybeNotFound(err *params.Error) error {
	if err == nil || !params.IsCodeNotFound(err) {
		return err
	}
	return errors.NewNotFound(err, "")
}

// Life returns the lifecycle state for the specified CAAS application
// or unit in the current model.
func (c *Client) Life(appName string) (life.Value, error) {
	if !names.IsValidApplication(appName) {
		return "", errors.NotValidf("application name %q", appName)
	}
	args := params.Entities{
		Entities: []params.Entity{{Tag: names.NewApplicationTag(appName).String()}},
	}

	var results params.LifeResults
	if err := c.facade.FacadeCall("Life", args, &results); err != nil {
		return "", err
	}
	if n := len(results.Results); n != 1 {
		return "", errors.Errorf("expected 1 result, got %d", n)
	}
	if err := results.Results[0].Error; err != nil {
		return "", maybeNotFound(err)
	}
	return life.Value(results.Results[0].Life), nil
}

// OperatorProvisioningInfo holds the info needed to provision an operator.
type OperatorProvisioningInfo struct {
	ImagePath string
	Version   version.Number
}

// OperatorProvisioningInfo returns the info needed to provision an operator.
func (c *Client) OperatorProvisioningInfo() (OperatorProvisioningInfo, error) {
	var result params.OperatorProvisioningInfo
	if err := c.facade.FacadeCall("OperatorProvisioningInfo", nil, &result); err != nil {
		return OperatorProvisioningInfo{}, err
	}
	return OperatorProvisioningInfo{
		ImagePath: result.ImagePath,
		Version:   result.Version,
	}, nil
}
