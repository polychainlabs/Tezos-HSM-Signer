package signer

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Server holds all configuration data from the signer
type Server struct {
	signer     Signer
	keyManager KeyManager
	bindString string
	enableTx   bool
}

// CreateServer returns a newly configured server
func CreateServer(keyfile string, hsmPin string, hsmSo string, serverBindString string, enableTx bool, debug bool, lockdir string) *Server {
	debugEnabled = debug

	if enableTx {
		log.Println("WARNING: Transaction signing is enabled.  Use with caution.")
	}

	return &Server{
		keyManager: loadKeyManager(keyfile),
		signer: &Hsm{
			UserPin: hsmPin,
			LibPath: hsmSo,
		},
		enableTx:   enableTx,
		bindString: serverBindString,
	}
}

// Middleware sets content type and log path for all requests
func Middleware(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.URL.Path, r.Method)
		// Default Content-Type is JSON.
		w.Header().Set("Content-Type", "application/json")
		f(w, r)
	}
}

// RouteUnmatched handles all requests that aren't matched by the below routes
func RouteUnmatched(w http.ResponseWriter, r *http.Request) {
	// Route: <anything not matched>
	// Response Body: `{"error":"not found"}`
	// Status: 404
	// mimetype: "application/json"
	log.Println(r.URL.Path[1:], "not found")

	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "{\"error\":\"not found\"}")

}

// RouteAuthorizedKeys list all of they keys that we currently support.  We choose to
// return an empty set to obscure our secrets.
func (server *Server) RouteAuthorizedKeys(w http.ResponseWriter, r *http.Request) {
	// Route: /authorized_keys
	// Response Body: `{}`
	// Status: 200
	// mimetype: "application/json"
	fmt.Fprintf(w, "{}")
}

// RouteKeys validates a /key/ request and routes based on HTTP Method
func (server *Server) RouteKeys(w http.ResponseWriter, r *http.Request) {
	requestedKeyHash := strings.Split(r.URL.Path, "/")[2]
	key := server.keyManager.GetKeyFromHash(requestedKeyHash)

	if key == nil {
		log.Println("Key not found:", requestedKeyHash)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Key not found")
		return
	}

	switch r.Method {
	case "GET":
		server.RouteKeysGET(w, r, key)
	case "POST":
		fmt.Println("hihi")
		server.RouteKeysPOST(w, r, key)
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "{\"error\":\"bad_verb\"}")
	}
}

// RouteKeysGET returns the corresponding public key to this public key *hash*
func (server *Server) RouteKeysGET(w http.ResponseWriter, r *http.Request, key *Key) {
	// Route: /keys/<key>
	// Response Body: `{"public_key": "<key>"}`
	// Status: 200
	// mimetype: "application/json"
	fmt.Fprintf(w, "{\"public_key\":\"%s\"}", key.PublicKey)
}

// RouteKeysPOST attempts to sign the provided message from the provided keys
func (server *Server) RouteKeysPOST(w http.ResponseWriter, r *http.Request, key *Key) {
	// Route: /keys/<key>
	// Method: POST
	// Response Body: `{"signature": "p2sig....."}`
	// Status: 200
	// mimetype: "application/json"

	// Only ever sign for one key at a time
	key.Lock()
	defer key.Unlock()

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading POST content: ", err)

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "{\"error\":\"%s\"}", "error reading the request")
		return
	}
	debugln("Received sign request: ", string(body))

	// Parse and sign the message
	request, err := ParseRequest(body)
	if err != nil {
		log.Println("Error parsing signing request: ", err)

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "{\"error\":\"%s\"}", "error signing the request")
		return
	}

	// Validate the operation
	if request.OpType() == opTypeTx && !server.enableTx {
		// Disallow transactions unless specifically enabled
		log.Println("Error, transaction signing disabled")

		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "{\"error\":\"%s\"}", "transactions cannot be signed")
		return
	}
	if !key.IsSafeToSign(request.OpType(), request.Level()) {
		// Never endorse or bake at the same level twice
		log.Println("Error, this level has already been signed")

		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "{\"error\":\"%s\"}", "level cannot be signed")
		return
	}

	// Sign the operation
	server.keyManager.SetLastSignedLevel(key, request.OpType(), request.Level())
	signed, err := request.TzSign(server.signer, key)
	if err != nil {
		log.Println("Error signing request:", err)

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "{\"error\":\"%s\"}", "error signing the request")
	} else {
		response := fmt.Sprintf("{\"signature\":\"%s\"}", signed)
		log.Println("Returning signed message: ", response)

		fmt.Fprintf(w, response)
	}
}

// shutdown gracefully
func shutdown(c chan os.Signal) {
	<-c
	log.Println("Shutting down")
	os.Exit(0)
}

// Serve our routes
func (server *Server) Serve() {
	// Handle Sigterm
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go shutdown(c)

	// Routes
	http.HandleFunc("/", Middleware(RouteUnmatched))
	http.HandleFunc("/authorized_keys", Middleware(server.RouteAuthorizedKeys))
	http.HandleFunc("/keys/", Middleware(server.RouteKeys))

	// Serve
	log.Println("Listening on:", server.bindString)
	log.Fatal(http.ListenAndServe(server.bindString, nil))
}
