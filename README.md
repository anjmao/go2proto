# go2proto

Generate Protobuf messages from given go structs. No RPC, not gogo syntax, just pure Protobuf messages.

### Example

```sh
git clone git@github.com:anjmao/go2proto.git
cd go2proto
go run main.go -f ${PWD}/example/out -p github.com/anjmao/go2proto/example/in
```

### Note

Generated code may not be perfect but since it just 180 lines of code you are free to adapt it yo your needs.
