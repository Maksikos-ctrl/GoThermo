package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func rpcCall(t *testing.T, h http.Handler, method, body string) (int, rpcReply) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rpc/"+method, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var rep rpcReply
	_ = json.Unmarshal(rec.Body.Bytes(), &rep)
	return rec.Code, rep
}

// Every exported App method must be consciously classified, so a new method
// is never exposed (or forgotten) by accident.
func TestEveryAppMethodIsClassified(t *testing.T) {
	at := reflect.TypeOf(&App{})
	for i := 0; i < at.NumMethod(); i++ {
		n := at.Method(i).Name
		if rpcAllowed[n] == rpcDesktopOnly[n] {
			t.Errorf("%s must be in exactly one of rpcAllowed / rpcDesktopOnly", n)
		}
	}
	for n := range rpcAllowed {
		if _, ok := at.MethodByName(n); !ok {
			t.Errorf("rpcAllowed lists %s, which does not exist", n)
		}
	}
}

func TestRPCRoundTrip(t *testing.T) {
	mr := useMiniRedis(t)
	tok := mkUser(t, mr, "alice")
	h := ServeRPC(&App{hub: NewHub()})
	go globalHub.Run()

	if err := SaveChannel(Channel{ID: "c1", Name: "general"}); err != nil {
		t.Fatal(err)
	}

	code, rep := rpcCall(t, h, "SendMessage", `["`+tok+`","alice","hello","general"]`)
	if code != 200 || rep.Error != "" {
		t.Fatalf("SendMessage: %d %+v", code, rep)
	}
	code, rep = rpcCall(t, h, "GetMessages", `["`+tok+`","general"]`)
	if code != 200 || rep.Error != "" {
		t.Fatalf("GetMessages: %d %+v", code, rep)
	}
	list, ok := rep.Result.([]interface{})
	if !ok || len(list) != 1 {
		t.Fatalf("want 1 message, got %#v", rep.Result)
	}
	if list[0].(map[string]interface{})["text"] != "hello" {
		t.Fatalf("unexpected message %#v", list[0])
	}
}

func TestRPCErrorsAreReturned(t *testing.T) {
	mr := useMiniRedis(t)
	_ = mkUser(t, mr, "alice")
	h := ServeRPC(&App{hub: NewHub()})

	// bad token: the method's own check refuses
	code, rep := rpcCall(t, h, "GetChannels", `["nope"]`)
	if code != 200 || rep.Error == "" {
		t.Fatalf("expected an error reply, got %d %+v", code, rep)
	}
}

func TestRPCRejectsBadRequests(t *testing.T) {
	useMiniRedis(t)
	h := ServeRPC(&App{hub: NewHub()})

	cases := []struct {
		name, method, body string
		want               int
	}{
		{"unknown method", "Nope", `[]`, 404},
		{"desktop only", "SaveFileToDisk", `["t","f"]`, 404},
		{"google login", "LoginWithGoogle", `[false]`, 404},
		{"lowercase internal", "startup", `[]`, 404},
		{"not an array", "GetChannels", `{"a":1}`, 400},
		{"wrong count", "GetChannels", `["a","b"]`, 400},
		{"wrong type", "GetChannels", `[5]`, 400},
	}
	for _, c := range cases {
		if code, _ := rpcCall(t, h, c.method, c.body); code != c.want {
			t.Errorf("%s: got %d, want %d", c.name, code, c.want)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/rpc/GetChannels", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 405 {
		t.Errorf("GET: got %d, want 405", rec.Code)
	}
}

func TestRPCCORSPreflight(t *testing.T) {
	h := ServeRPC(&App{})
	req := httptest.NewRequest(http.MethodOptions, "/rpc/Login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}
}

func TestRPCMultiReturnAndBool(t *testing.T) {
	mr := useMiniRedis(t)
	tok := mkUser(t, mr, "alice")
	h := ServeRPC(&App{hub: NewHub()})
	code, rep := rpcCall(t, h, "Logout", `["`+tok+`"]`)
	if code != 200 || rep.Error != "" || rep.Result != true {
		t.Fatalf("Logout: %d %+v", code, rep)
	}
}

func TestRPCRecoversFromPanic(t *testing.T) {
	boom := reflect.ValueOf(func() error { panic("boom") })
	_, err := callRPC(boom, nil)
	if err == nil || err.Error() != "internal error" {
		t.Fatalf("want internal error, got %v", err)
	}
}
