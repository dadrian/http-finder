# HTTP Finder

Check domains for redirects through HTTP that could result in warnings
when "Always Use Secure Connections" in Chrome (or a similar setting in another
browser) is enabled.

## Usage

Accepts a list of domains on stdin, and outputs JSON lines to stdout.

```console
$ echo 'example.com' | go run .
```

## Output Schema

A given `$HOSTNAME` from stdin is connected to using four different methodologies to resolve
the initial connection and any subsequent redirects:

- **HTTP**, stored in `http_result`, will attempt `http://$HOSTNAME` and follow
  any redirects directly
- **HTTPS**, stored in `https_result`, will attempt `https://$HOSTNAME` and
  follow any redirects directly
- **HTTP Upgrades**, stored in `http_upgrades`, will attempt to upgrade any step
  to HTTPS, and if it does not work, will fall back to HTTP. This includes the
  first step---HTTPS will be attempted before HTTP.
- **HTTP Force Upgrades**, stored in `http_force_upgrades`, will upgrade all
  steps to HTTPS, regardless of the scheme received in the redirect. This
  effectively causes the first step to always be HTTPS, and any subsequent steps
  will be HTTPS or an error.

For each of these, the value will be an enum:
- `SECURE`, meaning all steps in the chain were HTTPS,
- `INSECURE_REDIRECT`, meaning the end of the chain was HTTPS, but there was at least one step that was HTTPi
- `INSECURE`, meaning the chain ended on HTTP
- `ERROR`, meaning the chain could not be completed

The chain for each methodology is stored in the corresponding `_steps` field, which is an array of requests.

## Example Output

```json
{
  "hostname": "delta.com",
  "http_result": "INSECURE_REDIRECT",
  "http_steps": [
    {
      "hostname": "delta.com",
      "terminal": false,
      "status_code": 301,
      "next": "https://www.delta.com/",
      "insecure": true,
      "upgraded": false
    },
    {
      "hostname": "www.delta.com",
      "terminal": true,
      "status_code": 200,
      "insecure": false,
      "upgraded": false
    }
  ],
  "https_result": "ERROR",
  "https_steps": null,
  "http_upgrades": "INSECURE_REDIRECT",
  "http_upgrades_steps": [
    {
      "hostname": "delta.com",
      "terminal": false,
      "status_code": 301,
      "next": "https://www.delta.com/",
      "insecure": true,
      "upgraded": false
    },
    {
      "hostname": "www.delta.com",
      "terminal": true,
      "status_code": 200,
      "insecure": false,
      "upgraded": false
    }
  ],
  "http_force_upgrades": "ERROR",
  "http_force_upgrades_steps": null
}
```

