for v in v0.20.0 v0.21.0 v0.22.0 v0.23.0 v0.24.0 v0.19.0 v0.18.0; do
    echo "Testing $v"
    go get github.com/ncruces/go-sqlite3@$v
    go test ./internal/store/... && echo "$v WORKED" && break
done
