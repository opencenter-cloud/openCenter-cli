package config

import "github.com/opencenter-cloud/opencenter-cli/internal/config/services"

// AdoptionMode defines how Flux interacts with a service that may already exist in the cluster.
type AdoptionMode = services.AdoptionMode

const (
	AdoptionModeManaged  = services.AdoptionModeManaged
	AdoptionModeExternal = services.AdoptionModeExternal
	AdoptionModeSync     = services.AdoptionModeSync
	AdoptionModeDeferred = services.AdoptionModeDeferred
	AdoptionModeTakeover = services.AdoptionModeTakeover
)

// ValidAdoptionModes contains all valid adoption mode values.
var ValidAdoptionModes = []AdoptionMode{
	AdoptionModeManaged,
	AdoptionModeExternal,
	AdoptionModeSync,
	AdoptionModeDeferred,
	AdoptionModeTakeover,
}
