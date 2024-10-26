package chain

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func NewEthereumMainnetSubscriber(rpcUrl string, opts ...EthereumMainnetSubscriberOption) *ethereumMainnetSubscriber {
	e := &ethereumMainnetSubscriber{
		rpcUrl:            rpcUrl,
		registeredWallets: make(map[common.Address]bool),
	}

	for _, opt := range opts {
		opt.Apply(e)
	}

	return e
}

type subscribeNewHeadFn func(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
type blockByNumberFn func(ctx context.Context, number *big.Int) (*types.Block, error)

var _ TransactionSubscriber = (*ethereumMainnetSubscriber)(nil)

type ethereumMainnetSubscriber struct {
	rpcUrl string
	// Options that will be applied to rpc client in Init
	rpcClientOpts []rpc.ClientOption

	registeredWallets map[common.Address]bool
	// registeredWallets mutex
	mu sync.RWMutex

	c             *ethclient.Client
	chainId       *big.Int
	defaultSigner types.Signer

	subscribeNewHead subscribeNewHeadFn
	blockByNumber    blockByNumberFn
}

func (e *ethereumMainnetSubscriber) Init() error {
	rpcClient, err := rpc.DialContext(context.Background(), e.rpcUrl)
	if err != nil {
		return fmt.Errorf("failed to dial rpc: %w", err)
	}
	e.c = ethclient.NewClient(rpcClient)

	// Attempt to fetch some initial info
	chainId, err := e.c.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get chain id: %w", err)
	}
	e.chainId = chainId
	block, err := e.c.BlockByNumber(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	signer := types.MakeSigner(params.MainnetChainConfig, block.Number(), block.Time())
	e.defaultSigner = signer

	e.subscribeNewHead = e.c.SubscribeNewHead
	e.blockByNumber = e.c.BlockByNumber

	slog.Info("initialized ethereum mainnet subscriber",
		slog.String("rpc_url", e.rpcUrl),
	)

	return nil
}

func (e *ethereumMainnetSubscriber) Start() (<-chan *TrackedWalletEvent, <-chan error) {
	outEvents := make(chan *TrackedWalletEvent)
	outErrors := make(chan error)

	go func() {

		h := make(chan *types.Header)
		sub, err := e.subscribeNewHead(context.Background(), h)
		if err != nil {
			outErrors <- fmt.Errorf("failed to subscribe to new head: %w", err)
			return
		}

		for {
			select {
			case err := <-sub.Err():
				slog.Error("subscription error",
					slog.Any("error", err),
					slog.String("chain", string(e.Name())),
				)
				outErrors <- err

				// TODO retry and recreate the sub

			case newHead := <-h:
				slog.Info("received new block headers",
					slog.Any("block_number", newHead.Number.Uint64()),
				)

				block, err := e.blockByNumber(context.Background(), newHead.Number)
				if err != nil {
					slog.Error("failed to get block by number", slog.Any("error", err))

					// TODO send signal to retry, or inspect the error and
					// decide what to do next.

				} else {
					for _, tx := range block.Transactions() {
						to := tx.To()
						hash := tx.Hash()
						fees := big.NewInt(int64(tx.GasPrice().Uint64() * tx.Gas()))
						amount := tx.Value()
						wallet, err := types.Sender(
							e.defaultSigner, tx,
						)
						if err != nil {
							slog.Error("failed to recover public key",
								slog.Any("error", err),
								slog.String("tx_hash", hash.String()),
							)
							continue
						}

						// Check whether tx involves tracked wallets
						e.mu.RLock()
						_, okSender := e.registeredWallets[wallet]
						okRecipient := false
						if to != nil {
							_, ok := e.registeredWallets[*to]
							okRecipient = ok
						}
						e.mu.RUnlock()

						if okSender || okRecipient {
							outEvents <- &TrackedWalletEvent{
								ChainName:   e.Name(),
								Source:      wallet.String(),
								Destination: to.String(),
								Amount:      amount,
								Fees:        fees,
							}
						}
					}
					slog.Info(
						"processed a block",
						slog.String("chain", string(e.Name())),
					)
				}
			}
		}
	}()

	return outEvents, outErrors
}

func (e *ethereumMainnetSubscriber) TrackWallet(wallet string) error {
	address, err := validateEvmWallet(wallet)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.registeredWallets[address] = true

	return nil
}

func (e *ethereumMainnetSubscriber) UntrackWallet(wallet string) error {
	address, err := validateEvmWallet(wallet)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.registeredWallets, address)

	return nil
}

func (e *ethereumMainnetSubscriber) Name() ChainName {
	return EthereumMainnet
}

type EthereumMainnetSubscriberOption interface {
	Apply(*ethereumMainnetSubscriber)
}

type WithRpcClientOptions struct {
	Opts []rpc.ClientOption
}

func (w WithRpcClientOptions) Apply(e *ethereumMainnetSubscriber) {
	e.rpcClientOpts = w.Opts
}

func validateEvmWallet(wallet string) (common.Address, error) {
	if !common.IsHexAddress(wallet) {
		return common.Address{}, fmt.Errorf("invalid ethereum wallet address")
	}
	return common.HexToAddress(wallet), nil
}
