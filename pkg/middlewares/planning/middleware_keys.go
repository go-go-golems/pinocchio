package planning

import "github.com/go-go-golems/geppetto/pkg/turns"

// KeyDirective stores the planner's final execution directive for the current turn.
//
// This is a Pinocchio-local key (not part of geppetto) because the directive semantics
// are UI-driven (webchat planning widget) rather than provider/engine-driven.
var KeyDirective = turns.DataK[string]("pinocchio.webchat", "planning_directive", 1)
