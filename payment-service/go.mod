module payment-service

go 1.25.4

require (
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.12.0
	github.com/youruser/ap2-generated-contracts v0.0.0
	google.golang.org/grpc v1.65.0
	google.golang.org/protobuf v1.36.10
)

require (
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
)

replace github.com/youruser/ap2-generated-contracts => ../generated-contracts
