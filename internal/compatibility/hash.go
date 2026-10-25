package compatibility

import (
	"encoding/hex"
	"encoding/json"
	"strings"
)

func downloadHash(m Manifest, raw json.RawMessage) (result, error) {
	values, err := stringsValue(raw)
	urls, _ := stringsValue(m["url"])
	if err != nil || len(values) == 0 || len(values) != len(urls) {
		return unsupported("every download requires a digest")
	}
	normalized := make([]string, len(values))
	for i, value := range values {
		algorithm, digest, explicit := strings.Cut(strings.ToLower(value), ":")
		if !explicit {
			algorithm, digest = "sha256", algorithm
		}
		sizes := map[string]int{"md5": 16, "sha1": 20, "sha256": 32, "sha512": 64}
		decoded, err := hex.DecodeString(digest)
		if err != nil || sizes[algorithm] == 0 || len(decoded) != sizes[algorithm] {
			return unsupported("invalid MD5/SHA-1/SHA-256/SHA-512 digest")
		}
		normalized[i] = digest
		if algorithm != "sha256" {
			normalized[i] = algorithm + ":" + digest
		}
	}
	var value json.RawMessage
	if _, err := stringValue(raw); err == nil {
		value, _ = json.Marshal(normalized[0])
	} else {
		value, _ = json.Marshal(normalized)
	}
	return keep(value)
}
