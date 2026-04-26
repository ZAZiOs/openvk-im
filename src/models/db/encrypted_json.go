package db_models

import (
	"context"
	"fmt"
	"ovk-im/src/crypto"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type EncryptedJSON string

func (ej *EncryptedJSON) Scan(ctx context.Context, instance *gorm.DB, field *schema.Field, dbValue interface{}) error {
	if dbValue == nil {
		return nil
	}

	encryptedStr := fmt.Sprint(dbValue)

	decrypted, err := crypto.Decrypt(encryptedStr)
	if err != nil {
		return err
	}

	*ej = EncryptedJSON(decrypted)
	return nil
}

func (ej EncryptedJSON) Value(ctx context.Context, instance *gorm.DB, field *schema.Field, fieldValue interface{}) (interface{}, error) {
	if len(ej) == 0 {
		return nil, nil
	}

	encrypted, err := crypto.Encrypt(string(ej))
	if err != nil {
		return nil, err
	}

	return encrypted, nil
}
