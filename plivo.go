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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// plivoEndpoint is the Plivo Messages API base. It accepts the auth_id as a
// path segment and authenticates via HTTP Basic auth using auth_id:auth_token.
const plivoEndpoint = "https://api.plivo.com/v1/Account/%s/Message/"

type PlivoClient struct {
	template  string
	authID    string
	authToken string
	endpoint  string
	http      *http.Client
}

func GetPlivoClient(accessId string, accessKey string, template string) (*PlivoClient, error) {
	if accessId == "" || accessKey == "" {
		return nil, fmt.Errorf("plivo: empty auth_id or auth_token")
	}
	return &PlivoClient{
		template:  template,
		authID:    accessId,
		authToken: accessKey,
		endpoint:  fmt.Sprintf(plivoEndpoint, accessId),
		http:      &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// SendMessage targetPhoneNumber[0] is the sender's number, so targetPhoneNumber should have at least two parameters
func (c *PlivoClient) SendMessage(param map[string]string, targetPhoneNumber ...string) error {
	code, ok := param["code"]
	if !ok {
		return fmt.Errorf("missing parameter: code")
	}

	bodyContent := c.template
	if c.template != "" {
		bodyContent = fmt.Sprintf(c.template, code)
	}

	if len(targetPhoneNumber) < 2 {
		return fmt.Errorf("bad parameter: targetPhoneNumber")
	}

	from := targetPhoneNumber[0]
	for i := 1; i < len(targetPhoneNumber); i++ {
		if err := c.sendOne(from, targetPhoneNumber[i], bodyContent); err != nil {
			return err
		}
	}
	return nil
}

func (c *PlivoClient) sendOne(from, to, text string) error {
	payload, err := json.Marshal(map[string]string{"src": from, "dst": to, "text": text})
	if err != nil {
		return fmt.Errorf("plivo marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("plivo request: %w", err)
	}
	req.SetBasicAuth(c.authID, c.authToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("plivo send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("plivo send: status=%d body=%s", resp.StatusCode, string(b))
	}
	return nil
}
