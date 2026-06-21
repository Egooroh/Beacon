package telegrambot

import "strings"

// langMeta holds the display name and code template for a language.
// Templates use {{URL}} and {{TOKEN}} as placeholders.
type langMeta struct {
	label    string
	template string
}

var langTemplates = map[string]langMeta{
	"go": {
		label: "Go",
		template: `import (
    "context"
    "net/http"
    "strings"
    "fmt"
)

const (
    beaconURL   = "{{URL}}"
    beaconToken = "{{TOKEN}}"
)

func captureError(msg string) {
    go func() {
        body := fmt.Sprintf("{\"level\":\"error\",\"message\":\"%s\"}", msg)
        req, _ := http.NewRequestWithContext(context.Background(),
            http.MethodPost, beaconURL+"/api/v1/ingest",
            strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("X-Beacon-Token", beaconToken)
        http.DefaultClient.Do(req)
    }()
}

// Usage:
// if err != nil {
//     captureError(err.Error())
// }`,
	},
	"python": {
		label: "Python",
		template: `import requests

BEACON_URL   = "{{URL}}"
BEACON_TOKEN = "{{TOKEN}}"

def capture_error(message: str, level: str = "error") -> None:
    try:
        requests.post(
            f"{BEACON_URL}/api/v1/ingest",
            json={"level": level, "message": message},
            headers={"X-Beacon-Token": BEACON_TOKEN},
            timeout=3,
        )
    except Exception:
        pass

# Usage:
# try:
#     risky_call()
# except Exception as e:
#     capture_error(str(e))`,
	},
	"node": {
		label: "Node.js",
		template: `const BEACON_URL   = '{{URL}}';
const BEACON_TOKEN = '{{TOKEN}}';

async function captureError(message, level = 'error') {
  try {
    await fetch(BEACON_URL + '/api/v1/ingest', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Beacon-Token': BEACON_TOKEN,
      },
      body: JSON.stringify({ level, message }),
    });
  } catch (_) {}
}

// Usage:
// captureError(err.message);
//
// Express error middleware:
// app.use((err, req, res, next) => {
//   captureError(err.message);
//   res.status(500).json({ error: 'Internal server error' });
// });`,
	},
	"php": {
		label: "PHP",
		template: `define('BEACON_URL',   '{{URL}}');
define('BEACON_TOKEN', '{{TOKEN}}');

function captureError(string $message, string $level = 'error'): void {
    $ch = curl_init(BEACON_URL . '/api/v1/ingest');
    curl_setopt_array($ch, [
        CURLOPT_POST           => true,
        CURLOPT_POSTFIELDS     => json_encode(['level' => $level, 'message' => $message]),
        CURLOPT_HTTPHEADER     => [
            'Content-Type: application/json',
            'X-Beacon-Token: ' . BEACON_TOKEN,
        ],
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_TIMEOUT        => 3,
    ]);
    curl_exec($ch);
    curl_close($ch);
}

// Usage:
// try {
//     riskyCall();
// } catch (\Exception $e) {
//     captureError($e->getMessage());
// }`,
	},
	"csharp": {
		label: "C#",
		template: `using System.Net.Http;
using System.Text;
using System.Text.Json;

private static readonly HttpClient _http = new();
private const string BeaconUrl   = "{{URL}}";
private const string BeaconToken = "{{TOKEN}}";

private static async Task CaptureError(Exception ex, string level = "error")
{
    try
    {
        var payload = JsonSerializer.Serialize(new { level, message = ex.Message });
        using var req = new HttpRequestMessage(HttpMethod.Post, BeaconUrl + "/api/v1/ingest")
        {
            Content = new StringContent(payload, Encoding.UTF8, "application/json"),
        };
        req.Headers.Add("X-Beacon-Token", BeaconToken);
        await _http.SendAsync(req);
    }
    catch { }
}

// Usage:
// try { RiskyCall(); }
// catch (Exception ex) { await CaptureError(ex); throw; }`,
	},
}

// buildSnippet returns the display label and ready-to-paste code for lang.
// Returns ("", "", false) for unknown languages.
func buildSnippet(lang, url, token string) (label, code string, ok bool) {
	meta, exists := langTemplates[lang]
	if !exists {
		return "", "", false
	}
	r := strings.NewReplacer("{{URL}}", url, "{{TOKEN}}", token)
	return meta.label, r.Replace(meta.template), true
}
