package portfolio

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

// The web_search result the API returns attaches a large opaque
// encrypted_content blob to every source for citation replay. The
// transcript only needs the query (recorded on the tool_use) plus the
// sources, so slimWebSearchResult must drop the blob and keep title/url/age.
func TestSlimWebSearchResult_DropsEncryptedContent(t *testing.T) {
	content := anthropic.WebSearchToolResultBlockContentUnion{
		OfWebSearchResultBlockArray: []anthropic.WebSearchResultBlock{
			{
				Title:            "Brent, WTI oil prices: Trump calls off Iran strikes",
				URL:              "https://www.cnbc.com/2026/06/11/brent-wti-oil-prices.html",
				PageAge:          "1 day ago",
				EncryptedContent: strings.Repeat("ZmFrZS1lbmNyeXB0ZWQtYmxvYg==", 500),
			},
			{
				Title:            "Crude Oil - Price - Chart - Historical Data",
				URL:              "https://tradingeconomics.com/commodity/crude-oil",
				PageAge:          "3 hours ago",
				EncryptedContent: strings.Repeat("c2Vjb25kLW9wYXF1ZS1ibG9i", 500),
			},
		},
	}

	out := slimWebSearchResult(content)

	if strings.Contains(out, "encrypted") {
		t.Fatalf("slimmed result must not contain encrypted content, got: %s", out)
	}
	if len(out) > 1000 {
		t.Fatalf("slimmed result should be small once the blobs are dropped, got %d bytes", len(out))
	}

	var sources []map[string]string
	if err := json.Unmarshal([]byte(out), &sources); err != nil {
		t.Fatalf("slimmed result should be a JSON array of sources, got %q: %v", out, err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	first := sources[0]
	if first["title"] == "" || first["url"] == "" || first["page_age"] == "" {
		t.Fatalf("each source must keep title/url/page_age, got %+v", first)
	}
	if _, ok := first["encrypted_content"]; ok {
		t.Fatalf("source must not carry encrypted_content, got %+v", first)
	}
}

func TestSlimWebSearchResult_ErrorAndEmpty(t *testing.T) {
	// An errored search keeps a machine-readable error code.
	errOut := slimWebSearchResult(anthropic.WebSearchToolResultBlockContentUnion{
		ErrorCode: "max_uses_exceeded",
	})
	if !strings.Contains(errOut, "max_uses_exceeded") {
		t.Fatalf("error result should carry the error code, got: %s", errOut)
	}

	// An empty result records nothing, which the transcript view renders as
	// "the search returned nothing".
	if got := slimWebSearchResult(anthropic.WebSearchToolResultBlockContentUnion{}); got != "" {
		t.Fatalf("empty result should record nothing, got: %q", got)
	}
}
