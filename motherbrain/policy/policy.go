// Package policy is a controller that asks a policy server what to do: the observation goes out as JSON over HTTP, a twist comes back. Whatever learned policy sits behind that server is its business.
package policy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type Client struct {
	url  string
	name string
	http *http.Client
}

func New(serverURL string) *Client {
	name := serverURL
	if u, err := url.Parse(serverURL); err == nil && u.Host != "" {
		name = u.Host
	}
	return &Client{url: serverURL, name: name, http: &http.Client{}}
}

func (c *Client) Describe() *tourianv1.Controller {
	return &tourianv1.Controller{Id: "policy", Kind: tourianv1.ControllerKind_CONTROLLER_KIND_POLICY, Name: "Policy at " + c.name}
}

func (c *Client) Act(ctx context.Context, observation *tourianv1.Observation) (*tourianv1.Twist, error) {
	body, err := protojson.Marshal(observation)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/act", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("policy: %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<16))
	if err != nil {
		return nil, err
	}
	twist := &tourianv1.Twist{}
	if err := protojson.Unmarshal(data, twist); err != nil {
		return nil, fmt.Errorf("policy: decoding the action: %w", err)
	}
	return twist, nil
}
