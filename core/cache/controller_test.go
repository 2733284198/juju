// Copyright 2018 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package cache_test

import (
	"time"

	"github.com/juju/errors"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/juju/worker.v1/workertest"

	"github.com/juju/juju/core/cache"
	"github.com/juju/juju/core/life"
)

type ControllerSuite struct {
	cache.BaseSuite
}

var _ = gc.Suite(&ControllerSuite{})

func (s *ControllerSuite) TestConfigValid(c *gc.C) {
	err := s.Config.Validate()
	c.Assert(err, jc.ErrorIsNil)
}

func (s *ControllerSuite) TestConfigMissingChanges(c *gc.C) {
	s.Config.Changes = nil
	err := s.Config.Validate()
	c.Check(err, gc.ErrorMatches, "nil Changes not valid")
	c.Check(err, jc.Satisfies, errors.IsNotValid)
}

func (s *ControllerSuite) TestController(c *gc.C) {
	controller, err := s.NewController()
	c.Assert(err, jc.ErrorIsNil)

	c.Check(controller.ModelUUIDs(), gc.HasLen, 0)
	c.Check(controller.Report(), gc.HasLen, 0)

	workertest.CleanKill(c, controller)
}

func (s *ControllerSuite) TestAddModel(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, modelChange, events)

	c.Check(controller.ModelUUIDs(), jc.SameContents, []string{modelChange.ModelUUID})
	c.Check(controller.Report(), gc.DeepEquals, map[string]interface{}{
		"model-uuid": map[string]interface{}{
			"name":              "model-owner/test-model",
			"life":              life.Value("alive"),
			"application-count": 0,
			"charm-count":       0,
			"machine-count":     0,
			"unit-count":        0,
		}})

	// The model has the first ID and is registered.
	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	s.AssertResident(c, mod.CacheId(), true)
}

func (s *ControllerSuite) TestRemoveModel(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, modelChange, events)

	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)

	remove := cache.RemoveModel{ModelUUID: modelChange.ModelUUID}
	s.processChange(c, remove, events)

	c.Check(controller.ModelUUIDs(), gc.HasLen, 0)
	c.Check(controller.Report(), gc.HasLen, 0)
	s.AssertResident(c, mod.CacheId(), false)
}

func (s *ControllerSuite) TestAddApplication(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, appChange, events)

	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(mod.Report()["application-count"], gc.Equals, 1)

	app, err := mod.Application(appChange.Name)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(app, gc.NotNil)

	s.AssertResident(c, app.CacheId(), true)
}

func (s *ControllerSuite) TestRemoveApplication(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, appChange, events)

	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	app, err := mod.Application(appChange.Name)
	c.Assert(err, jc.ErrorIsNil)

	remove := cache.RemoveApplication{
		ModelUUID: modelChange.ModelUUID,
		Name:      appChange.Name,
	}
	s.processChange(c, remove, events)

	c.Assert(err, jc.ErrorIsNil)
	c.Check(mod.Report()["application-count"], gc.Equals, 0)
	s.AssertResident(c, app.CacheId(), false)
}

func (s *ControllerSuite) TestAddCharm(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, charmChange, events)

	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(mod.Report()["charm-count"], gc.Equals, 1)

	ch, err := mod.Charm(charmChange.CharmURL)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(ch, gc.NotNil)
	s.AssertResident(c, ch.CacheId(), true)
}

func (s *ControllerSuite) TestRemoveCharm(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, charmChange, events)

	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	ch, err := mod.Charm(charmChange.CharmURL)
	c.Assert(err, jc.ErrorIsNil)

	remove := cache.RemoveCharm{
		ModelUUID: modelChange.ModelUUID,
		CharmURL:  charmChange.CharmURL,
	}
	s.processChange(c, remove, events)

	c.Check(mod.Report()["charm-count"], gc.Equals, 0)
	s.AssertResident(c, ch.CacheId(), false)
}

func (s *ControllerSuite) TestAddMachine(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, machineChange, events)

	mod, err := controller.Model(machineChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(mod.Report()["machine-count"], gc.Equals, 1)

	machine, err := mod.Machine(machineChange.Id)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(machine, gc.NotNil)
	s.AssertResident(c, machine.CacheId(), true)
}

func (s *ControllerSuite) TestRemoveMachine(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, machineChange, events)

	mod, err := controller.Model(machineChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	machine, err := mod.Machine(machineChange.Id)
	c.Assert(err, jc.ErrorIsNil)

	remove := cache.RemoveMachine{
		ModelUUID: machineChange.ModelUUID,
		Id:        machineChange.Id,
	}
	s.processChange(c, remove, events)

	c.Check(mod.Report()["machine-count"], gc.Equals, 0)
	s.AssertResident(c, machine.CacheId(), false)
}

