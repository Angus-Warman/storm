## Generic Function ORM

*(It actually uses generic methods, but "gmorm" doesn't work quite as well)*

Highly opinionated, [Modern-C](https://github.com/modernc-org/sqlite) SQLite-backed (no CGO), using the new generic methods released in [Go 1.27](https://go.dev/blog/go1.27).

Note: as-and-or-when [GORM](https://gorm.io/) decides to do the same thing, I'll be going back to using GORM. The [generics](https://gorm.io/docs/the_generics_way.html) GORM API is a valiant effort to work around the missing stair of generic methods. And it works pretty well! However, the new language capabilities are a big win for developer ergonomics.

### Getting Started

```go
import (
	"embed"
    "uuid"

	"github.com/Angus-Warman/storm"
)

type Record struct {
	ID    int // All structs *must* have an ID property
	Value string
}

type Account struct {
    ID     uuid.UUID
    UserID string // TODO: Foreign keys
}

type User struct {
    ID   string // Empty string is an error, int and uuid IDs are automatically populated
    Name string
}

//go:embed all:migrations
var migrations embed.FS

func demo() error {
	store, err := storm.New("data.db") // file path or :memory:
	if err != nil {
		return err
	}
	defer store.Close()

    // Automatic migrations run during the CreateTables step,
    // custom SQL files are needed for renaming/deleting columns
	if err := store.RunMigrations(migrations); err != nil {
		return err
	}

	if err := store.CreateTables(&Record{}, &User{}, &Account{}); err != nil {
		return err
	}

    // store.CreateTable[Record]() also works

	f, err := store.Create(Record{Value: "first"})
	if err != nil {
		return err
	}
    _, err = store.Create(Record{Value: "second"})
	if err != nil {
		return err
	}

	first, err := store.Find(Record{ID: f.ID})
	if err != nil {
		return err
	}

	records, err := store.Get[Record]().
		Where("Value = ?", "second").
		OrderBy("Value DESC").
		All() // All, First or Last
	if err != nil {
		return err
	}

	if count, err := store.Count[Record](); err != nil || count != 2 {
		return err
	}

	if _, err := store.Update[Record]().This(Record{ID: first.ID, Value: "renamed"}); err != nil {
		return err
	}

    if _, err := store.Delete[Record]().All() {
        return err
    }
  
    return nil
}
```
