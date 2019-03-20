# go2proto

Generate Protobuf messages from given go structs. No RPC, not gogo syntax, just pure Protobuf messages.

### Example

```sh
GO111MODULE=off go get -u github.com/anjmao/go2proto
go2proto -f ${PWD}/example/out -p github.com/anjmao/go2proto/example/in
```

### Note

Generated code may not be perfect but since it just 180 lines of code you are free to adapt it yo your needs.
