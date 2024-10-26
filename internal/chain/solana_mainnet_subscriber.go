package chain

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/rpc"
	"github.com/mr-tron/base58"
)

func NewSolanaMainnetSubscriber(rpcUrl string) *solanaMainnetSubscriber {
	return &solanaMainnetSubscriber{
		rpcUrl:            rpcUrl,
		registeredWallets: make(map[common.PublicKey]bool),
	}
}

var _ TransactionSubscriber = (*solanaMainnetSubscriber)(nil)

type solanaMainnetSubscriber struct {
	rpcUrl string
	c      *client.Client

	registeredWallets map[common.PublicKey]bool
	// registeredWallets mutex
	mu sync.RWMutex

	currentSlot uint64

	getSlot  func(context.Context) (uint64, error)
	getBlock func(context.Context, uint64) (*client.Block, error)
}

func (s *solanaMainnetSubscriber) Init() error {
	c := client.NewClient(s.rpcUrl)
	s.c = c

	s.getSlot = func(ctx context.Context) (uint64, error) {
		return c.GetSlotWithConfig(ctx, client.GetSlotConfig{
			Commitment: rpc.CommitmentFinalized,
		})
	}
	s.getBlock = func(ctx context.Context, slot uint64) (*client.Block, error) {
		return c.GetBlockWithConfig(ctx, slot, client.GetBlockConfig{
			Commitment: rpc.CommitmentFinalized,
		})
	}

	slot, err := s.getSlot(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get initial slot value: %w", err)
	}
	s.currentSlot = slot

	return nil
}

// Start starts the slot fetching loop and distributes all unprocessed blocks to
// a list of fetchBlock goroutines. Slots are fetched every second. Start
// complies to TransactionSubscriber interface contract and does not block.
func (s *solanaMainnetSubscriber) Start() (<-chan *TrackedWalletEvent, <-chan error) {
	outEvents, outErrors := make(chan *TrackedWalletEvent, 1000), make(chan error)

	go func() {
		for range time.Tick(time.Second) {
			slot, err := s.getSlot(context.Background())
			if err != nil {
				outErrors <- fmt.Errorf("failed to get slot: %w", err)
				continue
			}

			if slot <= s.currentSlot {
				continue
			}

			for i := s.currentSlot; i < slot; i++ {
				go func(slot uint64) {
					if err := s.fetchBlock(slot, outEvents); err != nil {
						slog.Error(
							"failed to fetch block",
							slog.String("chian", string(s.Name())),
							slog.Int64("slot", int64(slot)),
							slog.Any("error", err),
						)
						return
						// TODO better error handling, retry logic, etc.
					}
				}(i)
			}
			s.currentSlot = slot
		}
	}()

	return outEvents, outErrors
}

// Fetch block fetches a block for given slot and processes all transactions in
// it and sends them via provided out channel. Only transasctions with non 0
// transfer amount are processed.
func (s *solanaMainnetSubscriber) fetchBlock(slot uint64, out chan<- *TrackedWalletEvent) error {
	start := time.Now()
	block, err := s.getBlock(context.Background(), slot)
	fetchEnd := time.Since(start)

	if err != nil {
		return err
	}
	for _, tx := range block.Transactions {
		if tx.Meta == nil {
			continue
		}
		if len(tx.Meta.PostBalances) < 2 || len(tx.Meta.PreBalances) < 2 || len(tx.Transaction.Message.Accounts) < 2 {
			continue
		}

		// Process only transfer transactions with greater than 0 amount, as
		// others are not interesting.
		amount := tx.Meta.PostBalances[1] - tx.Meta.PreBalances[1]
		if amount <= 0 {
			continue
		}

		// hash := base58.Encode(tx.Transaction.Signatures[0])
		sender := tx.Transaction.Message.Accounts[0]
		recipient := tx.Transaction.Message.Accounts[1]
		fee := tx.Meta.Fee

		s.mu.RLock()
		_, okSender := s.registeredWallets[sender]
		_, okRecipient := s.registeredWallets[recipient]
		s.mu.RUnlock()

		if okSender || okRecipient {
			out <- &TrackedWalletEvent{
				ChainName:   s.Name(),
				Source:      sender.String(),
				Destination: recipient.String(),
				Amount:      big.NewInt(int64(amount)),
				Fees:        big.NewInt(int64(fee)),
			}
		}
	}
	slog.Info(
		"processed a block",
		slog.String("chain", string(s.Name())),
		slog.Duration("tx_processing_duration", time.Since(start)-fetchEnd),
		slog.Duration("block_fetch_duration", fetchEnd),
	)

	return nil
}

func (e *solanaMainnetSubscriber) TrackWallet(wallet string) error {
	address, err := validateSolanaWallet(wallet)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.registeredWallets[address] = true

	return nil
}

func (e *solanaMainnetSubscriber) UntrackWallet(wallet string) error {
	address, err := validateSolanaWallet(wallet)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.registeredWallets, address)

	return nil
}

func (s *solanaMainnetSubscriber) Name() ChainName {
	return SolanaMainnet
}

func validateSolanaWallet(wallet string) (common.PublicKey, error) {
	b, err := base58.Decode(wallet)
	if err != nil {
		return common.PublicKey{}, fmt.Errorf("invalid wallet address: %w", err)
	}

	return common.PublicKeyFromBytes(b), nil
}