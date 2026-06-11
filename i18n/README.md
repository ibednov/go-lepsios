# i18n

Localization bundle. Translation JSON files stay in the service.

```go
bundle, _ := i18n.NewBundle("en")
_ = bundle.LoadMessages("en", enJSON)
loc := bundle.Localizer("ru")
msg := loc.T("errors.auth.not_found")
```
