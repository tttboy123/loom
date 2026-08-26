package harnessgateway

type Registration struct {
	Configured ConfiguredHarness
	Backend    Backend
}

type registeredBackend struct {
	configured ConfiguredHarness
	backend    Backend
}

type BackendRegistry struct {
	byHarness map[HarnessID]registeredBackend
}

func NewBackendRegistry(registrations []Registration) (*BackendRegistry, error) {
	if len(registrations) == 0 || len(registrations) > 64 {
		return nil, ErrInvalidRegistry
	}
	registry := &BackendRegistry{byHarness: make(map[HarnessID]registeredBackend, len(registrations))}
	for _, registration := range registrations {
		configured, backend := registration.Configured, registration.Backend
		if !configured.valid() || backend == nil || backend.ID() != configured.BackendID ||
			backend.Version() != configured.BackendVersion {
			return nil, ErrInvalidRegistry
		}
		if _, duplicate := registry.byHarness[configured.HarnessID]; duplicate {
			return nil, ErrInvalidRegistry
		}
		configured.Capabilities = normalizedCapabilities(configured.Capabilities)
		registry.byHarness[configured.HarnessID] = registeredBackend{configured, backend}
	}
	return registry, nil
}

func NewBuiltInBackendRegistry(registrations []Registration) (*BackendRegistry, error) {
	if len(registrations) != len(BuiltInHarnessIDs()) {
		return nil, ErrInvalidRegistry
	}
	registry, err := NewBackendRegistry(registrations)
	if err != nil {
		return nil, err
	}
	for _, harnessID := range BuiltInHarnessIDs() {
		if _, _, ok := registry.Resolve(harnessID); !ok {
			return nil, ErrInvalidRegistry
		}
	}
	return registry, nil
}

func (registry *BackendRegistry) Resolve(harnessID HarnessID) (ConfiguredHarness, Backend, bool) {
	if registry == nil {
		return ConfiguredHarness{}, nil, false
	}
	registered, ok := registry.byHarness[harnessID]
	if !ok {
		return ConfiguredHarness{}, nil, false
	}
	configured := registered.configured
	configured.Capabilities = append([]Capability(nil), configured.Capabilities...)
	return configured, registered.backend, true
}
