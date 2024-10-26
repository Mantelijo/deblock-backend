package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mantelijo/deblock-backend/internal/chain"
	"github.com/Mantelijo/deblock-backend/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestHttpApiServer(t *testing.T) {

	makeServer := func() (*httptest.Server, *httpServer) {
		s := &httpServer{
			txTracker: nil,
		}
		router := http.NewServeMux()
		s.registerRoutes(router)
		return httptest.NewServer(router), s
	}

	t.Run("post /tracked-wallets - bad request", func(t *testing.T) {
		server, _ := makeServer()
		defer server.Close()

		req, err := http.NewRequest(http.MethodPost, server.URL+"/tracked-wallets", nil)
		assert.NoError(t, err)
		resp, err := server.Client().Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
	t.Run("post /tracked-wallets - failed to register wallet", func(t *testing.T) {
		server, s := makeServer()
		defer server.Close()

		mockTracker := mocks.NewWalletTransactionTracker(t)
		mockTracker.EXPECT().
			TrackWallet(
				"bb",
				chain.SolanaMainnet,
			).
			Return(
				assert.AnError,
			)
		s.txTracker = mockTracker

		req, err := http.NewRequest(http.MethodPost, server.URL+"/tracked-wallets",
			bytes.NewBuffer([]byte(`{
				"user_id": 43,
				"ethereum_wallet": "",
				"bitcoin_wallet": "",
				"solana_wallet": "bb"
				}
				`)),
		)
		assert.NoError(t, err)
		resp, err := server.Client().Do(req)
		assert.NoError(t, err)
		respText, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Contains(
			t, string(respText),
			"failed to register wallet tracking for solana_mainnet",
		)
	})

	t.Run("post /tracked-wallets - success", func(t *testing.T) {
		server, s := makeServer()
		defer server.Close()

		mockTracker := mocks.NewWalletTransactionTracker(t)
		mockTracker.EXPECT().
			TrackWallet(
				"aa",
				chain.EthereumMainnet,
			).
			Return(
				nil,
			)
		mockTracker.EXPECT().
			TrackWallet(
				"bb",
				chain.Bitcoin,
			).
			Return(
				nil,
			)
		mockTracker.EXPECT().
			TrackWallet(
				"cc",
				chain.SolanaMainnet,
			).
			Return(
				nil,
			)

		s.txTracker = mockTracker

		req, err := http.NewRequest(http.MethodPost, server.URL+"/tracked-wallets",
			bytes.NewBuffer([]byte(`{
				"user_id": 43,
				"ethereum_wallet": "aa",
				"bitcoin_wallet": "bb",
				"solana_wallet": "cc"
				}
				`)),
		)
		assert.NoError(t, err)
		resp, err := server.Client().Do(req)
		assert.NoError(t, err)
		respText, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(
			t, string(respText),
			"OK",
		)
	})
	t.Run("delete /tracked-wallets - bad request", func(t *testing.T) {
		server, _ := makeServer()
		defer server.Close()

		req, err := http.NewRequest(http.MethodDelete, server.URL+"/tracked-wallets", nil)
		assert.NoError(t, err)
		resp, err := server.Client().Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
	t.Run("delete /tracked-wallets - failed to deregister wallet", func(t *testing.T) {
		server, s := makeServer()
		defer server.Close()

		mockTracker := mocks.NewWalletTransactionTracker(t)
		mockTracker.EXPECT().
			UntrackWallet(
				"bb",
				chain.SolanaMainnet,
			).
			Return(
				assert.AnError,
			)
		s.txTracker = mockTracker

		req, err := http.NewRequest(http.MethodDelete, server.URL+"/tracked-wallets",
			bytes.NewBuffer([]byte(`{
				"user_id": 43,
				"ethereum_wallet": "",
				"bitcoin_wallet": "",
				"solana_wallet": "bb"
				}
				`)),
		)
		assert.NoError(t, err)
		resp, err := server.Client().Do(req)
		assert.NoError(t, err)
		respText, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Contains(
			t, string(respText),
			"failed to deregister wallet tracking for solana_mainnet",
		)
	})

	t.Run("delete /tracked-wallets - success", func(t *testing.T) {
		server, s := makeServer()
		defer server.Close()

		mockTracker := mocks.NewWalletTransactionTracker(t)
		mockTracker.EXPECT().
			UntrackWallet(
				"aa",
				chain.EthereumMainnet,
			).
			Return(
				nil,
			)
		mockTracker.EXPECT().
			UntrackWallet(
				"bb",
				chain.Bitcoin,
			).
			Return(
				nil,
			)
		mockTracker.EXPECT().
			UntrackWallet(
				"cc",
				chain.SolanaMainnet,
			).
			Return(
				nil,
			)

		s.txTracker = mockTracker

		req, err := http.NewRequest(http.MethodDelete, server.URL+"/tracked-wallets",
			bytes.NewBuffer([]byte(`{
				"user_id": 43,
				"ethereum_wallet": "aa",
				"bitcoin_wallet": "bb",
				"solana_wallet": "cc"
				}
				`)),
		)
		assert.NoError(t, err)
		resp, err := server.Client().Do(req)
		assert.NoError(t, err)
		respText, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(
			t, string(respText),
			"OK",
		)
	})

}
