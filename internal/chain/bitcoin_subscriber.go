package chain

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"golang.org/x/exp/slog"
)

func NewBitcoinSubscriber(rpcUrl string) *bitcoinSubscriber {
	return &bitcoinSubscriber{
		rpcUrl: rpcUrl,
		// Wallets are stored as lowercase strings
		registeredWallets: make(map[string]bool),
	}
}

var _ TransactionSubscriber = (*solanaMainnetSubscriber)(nil)

type bitcoinSubscriber struct {
	rpcUrl string
	c      *rpcclient.Client

	registeredWallets map[string]bool
	// registeredWallets mutex
	mu sync.RWMutex

	lastBlockNum int64
}

func (b *bitcoinSubscriber) Init() error {
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Host:         b.rpcUrl,
		HTTPPostMode: true,
		User:         "none",
		Pass:         "none",
	}, nil)
	if err != nil {
		return err
	}
	b.c = client

	latestBlock, err := b.c.GetBlockCount()
	if err != nil {
		return fmt.Errorf("failed to get initial block count: %v", err)
	}
	// sub 1 for first time run
	b.lastBlockNum = latestBlock - 1

	slog.Info("initialized bitcoin subscriber",
		slog.String("rpc_url", b.rpcUrl),
	)

	return nil
}

func (b *bitcoinSubscriber) Start() (<-chan *TrackedWalletEvent, <-chan error) {
	outEvents := make(chan *TrackedWalletEvent)
	outErrs := make(chan error)

	go func() {
		// Bitcoin block time is ~10 minutes, so polling every 15s for new
		// blocks should be more than fine.
		for range time.Tick(15 * time.Second) {
			latestBlock, err := b.c.GetBlockCount()
			if err != nil {
				outErrs <- fmt.Errorf("failed to get block count: %w", err)
			}

			// Make sure we don't repeatedly process the same block
			if b.lastBlockNum < latestBlock {
				b.lastBlockNum = latestBlock
			} else {
				continue
			}

			blockHash, err := b.c.GetBlockHash(latestBlock)
			if err != nil {
				outErrs <- fmt.Errorf("failed to get block hash: %w", err)
			}
			start := time.Now()
			fullBlock, err := b.c.GetBlock(blockHash)
			if err != nil {
				outErrs <- fmt.Errorf("failed to get block info: %w", err)
			}
			slog.Info("fetched full bitcoin block",
				slog.String("block_hash", blockHash.String()),
				slog.Duration("duration", time.Since(start)),
				slog.Int("num_tx", len(fullBlock.Transactions)),
			)

			// TODO: potential improvement is to use a pool of worker goroutines
			// to process txs
			for _, tx := range fullBlock.Transactions {
				tx.TxHash()

				inAmounts := []int64{}
				inAmountTotal := int64(0)
				outAmounts := []int64{}
				outAmountTotal := int64(0)

				inWallets := []string{}
				outWallets := []string{}

				// Parse input transactions, fetch wallets from prev out,
				// amounts, etc.
				for _, txIn := range tx.TxIn {
					prevIndex := txIn.PreviousOutPoint.Index
					prevHash := txIn.PreviousOutPoint.Hash
					prevTx, err := b.c.GetRawTransaction(&prevHash)
					if err != nil {
						slog.Error("failed to get raw bitcoin transaction", slog.Any("error", err))
						continue
					}
					prevTxOut := prevTx.MsgTx().TxOut[prevIndex]
					_, addrs, _, err := txscript.ExtractPkScriptAddrs(prevTxOut.PkScript, &chaincfg.MainNetParams)
					if err != nil || len(addrs) < 1 {
						continue
					}
					inAmounts = append(inAmounts, prevTxOut.Value)
					inAmountTotal += prevTxOut.Value
					inWallets = append(inWallets, addrs[0].String())
				}

				// Same for outputs
				for _, txOut := range tx.TxOut {
					_, addrs, _, err := txscript.ExtractPkScriptAddrs(txOut.PkScript, &chaincfg.MainNetParams)
					if err != nil || len(addrs) < 1 {
						continue
					}
					outAmounts = append(outAmounts, txOut.Value)
					outAmountTotal += txOut.Value
					outWallets = append(outWallets, addrs[0].String())
				}

				fees := inAmountTotal - outAmountTotal

				// For each out wallet, let's send a TrackedWalletEvent
				sources := strings.Join(inWallets, ",")
				for i, outWallet := range outWallets {
					b.mu.RLock()
					_, ok := b.registeredWallets[strings.ToLower(outWallet)]
					b.mu.RUnlock()

					if ok {
						// Calculate fractional fee and total amount for current
						// out wallet
						currentOutputAmount := int64(0)
						currentOutputFees := int64(0)
						if outAmountTotal > 0 && outAmounts[i] > 0 {
							p := float64(outAmounts[i]) / float64(outAmountTotal)
							currentOutputAmount = int64(float64(outAmountTotal) * p)
							currentOutputFees = int64(float64(fees) * p)
						}

						outEvents <- &TrackedWalletEvent{
							ChainName:   Bitcoin,
							Source:      sources,
							Destination: outWallet,
							Amount:      big.NewInt(currentOutputAmount),
							Fees:        big.NewInt(currentOutputFees),
						}
					}
				}

			}

		}
	}()

	return outEvents, outErrs
}

func (b *bitcoinSubscriber) TrackWallet(wallet string) error {
	a, err := validateBtcAddress(wallet)
	if err != nil {
		return fmt.Errorf("invalid btc address: %w", err)
	}

	b.mu.Lock()
	b.registeredWallets[strings.ToLower(a.String())] = true
	b.mu.Unlock()

	return nil
}

func (b *bitcoinSubscriber) UntrackWallet(wallet string) error {
	a, err := validateBtcAddress(wallet)
	if err != nil {
		return fmt.Errorf("invalid btc address: %w", err)
	}

	b.mu.Lock()
	delete(b.registeredWallets, strings.ToLower(a.String()))
	b.mu.Unlock()

	return nil
}

func (b *bitcoinSubscriber) Name() ChainName {
	return Bitcoin
}

func validateBtcAddress(address string) (btcutil.Address, error) {
	return btcutil.DecodeAddress(address, &chaincfg.MainNetParams)
}
