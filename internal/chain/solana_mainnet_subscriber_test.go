package chain

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/blocto/solana-go-sdk/common"
	"github.com/blocto/solana-go-sdk/types"
	"github.com/stretchr/testify/assert"
)

func TestFetchBlock(t *testing.T) {
	acc1 := types.NewAccount() // sender
	acc2 := types.NewAccount() // receiver

	tests := []struct {
		name            string
		getBlcok        func(context.Context, uint64) (*client.Block, error)
		slot            uint64
		wantErr         string
		wantEvents      []*TrackedWalletEvent
		registerWallets []string
	}{
		{
			name: "failed to get block",
			getBlcok: func(ctx context.Context, slot uint64) (*client.Block, error) {
				return nil, assert.AnError
			},
			wantErr: assert.AnError.Error(),
		},
		{
			name: "transactions without sufficient data 0",
			getBlcok: func(ctx context.Context, slot uint64) (*client.Block, error) {
				b := &client.Block{
					Transactions: []client.BlockTransaction{
						{
							Meta: nil,
						},
					},
				}
				return b, nil
			},
			slot:       500,
			wantErr:    "",
			wantEvents: []*TrackedWalletEvent{},
		},
		{
			name: "transactions without sufficient data 1",
			getBlcok: func(ctx context.Context, slot uint64) (*client.Block, error) {
				b := &client.Block{
					Transactions: []client.BlockTransaction{
						{
							Meta: &client.TransactionMeta{
								PostBalances: []int64{1},
								PreBalances:  []int64{1},
							},
							Transaction: types.Transaction{
								Message: types.Message{
									Accounts: []common.PublicKey{},
								},
							},
						},
					},
				}
				return b, nil
			},
			slot:       500,
			wantErr:    "",
			wantEvents: []*TrackedWalletEvent{},
		},
		{
			name: "correctly returns events for tracked wallet",
			getBlcok: func(ctx context.Context, slot uint64) (*client.Block, error) {
				b := &client.Block{
					Transactions: []client.BlockTransaction{
						{
							Meta: &client.TransactionMeta{
								PostBalances: []int64{1000, 750},
								PreBalances:  []int64{1250, 500},
								Fee:          57,
							},
							Transaction: types.Transaction{
								Message: types.Message{
									Accounts: []common.PublicKey{
										acc1.PublicKey,
										acc2.PublicKey,
										acc2.PublicKey,
									},
								},
							},
						},
					},
				}
				return b, nil
			},
			slot: 500,
			wantEvents: []*TrackedWalletEvent{
				{
					ChainName:   SolanaMainnet,
					Source:      acc1.PublicKey.String(),
					Destination: acc2.PublicKey.String(),
					Amount:      big.NewInt(250),
					Fees:        big.NewInt(57),
				},
			},
			registerWallets: []string{
				acc1.PublicKey.String(),
			},
		},
		{
			name: "correctly returns no events for non-tracked wallet",
			getBlcok: func(ctx context.Context, slot uint64) (*client.Block, error) {
				b := &client.Block{
					Transactions: []client.BlockTransaction{
						{
							Meta: &client.TransactionMeta{
								PostBalances: []int64{1000, 750},
								PreBalances:  []int64{1250, 500},
								Fee:          57,
							},
							Transaction: types.Transaction{
								Message: types.Message{
									Accounts: []common.PublicKey{
										acc1.PublicKey,
										acc2.PublicKey,
										acc2.PublicKey,
									},
								},
							},
						},
					},
				}
				return b, nil
			},
			slot:            500,
			wantEvents:      []*TrackedWalletEvent{},
			registerWallets: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s := NewSolanaMainnetSubscriber("alchemy-or-other-rpc-url")
			s.getBlock = tt.getBlcok

			for _, w := range tt.registerWallets {
				err := s.TrackWallet(w)
				assert.NoError(t, err)
			}

			ch := make(chan *TrackedWalletEvent)
			chErr := make(chan error)

			go func() {
				err := s.fetchBlock(tt.slot, ch)
				if err != nil {
					chErr <- err
				}
			}()

			events := []*TrackedWalletEvent{}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			done := make(chan struct{})
			go func() {
				for {
					select {
					case e := <-ch:
						events = append(events, e)
						if len(events) == len(tt.wantEvents) {
							done <- struct{}{}
							return
						}
					case <-ctx.Done():
						done <- struct{}{}
						return
					}
				}
			}()
			<-done

			if tt.wantErr != "" {
				err := <-chErr
				assert.EqualError(t, err, tt.wantErr)
			} else {

				assert.Equal(t, tt.wantEvents, events)
			}
		})
	}
}
