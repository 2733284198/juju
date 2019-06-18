// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package cache_test

import (
	"github.com/juju/errors"
	"github.com/juju/juju/core/network"
	jc "github.com/juju/testing/checkers"
	"github.com/prometheus/client_golang/prometheus/testutil"
	gc "gopkg.in/check.v1"
	"gopkg.in/juju/worker.v1/workertest"

	"github.com/juju/juju/core/cache"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/core/status"
)

type ModelSuite struct {
	cache.EntitySuite
}

var _ = gc.Suite(&ModelSuite{})

func (s *ModelSuite) TestReport(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	c.Assert(m.Report(), jc.DeepEquals, map[string]interface{}{
		"name":              "model-owner/test-model",
		"life":              life.Value("alive"),
		"application-count": 0,
		"charm-count":       0,
		"machine-count":     0,
		"unit-count":        0,
		"branch-count":      0,
	})
}

func (s *ModelSuite) TestConfig(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	c.Assert(m.Config(), jc.DeepEquals, map[string]interface{}{
		"key":     "value",
		"another": "foo",
	})
}

func (s *ModelSuite) TestNewModelGeneratesHash(c *gc.C) {
	s.NewModel(modelChange, nil)
	c.Check(testutil.ToFloat64(s.Gauges.ModelHashCacheMiss), gc.Equals, float64(1))
}

func (s *ModelSuite) TestModelConfigIncrementsReadCount(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	c.Check(testutil.ToFloat64(s.Gauges.ModelConfigReads), gc.Equals, float64(0))
	m.Config()
	c.Check(testutil.ToFloat64(s.Gauges.ModelConfigReads), gc.Equals, float64(1))
	m.Config()
	c.Check(testutil.ToFloat64(s.Gauges.ModelConfigReads), gc.Equals, float64(2))
}

// Some of the tested behaviour in the following methods is specific to the
// watcher, but using a cached model avoids the need to put scaffolding code in
// export_test.go to create a watcher in isolation.
func (s *ModelSuite) TestConfigWatcherStops(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	w := m.WatchConfig()
	wc := NewNotifyWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange()
	wc.AssertStops()
}

func (s *ModelSuite) TestConfigWatcherChange(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	w := m.WatchConfig()
	defer workertest.CleanKill(c, w)
	wc := NewNotifyWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange()

	change := modelChange
	change.Config = map[string]interface{}{
		"key": "changed",
	}

	m.SetDetails(change)
	wc.AssertOneChange()

	// The hash is generated each time we set the details.
	c.Check(testutil.ToFloat64(s.Gauges.ModelHashCacheMiss), gc.Equals, float64(2))

	// The value is retrieved from the cache when the watcher is created and notified.
	c.Check(testutil.ToFloat64(s.Gauges.ModelHashCacheHit), gc.Equals, float64(2))
}

func (s *ModelSuite) TestConfigWatcherOneValue(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	w := m.WatchConfig("key")
	defer workertest.CleanKill(c, w)
	wc := NewNotifyWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange()

	change := modelChange
	change.Config = map[string]interface{}{
		"key":     "changed",
		"another": "foo",
	}

	m.SetDetails(change)
	wc.AssertOneChange()
}

func (s *ModelSuite) TestConfigWatcherOneValueOtherChange(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	w := m.WatchConfig("key")

	// The worker is the first and only resource (1).
	resourceId := uint64(1)
	s.AssertWorkerResource(c, m.Resident, resourceId, true)
	defer func() {
		workertest.CleanKill(c, w)
		s.AssertWorkerResource(c, m.Resident, resourceId, false)
	}()

	wc := NewNotifyWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange()

	change := modelChange
	change.Config = map[string]interface{}{
		"key":     "value",
		"another": "changed",
	}

	m.SetDetails(change)
	wc.AssertNoChange()
}

func (s *ModelSuite) TestConfigWatcherSameValuesCacheHit(c *gc.C) {
	m := s.NewModel(modelChange, nil)

	w := m.WatchConfig("key", "another")
	defer workertest.CleanKill(c, w)

	w2 := m.WatchConfig("another", "key")
	defer workertest.CleanKill(c, w2)

	// One cache miss for the "all" hash, and one for the specific fields.
	c.Check(testutil.ToFloat64(s.Gauges.ModelHashCacheMiss), gc.Equals, float64(2))

	// Specific field hash should get a hit despite the field ordering.
	c.Check(testutil.ToFloat64(s.Gauges.ModelHashCacheHit), gc.Equals, float64(1))
}

func (s *ModelSuite) TestApplicationNotFoundError(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	_, err := m.Application("nope")
	c.Assert(errors.IsNotFound(err), jc.IsTrue)
}

func (s *ModelSuite) TestCharmNotFoundError(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	_, err := m.Charm("nope")
	c.Assert(errors.IsNotFound(err), jc.IsTrue)
}

func (s *ModelSuite) TestMachineNotFoundError(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	_, err := m.Machine("nope")
	c.Assert(errors.IsNotFound(err), jc.IsTrue)
}

func (s *ModelSuite) TestUnitNotFoundError(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	_, err := m.Unit("nope")
	c.Assert(errors.IsNotFound(err), jc.IsTrue)
}

func (s *ModelSuite) TestUnitReturnsCopy(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	m.UpdateUnit(unitChange, s.Manager)

	u1, err := m.Unit(unitChange.Name)
	c.Assert(err, jc.ErrorIsNil)

	// Make a change to the slice returned in the copy.
	ports := u1.Ports()
	ports = append(ports, network.Port{Protocol: "tcp", Number: 54321})

	// Get another copy from the model and ensure it is unchanged.
	u2, err := m.Unit(unitChange.Name)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(u2.Ports(), gc.DeepEquals, unitChange.Ports)
}

