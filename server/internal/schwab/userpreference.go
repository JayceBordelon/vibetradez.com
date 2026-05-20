package schwab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

/*
StreamerInfo is the slice of /trader/v1/userPreference relevant to the
WebSocket streamer: socket URL plus the four session identifiers Schwab
requires in every LOGIN / SUBS / UNSUBS frame. Field names match the
upstream JSON exactly (case-sensitive — the streamer rejects frames
with wrong casing).
*/
type StreamerInfo struct {
	StreamerSocketURL      string `json:"streamerSocketUrl"`
	SchwabClientCustomerId string `json:"schwabClientCustomerId"`
	SchwabClientCorrelId   string `json:"schwabClientCorrelId"`
	SchwabClientChannel    string `json:"schwabClientChannel"`
	SchwabClientFunctionId string `json:"schwabClientFunctionId"`
}

type userPreferenceResp struct {
	StreamerInfo []StreamerInfo `json:"streamerInfo"`
}

/*
GetStreamerInfo fetches /trader/v1/userPreference and returns the first
streamerInfo entry. Schwab's response wraps it in an array (one per
linked account) but the streamer session is account-agnostic — the
identifiers are tied to the authenticated user, so any entry works.
Returns an error if the response is empty or malformed.
*/
func (c *Client) GetStreamerInfo() (*StreamerInfo, error) {
	tok, err := c.ValidToken()
	if err != nil {
		return nil, fmt.Errorf("get streamer info: token: %w", err)
	}

	req, err := http.NewRequest("GET", baseURL+"/trader/v1/userPreference", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userPreference request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userPreference HTTP %d: %s", resp.StatusCode, string(body))
	}

	var pref userPreferenceResp
	if err := json.Unmarshal(body, &pref); err != nil {
		return nil, fmt.Errorf("decode userPreference: %w", err)
	}
	if len(pref.StreamerInfo) == 0 {
		return nil, fmt.Errorf("userPreference response has no streamerInfo entries")
	}
	si := pref.StreamerInfo[0]
	if si.StreamerSocketURL == "" || si.SchwabClientCustomerId == "" {
		return nil, fmt.Errorf("streamerInfo missing required fields (url or customerId)")
	}
	return &si, nil
}
