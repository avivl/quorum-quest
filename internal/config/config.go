package config

import (
	"github.com/avivl/quorum-quest/internal/observability"
)

type QuorumQuestConfig struct {
	Config *ballotconfig.BallotConfig
	logger *observability.SLogger
}
