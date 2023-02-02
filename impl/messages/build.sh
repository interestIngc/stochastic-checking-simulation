protoc -I="/Users/veronika/go/pkg/mod/github.com/asynkron/protoactor-go@v0.0.0-20221002142108-880b460fcd1f/actor" \
--go_out=. --go_opt=paths=source_relative --proto_path=. messages.proto