func (s *ModelSuite) TestBranchNotFoundError(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	_, err := m.Branch("nope")
	c.Assert(errors.IsNotFound(err), jc.IsTrue)
}

func (s *ModelSuite) TestBranchReturnsCopy(c *gc.C) {
	m := s.NewModel(modelChange, nil)
	m.UpdateBranch(branchChange, s.Manager)

	b1, err := m.Branch(branchChange.Name)
	c.Assert(err, jc.ErrorIsNil)

	// Make a change to the map returned in the copy.
	au := b1.AssignedUnits()
	au["banana"] = []string{"banana/1", "banana/2"}

	// Get another copy from the model and ensure it is unchanged.
	b2, err := m.Branch(branchChange.Name)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(b2.AssignedUnits(), gc.DeepEquals, branchChange.AssignedUnits)
}

func (s *ControllerSuite) TestWatchMachineStops(c *gc.C) {
	controller, _ := s.newWithMachine(c)
	m, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)

	w, err := m.WatchMachines()
	c.Assert(err, jc.ErrorIsNil)
	wc := NewStringsWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange([]string{machineChange.Id})

	// The worker is the first and only resource (1).
	resourceId := uint64(1)
	s.AssertWorkerResource(c, m.Resident, resourceId, true)
	wc.AssertStops()
	s.AssertWorkerResource(c, m.Resident, resourceId, false)
}

func (s *ControllerSuite) TestWatchMachineAddMachine(c *gc.C) {
	w, events := s.setupWithWatchMachine(c)
	defer workertest.CleanKill(c, w)
	wc := NewStringsWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange([]string{machineChange.Id})

	change := cache.MachineChange{
		ModelUUID: modelChange.ModelUUID,
		Id:        "2",
	}
	s.processChange(c, change, events)
	wc.AssertOneChange([]string{change.Id})
}

func (s *ControllerSuite) TestWatchMachineAddContainerNoChange(c *gc.C) {
	w, events := s.setupWithWatchMachine(c)
	defer workertest.CleanKill(c, w)
	wc := NewStringsWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange([]string{machineChange.Id})

	change := cache.MachineChange{
		ModelUUID: modelChange.ModelUUID,
		Id:        "2/lxd/0",
	}
	s.processChange(c, change, events)
	change2 := change
	change2.Id = "3"
	s.processChange(c, change2, events)
	wc.AssertOneChange([]string{change2.Id})
}

func (s *ControllerSuite) TestWatchMachineRemoveMachine(c *gc.C) {
	w, events := s.setupWithWatchMachine(c)
	defer workertest.CleanKill(c, w)
	wc := NewStringsWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange([]string{machineChange.Id})

	change := cache.RemoveMachine{
		ModelUUID: modelChange.ModelUUID,
		Id:        machineChange.Id,
	}
	s.processChange(c, change, events)
	wc.AssertOneChange([]string{change.Id})
}

func (s *ControllerSuite) TestWatchMachineChangeMachine(c *gc.C) {
	w, events := s.setupWithWatchMachine(c)
	defer workertest.CleanKill(c, w)
	wc := NewStringsWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange([]string{machineChange.Id})

	change := cache.MachineChange{
		ModelUUID: modelChange.ModelUUID,
		Id:        "0",
	}
	s.processChange(c, change, events)
	wc.AssertNoChange()
}

func (s *ControllerSuite) TestWatchMachineGatherMachines(c *gc.C) {
	w, events := s.setupWithWatchMachine(c)
	defer workertest.CleanKill(c, w)
	wc := NewStringsWatcherC(c, w)
	// Sends initial event.
	wc.AssertOneChange([]string{machineChange.Id})

	change := cache.MachineChange{
		ModelUUID: modelChange.ModelUUID,
		Id:        "2",
	}
	s.processChange(c, change, events)
	change2 := change
	change2.Id = "3"
	s.processChange(c, change2, events)
	wc.AssertMaybeCombinedChanges([]string{change.Id, change2.Id})
}

func (s *ControllerSuite) newWithMachine(c *gc.C) (*cache.Controller, <-chan interface{}) {
	events := s.captureEvents(c)
	controller, err := s.NewController()
	c.Assert(err, jc.ErrorIsNil)
	s.AddCleanup(func(c *gc.C) { workertest.CleanKill(c, controller) })
	s.processChange(c, modelChange, events)
	s.processChange(c, machineChange, events)
	return controller, events
}

func (s *ControllerSuite) setupWithWatchMachine(c *gc.C) (*cache.PredicateStringsWatcher, <-chan interface{}) {
	controller, events := s.newWithMachine(c)
	m, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)

	containerChange := cache.MachineChange{
		ModelUUID: modelChange.ModelUUID,
		Id:        "2/lxd/0",
	}
	s.processChange(c, containerChange, events)

	w, err := m.WatchMachines()
	c.Assert(err, jc.ErrorIsNil)
	return w, events
}

var modelChange = cache.ModelChange{
	ModelUUID: "model-uuid",
	Name:      "test-model",
	Life:      life.Alive,
	Owner:     "model-owner",
	Config: map[string]interface{}{
		"key":     "value",
		"another": "foo",
	},

	Status: status.StatusInfo{
		Status: status.Active,
	},
}