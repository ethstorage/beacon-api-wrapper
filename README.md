# beacon-api-wrapper
A beacon API wrapper for archiver tests

# Start
```
 go run cmd/main.go 
```

# Options

```
  -b string
        beacon endpoint (default "http://88.99.30.186:3500")
  -p int
        listening port (default 3600)
  -r uint
        blob retention period in seconds (default 10800)
```

# How it works

The following requests will be redirected to the beacon API:
```
/eth/v1/config/spec
/eth/v1/node/version
/eth/v1/beacon/genesis
```
The `blob_sidecars` request will be redirected to the beacon API if the blobs are within the retention period, otherwise return an empty list: `{"data":[]}`
```
/eth/v1/beacon/blob_sidecars/{block_id}
```