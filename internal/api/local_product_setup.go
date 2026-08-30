package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalProductSetupAPI = errors.New(
	"invalid local product setup API",
)

type localProductSetupBackend interface {
	SetupSnapshot(context.Context) (app.SetupSnapshot, error)
	StartBuilder(
		context.Context,
		app.BuilderStartCommand,
	) (app.BuilderSessionView, error)
}

type LocalProductSetupAPI struct {
	backend localProductSetupBackend
}

func NewLocalProductSetupAPI(
	backend localProductSetupBackend,
) (*LocalProductSetupAPI, error) {
	if backend == nil {
		return nil, ErrInvalidLocalProductSetupAPI
	}
	return &LocalProductSetupAPI{backend: backend}, nil
}

func (service *LocalProductSetupAPI) SetupSnapshot(
	ctx context.Context,
) (app.SetupSnapshot, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.SetupSnapshot{}, ErrInvalidLocalProductSetupAPI
	}
	snapshot, err := service.backend.SetupSnapshot(ctx)
	if err != nil {
		return app.SetupSnapshot{}, err
	}
	return canonicalSetupSnapshot(snapshot), nil
}

func (service *LocalProductSetupAPI) ConnectCodex(
	ctx context.Context,
) (app.ProviderConnectResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.ProviderConnectResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ConnectCodex(context.Context) (app.ProviderConnectResult, error)
	})
	if !ok {
		return app.ProviderConnectResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ConnectCodex(ctx)
}

func (service *LocalProductSetupAPI) ConnectClaudeCode(
	ctx context.Context,
) (app.ProviderConnectResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.ProviderConnectResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ConnectClaudeCode(context.Context) (app.ProviderConnectResult, error)
	})
	if !ok {
		return app.ProviderConnectResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ConnectClaudeCode(ctx)
}

func (service *LocalProductSetupAPI) CancelClaudeCode(
	ctx context.Context,
) (app.ProviderConnectResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.ProviderConnectResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		CancelClaudeCode(context.Context) (app.ProviderConnectResult, error)
	})
	if !ok {
		return app.ProviderConnectResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.CancelClaudeCode(ctx)
}

func (service *LocalProductSetupAPI) ConfigureProviderAccountPolicy(
	ctx context.Context,
	command app.ProviderAccountPolicyCommand,
) (app.ProviderAccountPolicyResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.ProviderAccountPolicyResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ConfigureProviderAccountPolicy(
			context.Context,
			app.ProviderAccountPolicyCommand,
		) (app.ProviderAccountPolicyResult, error)
	})
	if !ok {
		return app.ProviderAccountPolicyResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ConfigureProviderAccountPolicy(ctx, command)
}

func (service *LocalProductSetupAPI) ConfigureProviderModelRateCard(
	ctx context.Context,
	command app.ProviderModelRateCardCommand,
) (app.ProviderModelRateCardResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.ProviderModelRateCardResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ConfigureProviderModelRateCard(
			context.Context,
			app.ProviderModelRateCardCommand,
		) (app.ProviderModelRateCardResult, error)
	})
	if !ok {
		return app.ProviderModelRateCardResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ConfigureProviderModelRateCard(ctx, command)
}

func (service *LocalProductSetupAPI) ConfigureRemoteToolBackendEnrollment(
	ctx context.Context,
	command app.RemoteToolBackendEnrollmentCommand,
) (app.RemoteToolBackendEnrollmentResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.RemoteToolBackendEnrollmentResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ConfigureRemoteToolBackendEnrollment(
			context.Context,
			app.RemoteToolBackendEnrollmentCommand,
		) (app.RemoteToolBackendEnrollmentResult, error)
	})
	if !ok {
		return app.RemoteToolBackendEnrollmentResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ConfigureRemoteToolBackendEnrollment(ctx, command)
}

func (service *LocalProductSetupAPI) RevokeRemoteToolBackendEnrollment(
	ctx context.Context,
	command app.RemoteToolBackendEnrollmentRevokeCommand,
) (app.RemoteToolBackendEnrollmentResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.RemoteToolBackendEnrollmentResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		RevokeRemoteToolBackendEnrollment(
			context.Context,
			app.RemoteToolBackendEnrollmentRevokeCommand,
		) (app.RemoteToolBackendEnrollmentResult, error)
	})
	if !ok {
		return app.RemoteToolBackendEnrollmentResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.RevokeRemoteToolBackendEnrollment(ctx, command)
}

func (service *LocalProductSetupAPI) Close() error {
	if service == nil || service.backend == nil {
		return ErrInvalidLocalProductSetupAPI
	}
	if closer, ok := service.backend.(interface {
		Close() error
	}); ok {
		return closer.Close()
	}
	return nil
}

func (service *LocalProductSetupAPI) StartBuilder(
	ctx context.Context,
	command app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.BuilderSessionView{}, ErrInvalidLocalProductSetupAPI
	}
	session, err := service.backend.StartBuilder(ctx, command)
	if err != nil {
		return app.BuilderSessionView{}, err
	}
	return canonicalBuilderSession(session), nil
}

