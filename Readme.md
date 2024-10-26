# Deblock backend home assignment 
Copy `.env.example` as `.env` and fill in the required RPC urls. 

Then run 
```bash
go run cmd/main.go
```

Or run with docker
```bash
docker build -t deblock-backend .


# Possible improvements:
    - Use multiple RPC urls from different providers
    - Include metrics
    - Backoff strategy for failed requests or internal restarts for subscriber components
