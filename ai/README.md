# AI provider sidecar

This module owns provider-neutral request/result contracts and provider strategy
registration. A host domain keeps its own prompts, conversation history,
knowledge retrieval, usage accounting, persistence, credentials encryption, and
fallback policy; it adapts those domain types to `ai.Request` and registers one
strategy per provider.

Strategies own provider credential validation and client construction. Providers
own the vendor request/response protocol. The library does not read environment
variables, persist secrets, choose a model, or decide when to fall back.

```go
registry, err := ai.NewRegistry(openAIStrategy, yandexStrategy)
provider, err := registry.New("openai", ai.Credentials{"api_key": key})
result, err := provider.Chat(ctx, ai.Request{Model: model, SystemPrompt: prompt, Message: text})
```
