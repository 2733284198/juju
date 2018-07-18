// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package client

import (
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/facades/client/bundle"
	"github.com/juju/juju/apiserver/params"
)

// GetBundleChanges returns the list of changes required to deploy the given
// bundle data. The changes are sorted by requirements, so that they can be
// applied in order.
// This call is deprecated, clients should use the GetChanges endpoint on the
// Bundle facade.
// Note: any new feature in the future like devices will never be supported here.
func (c *Client) GetBundleChanges(args params.BundleChangesParams) (params.BundleChangesResults, error) {
	st := c.api.state()

	bundleAPI, err := bundle.NewBundleAPI(st, c.api.auth, names.NewModelTag(st.ModelUUID()))
	if err != nil {
		return params.BundleChangesResults{}, err
	}
	apiV1 := bundle.APIv1{&bundle.APIv2{bundleAPI}}
	return apiV1.GetChanges(args)
}
