# config

Generic ENV loader for Alepsios services.

```go
import "github.com/ibednov/go-lepsios/config"

type Config struct {
    HTTPPort int `envconfig:"HTTP_PORT" default:"8080"`
}

var cfg Config
config.MustLoad(&cfg)
```
