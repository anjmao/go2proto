# go2proto

Generate Protobuf messages from given go structs. No RPC, not gogo syntax, just pure Protobuf messages.

### Syntax
```
-f string
    Protobuf output file path. (default ".")
-filter string
    Filter by type names.
-p value
    Fully qualified path of packages to analyse. Relative paths ("./example/in") are allowed.
```

### Example

```sh
GO111MODULE=off go get -u github.com/anjmao/go2proto
go2proto -f ${PWD}/example/out -p github.com/anjmao/go2proto/example/in
```

You can omit the -f path to default to

### Note

Generated code may not be perfect but since it just 180 lines of code you are free to adapt it for your needs.
