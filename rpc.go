package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
)

// Server mode: every App method that the desktop UI calls through Wails can
// also be called over HTTP, so a client on another computer can talk to one
// shared backend.
//
//	POST /rpc/<MethodName>   body: JSON array of arguments
//	                         reply: {"result": ..., "error": "..."}
//
// Security does not change: every method still checks the session token it
// receives as its first argument.

// maxRPCBody fits a 25 MB file sent as base64 plus some JSON overhead.
const maxRPCBody = 40 << 20

// rpcAllowed lists the methods reachable over the network.
var rpcAllowed = map[string]bool{
	"Register": true, "Login": true, "Logout": true, "CheckAuth": true,
	"GetUsers": true, "UpdateUserStatus": true,
	"GetChannels": true, "CreateChannel": true, "AddChannelMember": true,
	"DeleteChannel": true, "LeaveChannel": true, "JoinChannel": true,
	"GetDMChannels": true, "GetOrCreateDMChannel": true, "GetOrCreateGroupChannel": true, "DeleteDMChannel": true,
	"GetMessages": true, "GetMessagesPage": true, "SendMessage": true, "SendPost": true,
	"EditMessage": true, "DeleteMessage": true, "AddReaction": true, "SearchMessages": true,
	"MarkChannelRead": true, "GetUnreadCounts": true,
	"SendFile": true, "GetFileData": true, "LogCall": true, "GetLinkPreview": true,
	"CreateEvent": true, "GetEvents": true, "RespondToEvent": true, "UpdateEvent": true, "DeleteEvent": true, "GetDueReminders": true,
	"SendReply": true, "GetThread": true, "EditReply": true, "DeleteReply": true, "AddReplyReaction": true,
}

// rpcDesktopOnly lists methods that open dialogs or browsers on the machine
// that runs the backend. They make no sense on a server.
var rpcDesktopOnly = map[string]bool{
	"SaveFileToDisk": true, "ExportChannel": true,
	"LoginWithGoogle": true, "CancelGoogleLogin": true,
}

type rpcReply struct {
	Result interface{} `json:"result"`
	Error  string      `json:"error,omitempty"`
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func writeRPC(w http.ResponseWriter, status int, rep rpcReply) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rep)
}

func setCORS(w http.ResponseWriter) {
	// The token travels in the request body, not in a cookie, so any origin
	// may call; without a valid token every method refuses.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Invite-Code")
	w.Header().Set("Access-Control-Max-Age", "600")
}

// ServeRPC returns the HTTP handler for /rpc/<Method>.
func ServeRPC(app *App) http.HandlerFunc {
	appVal := reflect.ValueOf(app)
	return func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			writeRPC(w, http.StatusMethodNotAllowed, rpcReply{Error: "POST only"})
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/rpc/")
		if !rpcAllowed[name] {
			writeRPC(w, http.StatusNotFound, rpcReply{Error: "unknown method"})
			return
		}
		method := appVal.MethodByName(name)
		if !method.IsValid() {
			writeRPC(w, http.StatusNotFound, rpcReply{Error: "unknown method"})
			return
		}

		if name == "Register" {
			if msg, status := checkInvite(r); msg != "" {
				writeRPC(w, status, rpcReply{Error: msg})
				return
			}
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRPCBody)
		var raw []json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			writeRPC(w, http.StatusBadRequest, rpcReply{Error: "body must be a JSON array of arguments"})
			return
		}

		out, err := callRPC(method, raw)
		if err != nil {
			var bad *rpcBadRequest
			if errors.As(err, &bad) {
				writeRPC(w, http.StatusBadRequest, rpcReply{Error: bad.Error()})
				return
			}
			writeRPC(w, http.StatusOK, rpcReply{Error: err.Error()})
			return
		}
		writeRPC(w, http.StatusOK, rpcReply{Result: out})
	}
}

type rpcBadRequest struct{ msg string }

func (e *rpcBadRequest) Error() string { return e.msg }

// callRPC decodes the arguments to the method's parameter types, calls it and
// turns its results into (value, error).
func callRPC(method reflect.Value, raw []json.RawMessage) (out interface{}, err error) {
	mt := method.Type()
	if len(raw) != mt.NumIn() {
		return nil, &rpcBadRequest{fmt.Sprintf("expected %d arguments, got %d", mt.NumIn(), len(raw))}
	}
	args := make([]reflect.Value, mt.NumIn())
	for i := range args {
		p := reflect.New(mt.In(i))
		if err := json.Unmarshal(raw[i], p.Interface()); err != nil {
			return nil, &rpcBadRequest{fmt.Sprintf("argument %d has the wrong type", i+1)}
		}
		args[i] = p.Elem()
	}

	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("❌ rpc panic: %v", rec)
			out, err = nil, errors.New("internal error")
		}
	}()

	res := method.Call(args)
	if n := len(res); n > 0 && mt.Out(n-1) == errorType {
		if e, _ := res[n-1].Interface().(error); e != nil {
			return nil, e
		}
		res = res[:n-1]
	}
	switch len(res) {
	case 0:
		return nil, nil
	case 1:
		return res[0].Interface(), nil
	default:
		vals := make([]interface{}, len(res))
		for i, v := range res {
			vals[i] = v.Interface()
		}
		return vals, nil
	}
}

// newServerMux wires WebSocket, RPC and the health check together.
func newServerMux(app *App, hub *Hub) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ServeWS(hub))
	mux.HandleFunc("/rpc/", ServeRPC(app))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"app":"gothermo"}`))
	})
	return mux
}

// StartServer runs the full backend (WebSocket + RPC) for server mode.
func StartServer(app *App, hub *Hub, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           newServerMux(app, hub),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if inviteRequired() == "" {
		log.Printf("⚠️  Registration is open: anyone who can reach this server can create an account. Set GOTHERMO_INVITE_CODE to close it.")
	}
	log.Printf("🚀 GoThermo server listening on %s", addr)
	return srv.ListenAndServe()
}

// envOr returns the environment variable's value, or def when it is unset.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
