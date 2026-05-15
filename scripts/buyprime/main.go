//go:build buyprime

// One-shot operator utility: pays a running probably-prime-ai server via x402
// for one prime and prints the result.
//
// Reads LIVE_E2E_PRIVATE_KEY from the environment (use `set -a; source .env.live;
// set +a` to load it). Defaults to bits=256, defaults to http://localhost:4021.
//
//	go run -tags buyprime ./scripts/buyprime/ [bits] [url]
//
// Examples:
//
//	go run -tags buyprime ./scripts/buyprime/                              # 256-bit @ localhost:4021
//	go run -tags buyprime ./scripts/buyprime/ 1024                         # 1024-bit @ localhost:4021
//	go run -tags buyprime ./scripts/buyprime/ 512 http://127.0.0.1:4021    # explicit URL
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	x402core "github.com/x402-foundation/x402/go"
	x402http "github.com/x402-foundation/x402/go/http"
	evmexact "github.com/x402-foundation/x402/go/mechanisms/evm/exact/client"
	evmsigners "github.com/x402-foundation/x402/go/signers/evm"
)

const defaultBaseURL = "http://localhost:4021"

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "buyprime: "+msg+"\n", args...)
	os.Exit(1)
}

func main() {
	privateKey := os.Getenv("LIVE_E2E_PRIVATE_KEY")
	if privateKey == "" {
		die("LIVE_E2E_PRIVATE_KEY not set — run `set -a; source .env.live; set +a` first")
	}

	bits := 256
	baseURL := defaultBaseURL
	if len(os.Args) > 1 {
		b, err := strconv.Atoi(os.Args[1])
		if err != nil {
			die("invalid bits arg %q: %v", os.Args[1], err)
		}
		bits = b
	}
	if len(os.Args) > 2 {
		baseURL = os.Args[2]
	}

	signer, err := evmsigners.NewClientSignerFromPrivateKey(privateKey)
	if err != nil {
		die("signer: %v", err)
	}
	x402Client := x402core.Newx402Client().Register(
		"eip155:*",
		evmexact.NewExactEvmScheme(signer, nil),
	)
	httpClient := x402http.WrapHTTPClientWithPayment(
		&http.Client{},
		x402http.Newx402HTTPClient(x402Client),
	)

	url := fmt.Sprintf("%s/prime/%d", strings.TrimRight(baseURL, "/"), bits)
	fmt.Printf("→ GET %s\n", url)

	resp, err := httpClient.Get(url)
	if err != nil {
		die("request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		die("read body: %v", err)
	}

	fmt.Printf("← HTTP %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		die("non-200 response: %s", string(body))
	}

	// Decode the prime body.
	var pr struct {
		Bits         int    `json:"bits"`
		Prime        string `json:"prime"`
		PrimeDecimal string `json:"prime_decimal"`
		Rounds       int    `json:"rounds"`
		GeneratedAt  string `json:"generated_at"`
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		die("decode body: %v\nraw: %s", err, string(body))
	}

	// Decode the settlement header for the tx hash.
	settleHeader := resp.Header.Get("PAYMENT-RESPONSE")
	if settleHeader == "" {
		settleHeader = resp.Header.Get("X-PAYMENT-RESPONSE")
	}
	txHash := ""
	if settleHeader != "" {
		raw, derr := base64.StdEncoding.DecodeString(settleHeader)
		if derr == nil {
			var s x402core.SettleResponse
			if err := json.Unmarshal(raw, &s); err == nil {
				txHash = s.Transaction
			}
		}
	}

	fmt.Println()
	fmt.Println("=== Prime ===")
	fmt.Printf("  bits:         %d\n", pr.Bits)
	fmt.Printf("  rounds:       %d\n", pr.Rounds)
	fmt.Printf("  generated_at: %s\n", pr.GeneratedAt)
	fmt.Printf("  hex:          %s\n", pr.Prime)
	fmt.Printf("  decimal:      %s\n", pr.PrimeDecimal)
	if txHash != "" {
		fmt.Printf("  settlement:   %s (https://sepolia.basescan.org/tx/%s)\n", txHash, txHash)
	}
}
