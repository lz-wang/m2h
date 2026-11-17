package auth

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// argon2Params are the Argon2id parameters carried by a PHC hash string.
// They come from the configuration, never from the candidate secret, so a
// crafted hash cannot steer verification parameters.
type argon2Params struct {
	version  int
	memoryKB uint32
	time     uint32
	threads  uint8
	salt     []byte
	digest   []byte
}

// decodePHC parses an Argon2id PHC string:
// $argon2id$v=19$m=65536,t=3,p=4$c2FsdA$.<base64 digest without padding>.
// Both salt and digest use the PHC profile's unpadded base64 alphabet.
func decodePHC(value string) (argon2Params, error) {
	segments := strings.Split(value, "$")
	// Split on "$argon2id$v=..$m=..$salt$hash" yields ["", "argon2id", "v=..", "m=..", salt, hash].
	if len(segments) != 6 || segments[0] != "" || segments[1] != "argon2id" {
		return argon2Params{}, fmt.Errorf("not an argon2id PHC string")
	}
	var params argon2Params
	for field := range strings.SplitSeq(segments[2], ",") {
		name, value, found := strings.Cut(field, "=")
		if !found || name != "v" {
			return argon2Params{}, fmt.Errorf("malformed version field")
		}
		version, err := strconv.Atoi(value)
		if err != nil {
			return argon2Params{}, fmt.Errorf("malformed version field")
		}
		params.version = version
	}
	if params.version < 19 {
		return argon2Params{}, fmt.Errorf("unsupported argon2id version %d", params.version)
	}
	for field := range strings.SplitSeq(segments[3], ",") {
		name, raw, found := strings.Cut(field, "=")
		if !found {
			return argon2Params{}, fmt.Errorf("malformed parameters field")
		}
		value, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return argon2Params{}, fmt.Errorf("malformed %q parameter", name)
		}
		switch name {
		case "m":
			params.memoryKB = uint32(value)
		case "t":
			params.time = uint32(value)
		case "p":
			if value < 1 || value > 255 {
				return argon2Params{}, fmt.Errorf("parallelism out of range")
			}
			params.threads = uint8(value)
		default:
			return argon2Params{}, fmt.Errorf("unknown parameter %q", name)
		}
	}
	if params.memoryKB == 0 || params.time == 0 || params.threads == 0 {
		return argon2Params{}, fmt.Errorf("incomplete argon2id parameters")
	}
	salt, err := decodePHCBase64(segments[4])
	if err != nil || len(salt) < 8 {
		return argon2Params{}, fmt.Errorf("malformed salt")
	}
	digest, err := decodePHCBase64(segments[5])
	if err != nil || len(digest) == 0 {
		return argon2Params{}, fmt.Errorf("malformed digest")
	}
	params.salt = salt
	params.digest = digest
	return params, nil
}

func decodePHCBase64(value string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(value)
}

// phcBase64 encodes bytes in the PHC profile's unpadded base64 alphabet.
func phcBase64(value []byte) string {
	return base64.RawStdEncoding.EncodeToString(value)
}
