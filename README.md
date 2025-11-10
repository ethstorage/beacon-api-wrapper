# beacon-api-wrapper
A Beacon API wrapper for archiver tests that mocks a short blob retention window.

# Start
```
 go run cmd/main.go 
```

# Options

```
  -b string
        beacon endpoint (default "http://65.108.230.142:3500")
  -p int
        listening port (default 3600)
  -r uint
        blob retention period in epochs (default 3)
```

# How it works

The following requests will be redirected to the beacon API:
```
/eth/v1/config/spec
/eth/v1/node/version
/eth/v1/beacon/genesis
```
The `blobs` request will be redirected to the beacon API if the blobs are within the retention period, otherwise return an empty list: `{"data":[]}`
```
/eth/v1/beacon/blobs/{block_id}
```