package main

import (
	"crypto/rand"
	"io"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/work"
)

type productMissionAttemptRecoveryServices struct {
	authority  *work.AgentAttemptRecoveryAuthority
	completion *productAgentAttemptRecoveryCompletion
}

func newProductMissionAttemptRecoveryServices(
	config productMissionExecutionRuntimeConfig,
	runs *work.Authority,
	grants *authorization.Authority,
	readModel *projection.Projection,
	read productReadRoute,
	evidenceStore *evidence.Store,
	loops *work.AttemptLoopAuthority,
	inbox *work.AgentInboxCoordinator,
	active *productActiveAttemptRegistry,
) (productMissionAttemptRecoveryServices, error) {
	if inbox == nil {
		return productMissionAttemptRecoveryServices{}, nil
	}
	if runs == nil || grants == nil || readModel == nil ||
		nilProductAssetPort(read) || evidenceStore == nil || loops == nil || active == nil ||
		nilProductAgentInterface(config.ContextCapsules) ||
		nilProductAgentInterface(config.AgentCheckpointStore) ||
		len(config.AgentAdapters) == 0 || config.Now == nil {
		return productMissionAttemptRecoveryServices{},
			errProductInvalidAttemptRecoveryCompletion
	}
	continuation, err := newProductLoomNativeAttemptContinuation(
		config.AgentAdapters, config.ContextCapsules, config.AgentCheckpointStore,
		inbox, loops,
	)
	if err != nil {
		return productMissionAttemptRecoveryServices{}, err
	}
	runtime, err := newProductAgentAttemptRecoveryRuntime(active, continuation)
	if err != nil {
		return productMissionAttemptRecoveryServices{}, err
	}
	random := config.Random
	if nilProductAgentInterface(random) {
		random = io.Reader(rand.Reader)
	}
	authority, err := work.NewAgentAttemptRecoveryAuthority(
		inbox, continuation, config.Now, random,
	)
	if err != nil {
		return productMissionAttemptRecoveryServices{}, err
	}
	terminal, err := newProductWorkRecoveryTerminalAuthority(runs)
	if err != nil {
		return productMissionAttemptRecoveryServices{}, err
	}
	grantClosure, err := newProductAuthorizationRecoveryGrantClosure(grants)
	if err != nil {
		return productMissionAttemptRecoveryServices{}, err
	}
	observers, err := newProductRecoveryTeamFrameObserverFactory(
		readModel, read, evidenceStore,
	)
	if err != nil {
		return productMissionAttemptRecoveryServices{}, err
	}
	completion, err := newProductAgentAttemptRecoveryCompletionWithObserverFactory(
		runtime, terminal, grantClosure, observers,
	)
	if err != nil {
		return productMissionAttemptRecoveryServices{}, err
	}
	return productMissionAttemptRecoveryServices{
		authority: authority, completion: completion,
	}, nil
}
