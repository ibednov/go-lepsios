package ai

const ProviderOpenRouter = "openrouter"
const CredentialOpenRouterAPIKey = "openrouter_api_key"

// NewOpenRouterStrategy builds the OpenRouter strategy with host-configured transport.
func NewOpenRouterStrategy(build ProviderBuilder) Strategy {
	return credentialStrategy{name: ProviderOpenRouter,
		required: []string{CredentialOpenRouterAPIKey}, build: build}
}
