# db

PostgreSQL pool and sqlboiler-compatible Transactor.

```go
import "github.com/ibednov/go-lepsios/db"

sqlDB, _ := db.ProvideDB(db.DBConfig{URL: os.Getenv("DATABASE_URL")})
tx := db.NewTransactor(sqlDB)
```
