package ai

const ProviderOpenAI = "openai"
const CredentialOpenAIAPIKey = "openai_api_key"

// NewOpenAIStrategy builds the OpenAI strategy with host-configured transport.
func NewOpenAIStrategy(build ProviderBuilder) Strategy {
	return credentialStrategy{name: ProviderOpenAI,
		required: []string{CredentialOpenAIAPIKey}, build: build}
}
