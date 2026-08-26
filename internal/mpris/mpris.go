package mpris

import (
	"github.com/bprendie/omasub/internal/remote"
)

// Service exposes the live Omasub player to desktop media clients.
type Service interface {
	Publish(remote.Snapshot)
	Close() error
}
