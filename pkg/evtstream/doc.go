// Package evtstream provides a reusable substrate for event-streaming LLM and agent applications.
//
// Design goals:
//   - one canonical routing key: SessionId
//   - typed commands in, typed backend events out
//   - sibling UI and timeline projections
//   - storage and transport kept behind small public interfaces
//
// The package is intentionally generic. Application-specific concepts such as chat,
// agents, or legacy webchat envelopes belong in consuming packages and apps.
package evtstream
