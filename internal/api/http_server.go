package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/Mantelijo/deblock-backend/internal/chain"
)

func NewHttpServer(addr, port string, txTracker chain.WalletTransactionTracker) *httpServer {
	return &httpServer{
		addr:      addr,
		port:      port,
		txTracker: txTracker,
	}
}

type httpServer struct {
	addr string
	port string

	txTracker chain.WalletTransactionTracker

	l net.Listener
}

func (s *httpServer) Serve() error {
	router := http.NewServeMux()
	s.registerRoutes(router)
	return s.startServer(router)
}

func (s *httpServer) startServer(r *http.ServeMux) error {
	bindAddr := net.JoinHostPort(s.addr, s.port)

	l, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return err
	}
	s.port = strconv.Itoa(l.Addr().(*net.TCPAddr).Port)

	slog.Info("starting http api server",
		slog.String("addr", s.addr),
		slog.String("port", s.port),
	)

	return http.Serve(l, r)
}

func (s *httpServer) Close() error {
	return s.l.Close()
}

func (s *httpServer) registerRoutes(r *http.ServeMux) {
	r.HandleFunc("POST /tracked-wallets", s.trackWallet)
	r.HandleFunc("DELETE /tracked-wallets", s.untrackWallet)
}

type TrackWalletRequest struct {
	UserID         int    `json:"user_id"`
	EthereumWallet string `json:"ethereum_wallet"`
	BitcoinWallet  string `json:"bitcoin_wallet"`
	SolanaWallet   string `json:"solana_wallet"`
}

func (s *httpServer) trackWallet(w http.ResponseWriter, r *http.Request) {
	reqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read request body", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	req := &TrackWalletRequest{}
	if err := json.Unmarshal(reqBytes, req); err != nil {
		slog.Error("failed to parse request", slog.Any("error", err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("failed to parse request"))
		return
	}

	walletsToTrack := [][2]string{
		{req.EthereumWallet, string(chain.EthereumMainnet)},
		{req.BitcoinWallet, string(chain.Bitcoin)},
		{req.SolanaWallet, string(chain.SolanaMainnet)},
	}

	for _, tuple := range walletsToTrack {
		chainName := chain.ChainName(tuple[1])
		wallet := tuple[0]
		if len(wallet) > 0 {
			if err := s.txTracker.TrackWallet(wallet, chainName); err != nil {
				slog.Error("failed to track wallet",
					slog.String("chain", string(chainName)),
					slog.Any("error", err),
				)
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "failed to register wallet tracking for %s", chainName)
				return
			}
			slog.Info("registered wallet for tracking",
				slog.String("chain", string(chainName)),
				slog.String("wallet", wallet),
			)
		}

	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

}

func (s *httpServer) untrackWallet(w http.ResponseWriter, r *http.Request) {
	reqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read request body", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	req := &TrackWalletRequest{}
	if err := json.Unmarshal(reqBytes, req); err != nil {
		slog.Error("failed to parse request", slog.Any("error", err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("failed to parse request"))
		return
	}

	walletsToTrack := [][2]string{
		{req.EthereumWallet, string(chain.EthereumMainnet)},
		{req.BitcoinWallet, string(chain.Bitcoin)},
		{req.SolanaWallet, string(chain.SolanaMainnet)},
	}

	for _, tuple := range walletsToTrack {
		chainName := chain.ChainName(tuple[1])
		wallet := tuple[0]
		if len(wallet) > 0 {
			if err := s.txTracker.UntrackWallet(tuple[0], chainName); err != nil {
				slog.Error("failed to untrack a wallet",
					slog.String("chain", string(chainName)),
					slog.Any("error", err),
				)
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "failed to deregister wallet tracking for %s", chainName)
				return
			}
			slog.Info("deregistered wallet from tracking",
				slog.String("chain", string(chainName)),
				slog.String("wallet", wallet),
			)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
