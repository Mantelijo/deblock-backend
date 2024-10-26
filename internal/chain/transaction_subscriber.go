package chain

import "math/big"

// TransactionSubscriber subscribes to real time chain data for a particular blockchain.
type TransactionSubscriber interface {
	// Init initializes the subscriber, sets up any required connections and
	// prepares the subscriber to start receiving messages.
	Init() error

	// Start starts the subscriber. Returned channel will produce events
	// whenever a transaction for one of the registered wallets is received from
	// RPC provider. Start does not block.
	Start() (<-chan *TrackedWalletEvent, <-chan error)

	// TrackWallet starts to track transactions of provided wallet
	TrackWallet(wallet string) error

	// UntrackWallet stops tracking wallet's transactions
	UntrackWallet(wallet string) error

	// Name returns the chain name of given TransactionSubscriber
	Name() ChainName
}

// TrackedWalletEvent represents a tracked wallet event. For bitcoin events,
// Source and Destination will contain a string of comma separated addresses.
type TrackedWalletEvent struct {
	ChainName   ChainName
	Source      string
	Destination string
	Amount      *big.Int
	Fees        *big.Int
}

type ChainName string

const (
	EthereumMainnet ChainName = "ethereum_mainnet"
	Bitcoin         ChainName = "bitcoin"
	SolanaMainnet   ChainName = "solana_mainnet"
)
