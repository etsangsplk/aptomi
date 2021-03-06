package action

import (
	"github.com/Aptomi/aptomi/pkg/engine/actual"
	"github.com/Aptomi/aptomi/pkg/engine/resolve"
	"github.com/Aptomi/aptomi/pkg/event"
	"github.com/Aptomi/aptomi/pkg/external"
	"github.com/Aptomi/aptomi/pkg/lang"
	"github.com/Aptomi/aptomi/pkg/plugin"
)

// Context is a data struct that will be passed into all state update actions, giving actions access to desired
// policy/state, and actual state and a way to updatae it, list of plugins, event log, etc
type Context struct {
	DesiredPolicy      *lang.Policy
	DesiredState       *resolve.PolicyResolution
	ActualState        *resolve.PolicyResolution
	ActualStateUpdater actual.StateUpdater
	ExternalData       *external.Data
	Plugins            plugin.Registry
	EventLog           *event.Log
}

// NewContext creates a new instance of Context
func NewContext(desiredPolicy *lang.Policy, desiredState *resolve.PolicyResolution,
	actualState *resolve.PolicyResolution, actualStateUpdater actual.StateUpdater, externalData *external.Data,
	plugins plugin.Registry, eventLog *event.Log) *Context {

	return &Context{
		DesiredPolicy:      desiredPolicy,
		DesiredState:       desiredState,
		ActualState:        actualState,
		ActualStateUpdater: actualStateUpdater,
		ExternalData:       externalData,
		Plugins:            plugins,
		EventLog:           eventLog,
	}
}
