package ai

const ProviderGigaChat = "gigachat"
const CredentialGigaChatAuthorizationKey = "authorization_key"

// NewGigaChatStrategy builds the GigaChat strategy with host-configured transport.
func NewGigaChatStrategy(build ProviderBuilder) Strategy {
	return credentialStrategy{name: ProviderGigaChat, required: []string{CredentialGigaChatAuthorizationKey}, build: build}
}