func (service *LocalProductSetupAPI) AnswerBuilder(
	ctx context.Context,
	command app.BuilderAnswerCommand,
) (app.BuilderSessionView, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.BuilderSessionView{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		AnswerBuilder(
			context.Context,
			app.BuilderAnswerCommand,
		) (app.BuilderSessionView, error)
	})
	if !ok {
		return app.BuilderSessionView{}, ErrInvalidLocalProductSetupAPI
	}
	session, err := backend.AnswerBuilder(ctx, command)
	if err != nil {
		return app.BuilderSessionView{}, err
	}
	return canonicalBuilderSession(session), nil
}

func (service *LocalProductSetupAPI) EditBuilder(
	ctx context.Context,
	command app.BuilderEditCommand,
) (app.BuilderSessionView, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.BuilderSessionView{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		EditBuilder(
			context.Context,
			app.BuilderEditCommand,
		) (app.BuilderSessionView, error)
	})
	if !ok {
		return app.BuilderSessionView{}, ErrInvalidLocalProductSetupAPI
	}
	session, err := backend.EditBuilder(ctx, command)
	if err != nil {
		return app.BuilderSessionView{}, err
	}
	return canonicalBuilderSession(session), nil
}

func (service *LocalProductSetupAPI) ValidateBuilder(
	ctx context.Context,
	command app.BuilderValidateCommand,
) (app.BuilderSessionView, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.BuilderSessionView{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ValidateBuilder(
			context.Context,
			app.BuilderValidateCommand,
		) (app.BuilderSessionView, error)
	})
	if !ok {
		return app.BuilderSessionView{}, ErrInvalidLocalProductSetupAPI
	}
	session, err := backend.ValidateBuilder(ctx, command)
	if err != nil {
		return app.BuilderSessionView{}, err
	}
	return canonicalBuilderSession(session), nil
}

func (service *LocalProductSetupAPI) ConfirmBuilder(
	ctx context.Context,
	command app.BuilderConfirmCommand,
) (app.BuilderConfirmation, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.BuilderConfirmation{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ConfirmBuilder(
			context.Context,
			app.BuilderConfirmCommand,
		) (app.BuilderConfirmation, error)
	})
	if !ok {
		return app.BuilderConfirmation{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ConfirmBuilder(ctx, command)
}

func (service *LocalProductSetupAPI) ArchiveTeam(
	ctx context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.SetupSavedTeamPreview{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ArchiveTeam(
			context.Context,
			app.TeamStatusCommand,
		) (app.SetupSavedTeamPreview, error)
	})
	if !ok {
		return app.SetupSavedTeamPreview{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ArchiveTeam(ctx, command)
}

func (service *LocalProductSetupAPI) RestoreTeam(
	ctx context.Context,
	command app.TeamStatusCommand,
) (app.SetupSavedTeamPreview, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.SetupSavedTeamPreview{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		RestoreTeam(
			context.Context,
			app.TeamStatusCommand,
		) (app.SetupSavedTeamPreview, error)
	})
	if !ok {
		return app.SetupSavedTeamPreview{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.RestoreTeam(ctx, command)
}

func (service *LocalProductSetupAPI) ConfigureCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ConfigureCredential(
			context.Context,
			app.CredentialSetupCommand,
		) (app.CredentialSetupResult, error)
	})
	if !ok {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ConfigureCredential(ctx, command)
}

func (service *LocalProductSetupAPI) ImportCredentialCandidate(
	ctx context.Context,
	command app.CredentialImportCommand,
) (app.CredentialSetupResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ImportCredentialCandidate(
			context.Context,
			app.CredentialImportCommand,
		) (app.CredentialSetupResult, error)
	})
	if !ok {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ImportCredentialCandidate(ctx, command)
}

func (service *LocalProductSetupAPI) ApproveEndpointCandidate(
	ctx context.Context,
	command app.EndpointReviewCommand,
) (app.EndpointReviewResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.EndpointReviewResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ApproveEndpointCandidate(
			context.Context,
			app.EndpointReviewCommand,
		) (app.EndpointReviewResult, error)
	})
	if !ok {
		return app.EndpointReviewResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ApproveEndpointCandidate(ctx, command)
}

func (service *LocalProductSetupAPI) VerifyCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		VerifyCredential(
			context.Context,
			app.CredentialSetupCommand,
		) (app.CredentialSetupResult, error)
	})
	if !ok {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.VerifyCredential(ctx, command)
}

func (service *LocalProductSetupAPI) ReplaceCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		ReplaceCredential(
			context.Context,
			app.CredentialSetupCommand,
		) (app.CredentialSetupResult, error)
	})
	if !ok {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.ReplaceCredential(ctx, command)
}

