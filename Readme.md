# Deblock backend home assignment 
Copy `.env.example` as `.env` and fill in the required RPC urls. 

To utilize kafka, run it locally via docker compose:

```bash
docker compose up -d
```
Or supply `KAFKA_BROKER_URL` in `.env` to point to a remote kafka broker. By
default, events are pushed to `deblock_tx_tracker` topic.

To run the service:
```bash
go run cmd/main.go
```

Or run with docker and provide the required envs or env file:
```bash
docker build -t deblock-backend .
docker run -it --env-file .env deblock-backend
```

For REST API endpoints for registering wallet tracking, see
`internal/api/http_server.go`

# Possible improvements:
    - Use multiple RPC urls from different providers
    - Instrument and expose prometheus metrics
    - Backoff strategy for failed requests or internal restarts for subscriber components
    - Better subscriber error handling 
        - Missing blocks
        - RPC errors/retries
    - Limit to ~10-20k wallets per instance, run multiple instances with deterministic routing, or implement redis like persistence cache for registered tracked wallets.


# Bonus questions
    - How to handle 1H downtime?
        - Store last processed block numbers in a persistent storage: redis, db, etc. On startup, start querying txs from last processed block.
    - How to handle chain reorgs?
        - Maintain a cache of last processed block nums. If we get smaller one than last processed, chain reorg occured. Reprocess the txs in new blocks. Additional option is to utilize usage of "finalized" block querying if chain supports it. Maintain a 1-2 block lag before processing txs.
    - How to handle retries?
        - Implement a backoff and retry strategy for failed requests or hanging wss connections. For internal errors, restart the subscriber components.

