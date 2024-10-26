package config

// Environment variables used by the application
const (
	// Ethereum rpc url - should be websockets url
	RPC_URL_ETHEREUM = "RPC_URL_ETHEREUM"
	// Solana rpc url - http url
	RPC_URL_SOLANA = "RPC_URL_SOLANA"
	// Bitcoin rpc url - http url
	RPC_URL_BITCOIN = "RPC_URL_BITCOIN"

	// Http api port. Default is 8080
	API_PORT = "API_PORT"

	// Http api bind address. Default is 127.0.0.1
	API_BIND_ADDR = "API_BIND_ADDR"
)