func (s *ControllerSuite) TestAddUnit(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, unitChange, events)

	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(mod.Report()["unit-count"], gc.Equals, 1)

	unit, err := mod.Unit(unitChange.Name)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(unit, gc.NotNil)
	s.AssertResident(c, unit.CacheId(), true)
}

func (s *ControllerSuite) TestRemoveUnit(c *gc.C) {
	controller, events := s.new(c)
	s.processChange(c, unitChange, events)

	mod, err := controller.Model(modelChange.ModelUUID)
	c.Assert(err, jc.ErrorIsNil)
	unit, err := mod.Unit(unitChange.Name)
	c.Assert(err, jc.ErrorIsNil)

	remove := cache.RemoveUnit{
		ModelUUID: modelChange.ModelUUID,
		Name:      unitChange.Name,
	}
	s.processChange(c, remove, events)

	c.Check(mod.Report()["unit-count"], gc.Equals, 0)
	s.AssertResident(c, unit.CacheId(), false)
}

func (s *ControllerSuite) TestMarkAndSweep(c *gc.C) {
	controller, events := s.new(c)

	// Note that the model change is processed last.
	s.processChange(c, charmChange, events)
	s.processChange(c, appChange, events)
	s.processChange(c, machineChange, events)
	s.processChange(c, unitChange, events)
	s.processChange(c, modelChange, events)

	c.Assert(controller.Marked(), jc.IsFalse)
	controller.Mark()
	c.Assert(controller.Marked(), jc.IsTrue)

	done := make(chan struct{})
	go func() {
		// Removals are congruent with FILO.
		// Model is last because models are added if they do not exist,
		// when we first get a delta for one of their entities.
		c.Check(s.nextChange(c, events), gc.FitsTypeOf, cache.RemoveUnit{})
		c.Check(s.nextChange(c, events), gc.FitsTypeOf, cache.RemoveMachine{})
		c.Check(s.nextChange(c, events), gc.FitsTypeOf, cache.RemoveApplication{})
		c.Check(s.nextChange(c, events), gc.FitsTypeOf, cache.RemoveCharm{})
		c.Check(s.nextChange(c, events), gc.FitsTypeOf, cache.RemoveModel{})
		close(done)
	}()

	controller.Sweep()
	select {
	case <-done:
	case <-time.After(testing.LongWait):
		c.Fatal("timeout waiting for sweep removal messages")
	}

	c.Assert(controller.Marked(), jc.IsFalse)
	s.AssertNoResidents(c)
}

func (s *ControllerSuite) new(c *gc.C) (*cache.Controller, <-chan interface{}) {
	events := s.captureEvents(c)
	controller, err := s.NewController()
	c.Assert(err, jc.ErrorIsNil)
	s.AddCleanup(func(c *gc.C) { workertest.CleanKill(c, controller) })
	return controller, events
}

func (s *ControllerSuite) captureEvents(c *gc.C) <-chan interface{} {
	events := make(chan interface{})
	s.Config.Notify = func(change interface{}) {
		send := false
		switch change.(type) {
		case cache.ModelChange:
			send = true
		case cache.RemoveModel:
			send = true
		case cache.ApplicationChange:
			send = true
		case cache.RemoveApplication:
			send = true
		case cache.CharmChange:
			send = true
		case cache.RemoveCharm:
			send = true
		case cache.MachineChange:
			send = true
		case cache.RemoveMachine:
			send = true
		case cache.UnitChange:
			send = true
		case cache.RemoveUnit:
			send = true
		default:
			// no-op
		}
		if send {
			c.Logf("sending %#v", change)
			select {
			case events <- change:
			case <-time.After(testing.LongWait):
				c.Fatalf("change not processed by test")
			}
		}
	}
	return events
}

func (s *ControllerSuite) processChange(c *gc.C, change interface{}, notify <-chan interface{}) {
	select {
	case s.Changes <- change:
	case <-time.After(testing.LongWait):
		c.Fatalf("controller did not read change")
	}
	select {
	case obtained := <-notify:
		c.Check(obtained, jc.DeepEquals, change)
	case <-time.After(testing.LongWait):
		c.Fatalf("controller did not handle change")
	}
}

func (s *ControllerSuite) nextChange(c *gc.C, changes <-chan interface{}) interface{} {
	var obtained interface{}
	select {
	case obtained = <-changes:
	case <-time.After(testing.LongWait):
		c.Fatalf("no change")
	}
	return obtained
}
