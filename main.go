package main

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

type Result string

const (
	ResultSecure           Result = "SECURE"
	ResultInsecureRedirect Result = "INSECURE_REDIRECT"
	ResultInsecure         Result = "INSECURE"
	ResultError            Result = "ERROR"
)

type Fetch struct {
	Hostname   string `json:"hostname"`
	Terminal   bool   `json:"terminal"`
	StatusCode int    `json:"status_code"`
	Next       string `json:"next,omitempty"`
	Insecure   bool   `json:"insecure"`
	Error      string `json:"error,omitempty"`
	Upgraded   bool   `json:"upgraded"`
}

type OutputRecord = struct {
	Hostname               string   `json:"hostname"`
	HTTP                   Result   `json:"http_result"`
	HTTPSteps              []*Fetch `json:"http_steps"`
	HTTPS                  Result   `json:"https_result"`
	HTTPSSteps             []*Fetch `json:"https_steps"`
	HTTPWithUpgrades       Result   `json:"http_upgrades"`
	HTTPWithUpgradesSteps  []*Fetch `json:"http_upgrades_steps"`
	HTTPForceUpgrades      Result   `json:"http_force_upgrades"`
	HTTPForceUpgradesSteps []*Fetch `json:"http_force_upgrades_steps"`
}

func resultFromChain(chain []*Fetch) Result {
	if len(chain) == 0 {
		return ResultError
	}
	hasInsecure := false
	for _, step := range chain {
		if step.Insecure {
			hasInsecure = true
		}
	}
	last := len(chain) - 1
	endsInsecure := chain[last].Insecure
	if !endsInsecure && !hasInsecure {
		return ResultSecure
	} else if !endsInsecure && hasInsecure {
		return ResultInsecureRedirect
	}
	return ResultInsecure
}

type Upgrade int

const (
	NoUpgrade       Upgrade = 0
	OptionalUpgrade Upgrade = 1
	ForceUpgrade    Upgrade = 2
)

func (up Upgrade) Upgraded(u url.URL) *url.URL {
	u.Scheme = "https"
	return &u
}

func shouldUpgrade(u *url.URL, up Upgrade) bool {
	if u.Scheme == "https" {
		return false
	}
	if u.Scheme == "http" {
		switch up {
		case NoUpgrade:
			return false
		case OptionalUpgrade, ForceUpgrade:
			return true
		}
	}
	return false
}

func canRetry(up Upgrade) bool {
	switch up {
	case OptionalUpgrade:
		return true
	}
	return false
}

func navigateStartingAt(hostname string, scheme string, upgrade Upgrade) ([]*Fetch, Result, error) {
	u := &url.URL{
		Scheme: scheme,
		Host:   hostname,
		Path:   "/",
	}
	var chain []*Fetch
	for {
		if len(chain) >= 25 {
			break
		}
		original := u
		attemptUpgrade := false
		didUpgrade := false
		if attemptUpgrade = shouldUpgrade(u, upgrade); attemptUpgrade {
			u = upgrade.Upgraded(*u)
			didUpgrade = true
		}
		fetch, next, err := sendOne(u)
		if err != nil && attemptUpgrade && canRetry(upgrade) {
			didUpgrade = false
			fetch, next, err = sendOne(original)
		}
		if err != nil {
			return chain, ResultError, err
		}
		fetch.Upgraded = didUpgrade
		chain = append(chain, fetch)
		if fetch.Terminal || next == nil {
			break
		}
		u = next
	}
	return chain, resultFromChain(chain), nil
}

func sendOne(u *url.URL) (f *Fetch, next *url.URL, err error) {
	httpClient := http.Client{
		Timeout: time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req := http.Request{
		Method: "GET",
		URL:    u,
		Header: http.Header{
			"Host":             {u.Host},
			"User-Agent":       {userAgent},
			"Accept-Languague": {"en-US,en;q=0.9"},
		},
	}
	f = &Fetch{
		Hostname: u.Host,
	}

	var res *http.Response
	res, err = httpClient.Do(&req)
	if err != nil {
		f.Error = err.Error()
		return
	}
	defer res.Body.Close()
	f.StatusCode = res.StatusCode
	f.Insecure = res.TLS == nil
	next, err = res.Location()
	if err != nil {
		if err == http.ErrNoLocation {
			f.Terminal = true
			err = nil
			return
		}
		f.Error = err.Error()
		return
	}
	f.Next = next.String()
	return
}

func main() {
	var inputFile = os.Stdin
	var outputFile = os.Stdout

	r := csv.NewReader(inputFile)
	w := json.NewEncoder(outputFile)
	rowNumber := 0
	for {
		rowNumber += 1
		record, err := r.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("error reading input csv in row %d: %s", rowNumber, err)
		}
		if len(record) < 1 {
			log.Fatalf("empty record in row %d", rowNumber)
		}
		hostname := record[0]

		output := OutputRecord{
			Hostname: hostname,
		}

		output.HTTPSteps, output.HTTP, _ = navigateStartingAt(hostname, "http", NoUpgrade)
		output.HTTPSSteps, output.HTTPS, _ = navigateStartingAt(hostname, "https", NoUpgrade)
		output.HTTPWithUpgradesSteps, output.HTTPWithUpgrades, _ = navigateStartingAt(hostname, "http", OptionalUpgrade)
		output.HTTPForceUpgradesSteps, output.HTTPForceUpgrades, _ = navigateStartingAt(hostname, "http", ForceUpgrade)

		if err := w.Encode(&output); err != nil {
			log.Fatalf("error writing output for row %d: %s", rowNumber, err)
		}
	}
}
