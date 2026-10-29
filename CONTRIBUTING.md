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

## Участие в разработке (RU)

Вклад приветствуется: создавайте issues, присылайте коммиты и pull requests.

- Авторы принятых изменений указываются как соавторы (например, в AUTHORS или README).
- Отправляя вклад, вы соглашаетесь, что он распространяется на тех же условиях, что и проект: [PolyForm Noncommercial License 1.0.0](LICENSE).
- Коммерческое использование возможно только с письменного разрешения автора. Связаться можно через [alexbednov.dev](https://alexbednov.dev).

## Contributing (EN)

Contributions are welcome: open issues, send commits and pull requests.

- Authors of accepted changes are credited as co-authors (for example, in AUTHORS or the README).
- By contributing, you agree that your contribution is licensed under the same terms as the project: the [PolyForm Noncommercial License 1.0.0](LICENSE).
- Commercial use requires the author's written permission. Contact via [alexbednov.dev](https://alexbednov.dev).
