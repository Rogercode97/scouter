package main
import (
	"context"
	"fmt"
	"github.com/Rogercode97/scouter/internal/store"
)
func main() {
	s, err := store.NewStore(context.Background(), "test_db.sqlite")
	if err != nil { panic(err) }
	err = s.SaveFileIndex(context.Background(), &store.FileIndex{Path: "a.go", Project: "p"})
	fmt.Println("FileIndex A:", err)
	err = s.SaveSymbol(context.Background(), &store.Symbol{Name: "A", Path: "a.go"})
	fmt.Println("Symbol A:", err)
}
