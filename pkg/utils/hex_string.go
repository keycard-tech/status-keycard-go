package utils

import (
	"encoding/hex"
	"encoding/json"
)

type HexString []byte

// MarshalJSON serializes HexString to hex
func (s HexString) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(Btox(s))
	return bytes, err
}

// UnmarshalJSON deserializes HexString to hex
func (s *HexString) UnmarshalJSON(data []byte) error {
	var x string
	err := json.Unmarshal(data, &x)
	if err != nil {
		return err
	}
	str, err := Xtob(x)
	if err != nil {
		return err
	}

	*s = HexString([]byte(str))
	return nil
}

func (s HexString) String() string {
	return Btox(s)
}

func Btox(bytes []byte) string {
	return hex.EncodeToString(bytes)
}

func Xtob(str string) ([]byte, error) {
	return hex.DecodeString(str)
}
