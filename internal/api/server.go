package api

// Server is API layer that accepts requests to track/untrack wallets
type Server interface {
	// Serve starts the API server. Serve blocks until the server is stopped or
	// an error is encoutered.
	Serve() error

	// Close stops the server and cleans up any resources.
	Close() error
}
