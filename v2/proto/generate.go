// Package proto contains the protobuf definitions for the Zentinel Agent Protocol v2.
//
// To regenerate the Go code from the proto definitions, run:
//
//	go generate ./v2/proto/
//
// This requires protoc and the protoc-gen-go / protoc-gen-go-grpc plugins:
//
//	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
package proto

//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative agent_v2.proto
