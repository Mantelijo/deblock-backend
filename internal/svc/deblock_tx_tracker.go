package svc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/IBM/sarama"
	"github.com/Mantelijo/deblock-backend/internal/api"
	"github.com/Mantelijo/deblock-backend/internal/chain"
	"github.com/Mantelijo/deblock-backend/internal/config"
)

func RunDeblockTxTracker() {
	// Init logger
	logger := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})
	slog.SetDefault(slog.New(logger))

	// Parse the required env values
	// Ethereum RPC, Solana RPC, Bitcoin RPC, Kafka broker endpoint
	if err := config.LoadRequiredEnv(); err != nil {
		slog.Error(
			"failed to load required env values",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

	// Initialize the chain subscribers
	ethereum := chain.NewEthereumMainnetSubscriber(config.Global.String(config.RPC_URL_ETHEREUM))
	solana := chain.NewSolanaMainnetSubscriber(config.Global.String(config.RPC_URL_SOLANA))
	bitcoin := chain.NewBitcoinSubscriber(config.Global.String(config.RPC_URL_BITCOIN))
	subManager := chain.NewSubsciberManager()
	if err := subManager.RegisterSubscribers(ethereum, solana, bitcoin); err != nil {
		slog.Error(
			"failed to register subscriber",
			slog.Any("error", err),
		)
		return
	}

	errorsCh := make(chan error)

	// Start all subscribers
	eventsSink := make(chan *chain.TrackedWalletEvent)
	go func() {
		err := subManager.StartAll(eventsSink)
		if err != nil {
			errorsCh <- fmt.Errorf("subscriber failure: %w", err)
		}
	}()

	// Start the api server
	var apiServer api.Server = api.NewHttpServer(
		config.Global.String(config.API_BIND_ADDR),
		config.Global.String(config.API_PORT),
		subManager,
	)
	go func() {
		if err := apiServer.Serve(); err != nil {
			errorsCh <- fmt.Errorf("failed to start api server: %w", err)
		}
	}()

	kafkaProd, err := InitKafka()
	if err != nil {
		slog.Info(
			"kafka producer not initialized",
			slog.Any("error", err),
		)
	}
	if kafkaProd != nil {
		go func() {
			for err := range kafkaProd.Errors() {
				slog.Error(
					"failed to produce message to kafka",
					slog.Any("error", err),
				)
			}
		}()
	}

	for {
		select {
		case err := <-errorsCh:
			slog.Error(
				"service encountered critical error",
				slog.Any("error", err),
			)
			return
		case event := <-eventsSink:
			slog.Info(
				"received new event",
				slog.Any("event", event),
			)

			// If kafka is enabled - push the event to kafka topic
			if kafkaProd != nil {
				eventJson, err := json.Marshal(event)
				if err == nil {
					kafkaProd.Input() <- &sarama.ProducerMessage{
						Topic: "deblock_tx_tracker",
						Value: sarama.StringEncoder(eventJson),
					}
				}

			}
		}
	}
}

func InitKafka() (sarama.AsyncProducer, error) {
	brokerUrl := config.Global.String(config.KAFKA_BROKER_URL)
	slog.Info("kafka broker url", slog.String("url", brokerUrl))
	if brokerUrl != "" {
		cfg := sarama.NewConfig()
		prod, err := sarama.NewAsyncProducer([]string{brokerUrl}, cfg)
		if err != nil {
			return nil, err
		}

		return prod, err
	}

	slog.Info(
		"kafka producer not initialized, env KAFKA_BROKER_URL value is empty",
	)

	return nil, nil
}
