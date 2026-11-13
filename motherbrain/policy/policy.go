// Package policy is a controller that asks a server what to do: the observation goes out as JSON over HTTP, a twist comes back. Whatever sits behind that server, a learned policy or a script in any language, is its business; a server that describes itself at its root is offered under that name.
package policy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	tourianv1 "github.com/vlumi/tourian/gen/go/tourian/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type Client struct {
	url         string
	description *tourianv1.Controller
	http        *http.Client
}

// New makes the client, asking the server at serverURL to describe itself; one that does not answer is offered as a policy named by its host.
func New(serverURL string) *Client {
	c := &Client{url: serverURL, http: &http.Client{Timeout: time.Second}}
	c.description = c.describe()
	return c
}

func (c *Client) describe() *tourianv1.Controller {
	host := c.url
	if u, err := url.Parse(c.url); err == nil && u.Host != "" {
		host = u.Host
	}
	fallback := &tourianv1.Controller{Id: "policy@" + host, Kind: tourianv1.ControllerKind_CONTROLLER_KIND_POLICY, Name: "Policy at " + host}
	response, err := c.http.Get(c.url + "/")
	if err != nil {
		return fallback
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<16))
	if err != nil || response.StatusCode != http.StatusOK {
		return fallback
	}
	description := &tourianv1.Controller{}
	if protojson.Unmarshal(data, description) != nil || description.GetId() == "" {
		return fallback
	}
	if description.GetKind() == tourianv1.ControllerKind_CONTROLLER_KIND_UNSPECIFIED {
		description.Kind = tourianv1.ControllerKind_CONTROLLER_KIND_POLICY
	}
	return description
}

func (c *Client) Describe() *tourianv1.Controller {
	return c.description
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
