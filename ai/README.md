# AI provider sidecar

This module owns provider-neutral request/result contracts, the registry, and
the concrete GigaChat, Yandex, OpenRouter, and OpenAI strategies, including
their HTTP protocols and credential validation. The host supplies HTTP clients
and endpoint overrides. A host domain keeps its own prompts, conversation history,
knowledge retrieval, usage accounting, persistence, credentials encryption, and
fallback policy; it adapts those domain types to `ai.Request` and registers one
strategy per provider.

Strategies own provider identity, credential validation, and vendor request/response
protocols. The host supplies transport configuration. The library does not read environment
variables, persist secrets, choose a model, or decide when to fall back.

The exported constructors are `NewGigaChatStrategy`, `NewYandexStrategy`,
`NewOpenRouterStrategy`, and `NewOpenAIStrategy`. They respectively implement
GigaChat OAuth and chat completions, Yandex Foundation Models, OpenRouter chat
completions, and the OpenAI Responses API with optional JSON Schema output.

```go
registry, err := ai.NewRegistry(
    ai.NewOpenAIStrategy(ai.HTTPConfig{Client: client}),
    ai.NewYandexStrategy(ai.HTTPConfig{Client: client}),
)
provider, err := registry.New("openai", ai.Credentials{"api_key": key})
result, err := provider.Chat(ctx, ai.Request{Model: model, SystemPrompt: prompt, Message: text})
```