func (service *LocalProductSetupAPI) RevokeCredential(
	ctx context.Context,
	command app.CredentialSetupCommand,
) (app.CredentialSetupResult, error) {
	if service == nil || service.backend == nil || ctx == nil {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	backend, ok := service.backend.(interface {
		RevokeCredential(
			context.Context,
			app.CredentialSetupCommand,
		) (app.CredentialSetupResult, error)
	})
	if !ok {
		return app.CredentialSetupResult{}, ErrInvalidLocalProductSetupAPI
	}
	return backend.RevokeCredential(ctx, command)
}

func canonicalSetupSnapshot(input app.SetupSnapshot) app.SetupSnapshot {
	if input.CredentialVault != nil {
		vault := *input.CredentialVault
		input.CredentialVault = &vault
	}
	input.Providers = append(
		[]app.ProviderDirectoryEntry{},
		input.Providers...,
	)
	input.ProviderAccounts = append(
		[]app.ProviderAccountDirectoryEntry{},
		input.ProviderAccounts...,
	)
	for index := range input.ProviderAccounts {
		input.ProviderAccounts[index].RateCards = append(
			[]app.ProviderModelRateCardDirectoryEntry{},
			input.ProviderAccounts[index].RateCards...,
		)
		input.ProviderAccounts[index].RemoteToolBackends = append(
			[]app.RemoteToolBackendDirectoryEntry{},
			input.ProviderAccounts[index].RemoteToolBackends...,
		)
		for backendIndex := range input.ProviderAccounts[index].RemoteToolBackends {
			input.ProviderAccounts[index].RemoteToolBackends[backendIndex].AllowedTools = append(
				[]string{},
				input.ProviderAccounts[index].RemoteToolBackends[backendIndex].AllowedTools...,
			)
		}
	}
	input.ConversationProfiles = append(
		[]app.ConversationProviderProfile{}, input.ConversationProfiles...,
	)
	input.Runtimes = append([]app.SetupRuntimePreview{}, input.Runtimes...)
	for index := range input.Runtimes {
		input.Runtimes[index].ModelIDs = append(
			[]string{},
			input.Runtimes[index].ModelIDs...,
		)
		input.Runtimes[index].ObservedCapabilities = append(
			[]string{},
			input.Runtimes[index].ObservedCapabilities...,
		)
	}
	input.SavedTeams = append(
		[]app.SetupSavedTeamPreview{},
		input.SavedTeams...,
	)
	input.Templates = append(
		[]app.SetupTeamTemplatePreview{},
		input.Templates...,
	)
	for index := range input.Templates {
		input.Templates[index].SubagentRoleIDs = append(
			[]string{},
			input.Templates[index].SubagentRoleIDs...,
		)
	}
	input.RoleOptions = append(
		[]app.SetupRoleOptionPreview{},
		input.RoleOptions...,
	)
	for index := range input.RoleOptions {
		role := &input.RoleOptions[index]
		role.SkillRevisionIDs = append(
			[]string{},
			role.SkillRevisionIDs...,
		)
		role.PermissionIDs = append([]string{}, role.PermissionIDs...)
		role.ResourceIDs = append([]string{}, role.ResourceIDs...)
		role.RequiredCapabilities = append(
			[]string{},
			role.RequiredCapabilities...,
		)
	}
	input.Skills = append([]app.SetupSkillRevision{}, input.Skills...)
	for index := range input.Skills {
		input.Skills[index].CompatibleRuntimes = append(
			[]string{},
			input.Skills[index].CompatibleRuntimes...,
		)
	}
	input.Permissions = append([]string{}, input.Permissions...)
	input.Resources = append([]app.SetupResourcePointer{}, input.Resources...)
	return input
}

func canonicalBuilderSession(
	input app.BuilderSessionView,
) app.BuilderSessionView {
	input.Question.Options = append(
		[]app.BuilderQuestionOption{},
		input.Question.Options...,
	)
	input.Preview.Roles = append(
		[]app.BuilderRolePreview{},
		input.Preview.Roles...,
	)
	for index := range input.Preview.Roles {
		role := &input.Preview.Roles[index]
		role.Runtime.ModelIDs = append([]string{}, role.Runtime.ModelIDs...)
		role.Runtime.ObservedCapabilities = append(
			[]string{},
			role.Runtime.ObservedCapabilities...,
		)
		role.Skills = append([]app.SetupSkillRevision{}, role.Skills...)
		for skillIndex := range role.Skills {
			role.Skills[skillIndex].CompatibleRuntimes = append(
				[]string{},
				role.Skills[skillIndex].CompatibleRuntimes...,
			)
		}
		role.PermissionIDs = append([]string{}, role.PermissionIDs...)
		role.ResourceIDs = append([]string{}, role.ResourceIDs...)
		role.RequiredCapabilities = append(
			[]string{},
			role.RequiredCapabilities...,
		)
	}
	input.Preview.Permissions = append(
		[]string{},
		input.Preview.Permissions...,
	)
	input.Preview.Resources = append(
		[]string{},
		input.Preview.Resources...,
	)
	input.Preview.CompatibilityGaps = append(
		[]string{},
		input.Preview.CompatibilityGaps...,
	)
	return input
}
