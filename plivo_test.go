// Copyright 2026 The Hanzo IAM Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package go_sms_sender

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetPlivoClient_EmptyCreds(t *testing.T) {
	cases := []struct {
		name, id, key string
	}{
		{"empty id", "", "token"},
		{"empty token", "MA123", ""},
		{"both empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := GetPlivoClient(tc.id, tc.key, "Your code is %s")
			if err == nil {
				t.Fatalf("expected error, got client=%v", c)
			}
		})
	}
}

func TestGetPlivoClient_OK(t *testing.T) {
	c, err := GetPlivoClient("MA123", "tok", "Your code is %s")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if c.authID != "MA123" || c.authToken != "tok" {
		t.Fatalf("creds not stored: %+v", c)
	}
	if !strings.Contains(c.endpoint, "MA123") {
		t.Fatalf("endpoint missing auth id: %s", c.endpoint)
	}
}

func TestPlivoSendMessage_MissingCode(t *testing.T) {
	c, _ := GetPlivoClient("MA123", "tok", "Your code is %s")
	if err := c.SendMessage(map[string]string{}, "+15551234567", "+15557654321"); err == nil {
		t.Fatal("expected missing-code error")
	}
}

func TestPlivoSendMessage_NoRecipient(t *testing.T) {
	c, _ := GetPlivoClient("MA123", "tok", "Your code is %s")
	if err := c.SendMessage(map[string]string{"code": "1234"}, "+15551234567"); err == nil {
		t.Fatal("expected bad-parameter error")
	}
}

func TestPlivoSendMessage_TemplateSubAndAuth(t *testing.T) {
	var gotBody map[string]string
	var gotAuthUser, gotAuthPass string
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthUser, gotAuthPass, _ = r.BasicAuth()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"api_id":"abc","message_uuid":["uuid1"],"message":"message(s) queued"}`))
	}))
	defer srv.Close()

	c, err := GetPlivoClient("MA123", "tok", "Your code is %s")
	if err != nil {
		t.Fatalf("client err: %v", err)
	}
	// Redirect endpoint to the test server while preserving the auth_id path segment.
	c.endpoint = srv.URL + "/v1/Account/MA123/Message/"

	err = c.SendMessage(map[string]string{"code": "424242"}, "+12062598397", "+19137779708")
	if err != nil {
		t.Fatalf("send err: %v", err)
	}
	if gotAuthUser != "MA123" || gotAuthPass != "tok" {
		t.Fatalf("basic auth wrong: %q:%q", gotAuthUser, gotAuthPass)
	}
	if gotBody["src"] != "+12062598397" || gotBody["dst"] != "+19137779708" {
		t.Fatalf("bad payload: %+v", gotBody)
	}
	if gotBody["text"] != "Your code is 424242" {
		t.Fatalf("template not substituted: %q", gotBody["text"])
	}
	if !strings.Contains(gotPath, "/Account/MA123/Message/") {
		t.Fatalf("endpoint path wrong: %s", gotPath)
	}
}

func TestPlivoSendMessage_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"invalid auth"}`)
	}))
	defer srv.Close()

	c, _ := GetPlivoClient("MA123", "tok", "Your code is %s")
	c.endpoint = srv.URL + "/v1/Account/MA123/Message/"

	err := c.SendMessage(map[string]string{"code": "1"}, "+12062598397", "+19137779708")
	if err == nil {
		t.Fatal("expected error from non-2xx")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid auth") {
		t.Fatalf("error missing status/body: %v", err)
	}
}

func TestPlivoSendMessage_MultipleRecipients(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c, _ := GetPlivoClient("MA123", "tok", "Your code is %s")
	c.endpoint = srv.URL + "/v1/Account/MA123/Message/"

	if err := c.SendMessage(map[string]string{"code": "1"}, "+12062598397", "+19137779708", "+19135551234"); err != nil {
		t.Fatalf("send err: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", calls)
	}
}
