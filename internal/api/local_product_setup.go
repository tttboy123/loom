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
