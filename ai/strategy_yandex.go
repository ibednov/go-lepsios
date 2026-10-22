package ai

const ProviderYandex = "yandex"
const CredentialYandexAPIKey = "api_key"
const CredentialYandexFolderID = "folder_id"

// NewYandexStrategy builds the Yandex strategy with host-configured transport.
func NewYandexStrategy(build ProviderBuilder) Strategy {
	return credentialStrategy{name: ProviderYandex,
		required: []string{CredentialYandexAPIKey, CredentialYandexFolderID}, build: build}
}
