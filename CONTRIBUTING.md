# Contributing

## Commit messages

Write commit subjects in English, in the imperative or concise present tense, and keep them at 70 characters or fewer.

Use this format:

```text
<type> (<domain>/<scope>/<module>): <short description>
```

There is one space between the type and the opening parenthesis. Do not use `feat(scope): ...`.

Common types are `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `chore`, `build`, and `ci`. Use lowercase path-like scopes from the shared-code domain to the module, for example `shared/billing/payment`, `shared/auth/password`, or `shared/i18n/locale`. Omit levels only when they do not add useful context.

Examples:

```text
feat (shared/billing/booking): support booking products
fix (shared/i18n/locale): match accepted languages
docs (shared/auth/session): explain refresh rotation
```

Keep each commit focused on one logical change. Add a body only when context or a breaking change needs explanation. Do not put work labels such as `[front]` or `[back]` in the subject.
