package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

const (
	beaconEndpointDefault = "http://127.0.0.1:5052"
	portDefault           = 3600
	secondsPerSlot        = 12
	slotsPerEpoch         = 32

	retentionPeriodDefault uint64 = 3
	versionMethod                 = "/eth/v1/node/version"
	specMethod                    = "/eth/v1/config/spec"
	genesisMethod                 = "/eth/v1/beacon/genesis"
	sidecarsMethod                = "/eth/v1/beacon/blob_sidecars/{id}"
)

var (
	slot0Timestamp   uint64
	retentionPeriod  uint64
	port             int
	beaconEndpoint   string
	emptySidecarList = &struct {
		Data []interface{} `json:"data"`
	}{Data: []interface{}{}}
)

func init() {
	flag.Uint64Var(&retentionPeriod, "r", retentionPeriodDefault, "blob retention period in epochs")
	flag.IntVar(&port, "p", portDefault, "listening port")
	flag.StringVar(&beaconEndpoint, "b", beaconEndpointDefault, "beacon endpoint")
	flag.Parse()
}

func main() {
	slot0Timestamp = queryGenesisTime()
	targetURL, _ := url.Parse(beaconEndpoint)
	r := mux.NewRouter()
	r.HandleFunc(versionMethod, createReverseProxy(targetURL))
	r.HandleFunc(specMethod, createReverseProxy(targetURL))
	r.HandleFunc(genesisMethod, createReverseProxy(targetURL))
	r.HandleFunc(sidecarsMethod, handleBlobSidecarsRequest)

	server := &http.Server{
		Handler: r,
	}
	endpoint := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	log.Printf("Beacon API wrapper started on %s\n", listener.Addr().String())
	log.Printf("Beacon endpoint: %s\n", beaconEndpoint)
	log.Printf("Retaining blobs for %d epochs (%d slots) \n", retentionPeriod, retentionPeriod*slotsPerEpoch)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server shutdown failed:%+v", err)
	}
	log.Println("Server exiting")
}

func handleBlobSidecarsRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request for %s\n", r.URL.Path)

	id := mux.Vars(r)["id"]
	if isHash(id) {
		http.Error(w, "Block hash is not supported yet", http.StatusInternalServerError)
		return
	}
	if isKnownIdentifier(id) {
		http.Error(w, fmt.Sprintf("%s is not supported yet", id), http.StatusInternalServerError)
		return
	}
	age, err := slotAge(id)
	if err != nil {
		http.Error(w, "Invalid block ID", http.StatusBadRequest)
		return
	}
	// if block is not in the retention window  return 200 w/ empty list
	// refer to https://github.com/prysmaticlabs/prysm/blob/feb16ae4aaa41d9bcd066b54b779dcd38fc928d2/beacon-chain/rpc/lookup/blocker.go#L226C20-L226C41
	if age > retentionPeriod*slotsPerEpoch {
		w.Header().Set("Content-Type", "application/json")
		log.Printf("Block %s is not in the retention window\n", id)
		json.NewEncoder(w).Encode(emptySidecarList)
		return
	}
	targetURL, _ := url.Parse(beaconEndpoint)
	httputil.NewSingleHostReverseProxy(targetURL).ServeHTTP(w, r)
}

func createReverseProxy(targetURL *url.URL) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request for %s\n", r.URL.Path)
		httputil.NewSingleHostReverseProxy(targetURL).ServeHTTP(w, r)
	}
}

func slotAge(id string) (uint64, error) {
	slot, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, err
	}

	curSlot := (uint64(time.Now().Unix()) - slot0Timestamp) / secondsPerSlot
	if curSlot < slot {
		return 0, errors.New("querying a future slot")
	}
	return curSlot - slot, nil
}

var knownIds = []string{"genesis", "finalized", "head"}

func isHash(s string) bool {
	if len(s) != 66 || !strings.HasPrefix(s, "0x") {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func isKnownIdentifier(id string) bool {
	for _, element := range knownIds {
		if element == id {
			return true
		}
	}
	return false
}

type GenesisResponse struct {
	Data struct {
		GenesisTime string `json:"genesis_time"`
	} `json:"data"`
}

func queryGenesisTime() uint64 {
	resp, err := http.Get(beaconEndpoint + genesisMethod)
	if err != nil {
		log.Fatalf("Error fetching data: %v", err)
	}
	defer resp.Body.Close()
	genesisResponse := new(GenesisResponse)
	err = json.NewDecoder(resp.Body).Decode(&genesisResponse)
	if err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}
	gt, err := strconv.ParseUint(genesisResponse.Data.GenesisTime, 10, 64)
	if err != nil {
		log.Fatalf("Error parsing genesis time: %v", err)
	}
	fmt.Println("genesis time", gt)
	return gt
}
