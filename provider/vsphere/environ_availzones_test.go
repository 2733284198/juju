// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package vsphere_test

import (
	jc "github.com/juju/testing/checkers"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/provider/common"
)

type environAvailzonesSuite struct {
	EnvironFixture
}

var _ = gc.Suite(&environAvailzonesSuite{})

func (s *environAvailzonesSuite) TestAvailabilityZones(c *gc.C) {
	emptyResource := newComputeResource("empty")
	emptyResource.Summary.(*mockSummary).EffectiveCpu = 0
	s.client.computeResources = []*mo.ComputeResource{
		emptyResource,
		newComputeResource("z1"),
		newComputeResource("z2"),
	}
	s.client.resourcePools = map[string][]*object.ResourcePool{
		"z1/...": {makeResourcePool("pool-1", "/DC/host/z1/Resources")},
		"z2/...": {
			// Check we don't get broken by trailing slashes.
			makeResourcePool("pool-2", "/DC/host/z2/Resources/"),
			makeResourcePool("pool-3", "/DC/host/z2/Resources/child"),
			makeResourcePool("pool-4", "/DC/host/z2/Resources/child/nested"),
			makeResourcePool("pool-5", "/DC/host/z2/Resources/child/nested/other/"),
		},
	}

	c.Assert(s.env, gc.Implements, new(common.ZonedEnviron))
	zonedEnviron := s.env.(common.ZonedEnviron)
	zones, err := zonedEnviron.AvailabilityZones(s.callCtx)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(len(zones), gc.Equals, 5)
	// No zones for the empty resource.
	c.Assert(zones[0].Name(), gc.Equals, "z1")
	c.Assert(zones[1].Name(), gc.Equals, "z2")
	c.Assert(zones[2].Name(), gc.Equals, "z2/child")
	c.Assert(zones[3].Name(), gc.Equals, "z2/child/nested")
	c.Assert(zones[4].Name(), gc.Equals, "z2/child/nested/other")
}

func (s *environAvailzonesSuite) TestInstanceAvailabilityZoneNames(c *gc.C) {
	z1 := newComputeResource("z1")
	z2 := newComputeResource("z2")
	z3 := newComputeResource("z3")
	s.client.computeResources = []*mo.ComputeResource{z1, z2, z3}

	childPool := makeResourcePool("rp-child", "/DC/host/z3/Resources/child")
	childRef := childPool.Reference()
	s.client.resourcePools = map[string][]*object.ResourcePool{
		"z1/...": {makeResourcePool("rp-z1", "/DC/host/z1/Resources")},
		"z2/...": {makeResourcePool("rp-z2", "/DC/host/z2/Resources")},
		"z3/...": {
			makeResourcePool("rp-z3", "/DC/host/z3/Resources"),
			childPool,
		},
	}

	s.client.virtualMachines = []*mo.VirtualMachine{
		buildVM("inst-0").resourcePool(z2.ResourcePool).vm(),
		buildVM("inst-1").resourcePool(z1.ResourcePool).vm(),
		buildVM("inst-2").vm(),
		buildVM("inst-3").resourcePool(&childRef).vm(),
	}
	ids := []instance.Id{"inst-0", "inst-1", "inst-2", "inst-3", "inst-4"}

	zonedEnviron := s.env.(common.ZonedEnviron)
	zones, err := zonedEnviron.InstanceAvailabilityZoneNames(s.callCtx, ids)
	c.Assert(err, gc.Equals, environs.ErrPartialInstances)
	c.Assert(zones, jc.DeepEquals, []string{"z2", "z1", "", "z3/child", ""})
}

func (s *environAvailzonesSuite) TestInstanceAvailabilityZoneNamesNoInstances(c *gc.C) {
	zonedEnviron := s.env.(common.ZonedEnviron)
	_, err := zonedEnviron.InstanceAvailabilityZoneNames(s.callCtx, []instance.Id{"inst-0"})
	c.Assert(err, gc.Equals, environs.ErrNoInstances)
}

func (s *environAvailzonesSuite) TestDeriveAvailabilityZones(c *gc.C) {
	s.client.computeResources = []*mo.ComputeResource{
		newComputeResource("test-available"),
	}
	s.client.resourcePools = map[string][]*object.ResourcePool{
		"test-available/...": {makeResourcePool("pool-23", "/DC/host/test-available/Resources")},
	}

	c.Assert(s.env, gc.Implements, new(common.ZonedEnviron))
	zonedEnviron := s.env.(common.ZonedEnviron)

	zones, err := zonedEnviron.DeriveAvailabilityZones(
		s.callCtx,
		environs.StartInstanceParams{Placement: "zone=test-available"})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(zones, gc.DeepEquals, []string{"test-available"})
}

func (s *environAvailzonesSuite) TestDeriveAvailabilityZonesUnknown(c *gc.C) {
	c.Assert(s.env, gc.Implements, new(common.ZonedEnviron))
	zonedEnviron := s.env.(common.ZonedEnviron)

	zones, err := zonedEnviron.DeriveAvailabilityZones(
		s.callCtx,
		environs.StartInstanceParams{Placement: "zone=test-unknown"})
	c.Assert(err, gc.ErrorMatches, `availability zone "test-unknown" not found`)
	c.Assert(zones, gc.HasLen, 0)
}

func (s *environAvailzonesSuite) TestDeriveAvailabilityZonesInvalidPlacement(c *gc.C) {
	c.Assert(s.env, gc.Implements, new(common.ZonedEnviron))
	zonedEnviron := s.env.(common.ZonedEnviron)

	zones, err := zonedEnviron.DeriveAvailabilityZones(
		s.callCtx,
		environs.StartInstanceParams{
			Placement: "invalid-placement",
		})
	c.Assert(err, gc.ErrorMatches, `unknown placement directive: invalid-placement`)
	c.Assert(zones, gc.HasLen, 0)
}

func (s *environAvailzonesSuite) TestAvailabilityZonesPermissionError(c *gc.C) {
	AssertInvalidatesCredential(c, s.client, func(ctx context.ProviderCallContext) error {
		zonedEnv := s.env.(common.ZonedEnviron)
		_, err := zonedEnv.AvailabilityZones(ctx)
		return err
	})
}
