# AI provider sidecar

This module owns provider-neutral request/result contracts, the registry, and
the concrete GigaChat, Yandex, OpenRouter, and OpenAI credential strategies.
The strategies validate their provider-specific credentials and use a host
builder for transport configuration. A host domain keeps its own prompts, conversation history,
knowledge retrieval, usage accounting, persistence, credentials encryption, and
fallback policy; it adapts those domain types to `ai.Request` and registers one
strategy per provider.

Strategies own provider identity and credential validation. Host builders
provide clients with the domain's transport configuration; provider adapters
own the vendor request/response protocol. The library does not read environment
variables, persist secrets, choose a model, or decide when to fall back.

```go
registry, err := ai.NewRegistry(
    ai.NewOpenAIStrategy(openAIProviderBuilder),
    ai.NewYandexStrategy(yandexProviderBuilder),
)
provider, err := registry.New("openai", ai.Credentials{"api_key": key})
result, err := provider.Chat(ctx, ai.Request{Model: model, SystemPrompt: prompt, Message: text})
```
