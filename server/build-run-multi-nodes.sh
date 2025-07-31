go run cmd/main.go --config config/multi-node-1.yaml &
go run cmd/main.go --config config/multi-node-2.yaml &
go run cmd/main.go --config config/multi-node-3.yaml &

wait