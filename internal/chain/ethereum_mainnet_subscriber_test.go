package chain

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	go_ethereuem_mocks "github.com/Mantelijo/deblock-backend/internal/mocks/go_ethereum"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/assert"
)

func TestEthereumMainnetSubscriberStart(t *testing.T) {
	tests := []struct {
		name             string
		subscribeNewHead subscribeNewHeadFn
		blockByNumberFn  blockByNumberFn
		wantEvents       []*TrackedWalletEvent
		wantErrs         []error
		trackWallets     []string
	}{
		{
			name: "failed sub",
			subscribeNewHead: func(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error) {
				return nil, assert.AnError
			},
			blockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {
				return nil, nil
			},
			wantEvents: nil,
			wantErrs: []error{
				fmt.Errorf("failed to subscribe to new head: %w", assert.AnError),
			},
		},
		{
			name: "gets headers, correctly processes event",
			subscribeNewHead: func(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error) {
				go func() {
					ch <- &types.Header{
						Number: big.NewInt(500),
					}
				}()

				sub := &go_ethereuem_mocks.MockGoEthereumSubscription{}
				sub.EXPECT().Err().Return(
					make(<-chan error),
				)

				return sub, nil
			},
			blockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {

				R, _ := big.NewInt(0).SetString("41381143044471666193394495856779718433748443387095402661844025890319923186141", 10)
				S, _ := big.NewInt(0).SetString("51098266734372285490093418638008504503442167242690029592223759640366292416179", 10)

				block := types.NewBlockWithHeader(
					&types.Header{
						Number: big.NewInt(500),
					},
				)
				block = block.WithBody(types.Body{
					Transactions: []*types.Transaction{
						// TX hash: "0x5bf0d5650d4df9e308a8ce1b3be8757746c532f7f111d3529e98ba74b873ea06"
						// FROM: 0x9642b23Ed1E01Df1092B92641051881a322F5D4E
						// TO: 0xeEa5b26B94E4e5bA416c9725e51aB755E2ddE107
						types.NewTx(&types.LegacyTx{
							Nonce:    257664,
							GasPrice: big.NewInt(7424228342),
							Gas:      50000,
							To: (func() *common.Address {
								a := common.HexToAddress("0xeEa5b26B94E4e5bA416c9725e51aB755E2ddE107")
								return &a
							})(),
							Value: big.NewInt(19220000000000000),
							Data:  []byte{},
							V:     big.NewInt(38),
							R:     R,
							S:     S,
						}),
					},
				})
				return block, nil
			},
			wantEvents: []*TrackedWalletEvent{
				{
					ChainName:   EthereumMainnet,
					Source:      "0x9642b23Ed1E01Df1092B92641051881a322F5D4E",
					Destination: "0xeEa5b26B94E4e5bA416c9725e51aB755E2ddE107",
					Amount:      big.NewInt(19220000000000000),
					Fees:        big.NewInt(371211417100000),
				},
			},
			wantErrs: []error{},
			trackWallets: []string{
				"0x9642b23Ed1E01Df1092B92641051881a322F5D4E",
			},
		},
		{
			name: "gets headers, correctly does not process event for non-tracked wallet",
			subscribeNewHead: func(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error) {
				go func() {
					ch <- &types.Header{
						Number: big.NewInt(500),
					}
				}()

				sub := &go_ethereuem_mocks.MockGoEthereumSubscription{}
				sub.EXPECT().Err().Return(
					make(<-chan error),
				)

				return sub, nil
			},
			blockByNumberFn: func(ctx context.Context, number *big.Int) (*types.Block, error) {

				R, _ := big.NewInt(0).SetString("41381143044471666193394495856779718433748443387095402661844025890319923186141", 10)
				S, _ := big.NewInt(0).SetString("51098266734372285490093418638008504503442167242690029592223759640366292416179", 10)

				block := types.NewBlockWithHeader(
					&types.Header{
						Number: big.NewInt(500),
					},
				)
				block = block.WithBody(types.Body{
					Transactions: []*types.Transaction{
						// TX hash: "0x5bf0d5650d4df9e308a8ce1b3be8757746c532f7f111d3529e98ba74b873ea06"
						// FROM: 0x9642b23Ed1E01Df1092B92641051881a322F5D4E
						// TO: 0xeEa5b26B94E4e5bA416c9725e51aB755E2ddE107
						types.NewTx(&types.LegacyTx{
							Nonce:    257664,
							GasPrice: big.NewInt(7424228342),
							Gas:      50000,
							To: (func() *common.Address {
								a := common.HexToAddress("0xeEa5b26B94E4e5bA416c9725e51aB755E2ddE107")
								return &a
							})(),
							Value: big.NewInt(19220000000000000),
							Data:  []byte{},
							V:     big.NewInt(38),
							R:     R,
							S:     S,
						}),
					},
				})
				return block, nil
			},
			wantEvents: []*TrackedWalletEvent{},
			wantErrs:   []error{},
			trackWallets: []string{
				"0xA642b23Ed1E01Df1092B92641051881a322F5D4E",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			e := NewEthereumMainnetSubscriber("http://dummy.net")

			// Manual init
			e.subscribeNewHead = tt.subscribeNewHead
			e.blockByNumber = tt.blockByNumberFn
			e.defaultSigner = types.NewCancunSigner(params.MainnetChainConfig.ChainID)
			e.chainId = params.MainnetChainConfig.ChainID

			events, errs := e.Start()

			gotEvents := make([]*TrackedWalletEvent, 0)
			gotErrors := make([]error, 0)

			if len(tt.trackWallets) > 0 {
				for _, wallet := range tt.trackWallets {
					err := e.TrackWallet(wallet)
					assert.NoError(t, err)
				}
			}

			done := make(chan struct{})
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
				defer cancel()
				for {
					select {
					case e := <-events:
						gotEvents = append(gotEvents, e)
						if len(gotEvents) == len(tt.wantEvents) {
							done <- struct{}{}
							return
						}
					case err := <-errs:
						gotErrors = append(gotErrors, err)
						if len(gotErrors) == len(tt.wantErrs) {
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

			if len(tt.wantEvents) > 0 {
				assert.Len(t, gotErrors, 0)
				assert.Equal(t, tt.wantEvents, gotEvents)
			}
			if len(tt.wantErrs) > 0 {
				assert.Equal(t, tt.wantErrs, gotErrors)
			}

		})
	}
}
