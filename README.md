# Leaderelection Go Library

The `leaderelection` Go library provides a mechanism for leader election among a group of distributed processes. This library ensures that only one process acts as the leader at any given time, which is useful for tasks that require a single point of control.

## Features

- Leader election using leases
- Graceful shutdown and leader takeover
- Lease expiration and automatic leader re-election
- Allows for custom lease store extensions (e.g., etcd, consul, redis.). In this repo you can find an in-memory lease store implementation example.

## Installation

To install the `leaderelection` library, use the following command:

```sh
go get github.com/rbroggi/leaderelection
```

## Usage


```go
package main

import (
	"context"
	"log"
	"time"
	"leaderelection"
	"leaderelection/inmemory"
)

func main() {
	store := inmemory.NewInMemoryLeaseStore()
	config := leaderelection.ElectorConfig{
		LeaseDuration:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		LeaseStore:      store,
		// The identity of the candidate - must be unique among all candidates
		// os.Hostname() can be used to get the hostname of the machine (pod name in k8s)
		CandidateID:     "candidate-1",
		ReleaseOnCancel: true,
		OnStartedLeading: func(ctx context.Context) {
			log.Println("Started leading")
		},
		OnStoppedLeading: func() {
			log.Println("Stopped leading")
		},
		OnNewLeader: func(identity string) {
			log.Printf("New leader elected: %s", identity)
		},
	}

	elector := leaderelection.NewElector(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := elector.Run(ctx)

	// Gracefully shutdown by cancelling the context
	cancel()
	// waiting for the elector to finish
	<-done
}
```

## Testing

To run the tests, use the following command:

```sh
go test ./...
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

