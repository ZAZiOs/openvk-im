package db_models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"ovk-im/src/crypto"
)

type EncryptedJSON []byte

func (ej EncryptedJSON) Value() (driver.Value, error) {
	if len(ej) == 0 {
		return nil, nil
	}
	encrypted, err := crypto.Encrypt(string(ej))
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func (ej *EncryptedJSON) Scan(value interface{}) error {
	if value == nil {
		*ej = nil
		return nil
	}
	s, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("invalid data type for EncryptedJSON")
	}

	decrypted, err := crypto.Decrypt(string(s))
	if err != nil {
		return err
	}
	*ej = []byte(decrypted)
	return nil
}

func (ej EncryptedJSON) Unmarshal(v interface{}) error {
	return json.Unmarshal(ej, v)
}

func (ej EncryptedJSON) MarshalJSON() ([]byte, error) {
	if len(ej) == 0 {
		return []byte("null"), nil
	}
	return json.Marshal(string(ej))
}
