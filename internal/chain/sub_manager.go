package chain

import (
	"fmt"
)

type WalletTransactionTracker interface {
	// TrackWallet starts tracking wallet's transactions within the given chain
	// subscriber.
	TrackWallet(wallet string, chain ChainName) error

	// UntrackWallet stops tracking wallet's transactions within the given chain
	// subscriber.
	UntrackWallet(wallet string, chain ChainName) error
}

// SubscriberManager manages all blockchain transaction subscribers within the
// application
type SubscriberManager interface {
	WalletTransactionTracker

	// RegisterSubscribers registers new subscribers and calls its Init.
	// RegisterSubscriber should not be called concurrently.
	RegisterSubscribers(subscribers ...TransactionSubscriber) error

	// StartAll accepts a sink which will receive all tracked wallet events from
	// all of the registered subscribers. StartAll blocks and exits with an
	// error if something goes wrong in one of the registered subscribers.
	StartAll(sink chan<- *TrackedWalletEvent) error
}

func NewSubsciberManager() SubscriberManager {
	return &mapSubManager{
		subs: make(map[ChainName]TransactionSubscriber),
	}
}

var _ SubscriberManager = (*mapSubManager)(nil)

type mapSubManager struct {
	subs map[ChainName]TransactionSubscriber
}

func (m *mapSubManager) RegisterSubscribers(subscribers ...TransactionSubscriber) error {
	for _, subscriber := range subscribers {
		chain := subscriber.Name()
		if _, ok := m.subs[chain]; ok {
			return fmt.Errorf("subscriber for chain %s already exists", chain)
		}

		if err := subscriber.Init(); err != nil {
			return fmt.Errorf("initializing %s subscriber: %w", chain, err)
		}
		m.subs[chain] = subscriber
	}
	return nil
}

func (m *mapSubManager) TrackWallet(wallet string, chain ChainName) error {
	if sub, ok := m.subs[chain]; ok {
		return sub.TrackWallet(wallet)
	}
	return fmt.Errorf("no registered subscriber for chain %s", chain)
}

func (m *mapSubManager) UntrackWallet(wallet string, chain ChainName) error {
	if sub, ok := m.subs[chain]; ok {
		return sub.UntrackWallet(wallet)
	}
	return fmt.Errorf("no registered subscriber for chain %s", chain)
}

func (m *mapSubManager) StartAll(sink chan<- *TrackedWalletEvent) error {
	errCh := make(chan error)
	for _, sub := range m.subs {
		events, errs := sub.Start()
		go func() {
			for {
				select {
				case event := <-events:
					sink <- event
				case err := <-errs:
					errCh <- err
				}
			}
		}()
	}
	return <-errCh
}
